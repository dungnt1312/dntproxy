package domain

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

// NonFallbackStatusCodes are status codes that indicate a client error
// that should not trigger fallback to next account/model.
var NonFallbackStatusCodes = map[int]bool{
	400: true, 405: true, 411: true, 413: true,
	414: true, 415: true, 422: true, 431: true,
}

// IsNonFallbackStatus returns true if the status code indicates a client error
// that should not trigger fallback.
func IsNonFallbackStatus(status int) bool {
	return NonFallbackStatusCodes[status]
}

// Cooldown durations in milliseconds.
const (
	CooldownUnauthorized   = 5000
	CooldownPaymentReq     = 300000  // 5 min
	CooldownNotFound       = 1000
	CooldownTransient      = 2000
	CooldownRequestNotAlwd = 60000
	CooldownAuthRevoked    = 1800000 // 30 min - dead refresh token / revoked grant, needs re-auth
	CooldownSuspended      = 1800000 // 30 min - account sidelined by upstream, operator likely must act
	CooldownQuotaExhausted = 3600000 // 1 hour - monthly/usage cap hit; short retries just burn budget
	BackoffBase            = 1000
	BackoffMax             = 120000  // 2 min
	BackoffMaxLevel        = 7
)

// truncatedStreamMarker is a stable, lowercase substring embedded in the error
// an executor returns when a transport-clean upstream stream carried content but
// never a terminal signal (the classic Kiro mid-answer truncation). CheckFallbackError
// treats it as a no-penalty failover so the request retries on another account
// without cooling the current one. Kept lowercase so the case-insensitive match
// in CheckFallbackError is exact.
const truncatedStreamMarker = "upstream truncated response without stop reason"

// TruncatedStreamError builds the error text an executor returns for a detected
// truncation, embedding the sentinel CheckFallbackError matches on.
func TruncatedStreamError(model string, contentBytes int) string {
	return fmt.Sprintf("%s (model=%s, contentBytes=%d)", truncatedStreamMarker, model, contentBytes)
}

// IsTruncatedStreamError reports whether an error text carries the truncation
// sentinel.
func IsTruncatedStreamError(errorText string) bool {
	return strings.Contains(strings.ToLower(errorText), truncatedStreamMarker)
}

// FallbackResult describes whether to fallback and how long to cool down.
type FallbackResult struct {
	ShouldFallback  bool
	CooldownMs      int
	NewBackoffLevel int
	ModelOnly       bool
}

// GetQuotaCooldown returns exponential backoff cooldown for a given level.
func GetQuotaCooldown(level int) int {
	cooldown := float64(BackoffBase) * math.Pow(2, float64(level))
	if cooldown > float64(BackoffMax) {
		return BackoffMax
	}
	return int(cooldown)
}

// CheckFallbackError determines if an error should trigger account fallback.
func CheckFallbackError(status int, errorText string, backoffLevel int) FallbackResult {
	lower := strings.ToLower(errorText)
	class := ClassifyUpstream(status, errorText)

	if class == UpstreamNonFallback {
		return FallbackResult{ShouldFallback: false, NewBackoffLevel: backoffLevel}
	}
	if class == UpstreamModelEntitlement {
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownPaymentReq, NewBackoffLevel: backoffLevel, ModelOnly: true}
	}

	if strings.Contains(lower, "timeout awaiting response headers") {
		// HTTP/2 response-header timeouts can fail over without penalizing the account.
		return FallbackResult{ShouldFallback: true, NewBackoffLevel: backoffLevel}
	}
	if strings.Contains(lower, truncatedStreamMarker) {
		// A transport-clean stream that carried content but never a terminal signal
		// is an upstream blip, not an unhealthy account: fail over to the next
		// account for this request without penalizing (mirrors Kiro IDE, which
		// retries truncation on the same credential without cooling it).
		return FallbackResult{ShouldFallback: true, NewBackoffLevel: backoffLevel}
	}
	if strings.Contains(lower, "no credentials") {
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownNotFound, NewBackoffLevel: backoffLevel}
	}
	if strings.Contains(lower, "request not allowed") {
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownRequestNotAlwd, NewBackoffLevel: backoffLevel}
	}

	switch class {
	case UpstreamSuspended:
		// Account sidelined by upstream (suspended/banned). Not transient: sideline
		// it for a long account-level window instead of retrying every few seconds.
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownSuspended, NewBackoffLevel: BackoffMaxLevel}
	case UpstreamAuth:
		// A dead refresh token / revoked grant cannot be fixed by an in-place
		// refresh, so sideline the account for a long window instead of hammering
		// it on the 5s cooldown. A plain expired access token stays short: the
		// chat-service refreshes it in place before ever reaching a fallback.
		if isAuthRevokedError(lower) {
			return FallbackResult{ShouldFallback: true, CooldownMs: CooldownAuthRevoked, NewBackoffLevel: BackoffMaxLevel}
		}
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownUnauthorized, NewBackoffLevel: backoffLevel}
	case UpstreamQuota:
		// A monthly/usage-cap/credit exhaustion keeps returning the same error
		// for a long window; short exponential backoff just burns request budget
		// re-selecting an account that cannot serve. Sideline it for a long
		// account-level window, honoring an upstream reset hint when present.
		if isHardQuotaExhaustion(lower) {
			return FallbackResult{ShouldFallback: true, CooldownMs: hardQuotaCooldownMs(lower), NewBackoffLevel: BackoffMaxLevel}
		}
		// Preserve Command Code's unqualified 402 payment cooldown. A generic
		// 403 remains a no-penalty failover unless it is model entitlement.
		if status == 403 {
			hasRateLimitKeyword := false
			for _, keyword := range []string{"rate limit", "too many requests", "quota exceeded", "capacity", "overloaded"} {
				if strings.Contains(lower, keyword) {
					hasRateLimitKeyword = true
					break
				}
			}
			if !hasRateLimitKeyword {
				return FallbackResult{ShouldFallback: true, NewBackoffLevel: backoffLevel}
			}
		}
		if status == 402 {
			return FallbackResult{ShouldFallback: true, CooldownMs: CooldownPaymentReq, NewBackoffLevel: backoffLevel}
		}
		newLevel := min(backoffLevel+1, BackoffMaxLevel)
		// A server-provided Retry-After is authoritative for a plain rate limit:
		// honor it instead of a guessed exponential backoff. Still clamped by the
		// operator's MaxCooldownSeconds downstream in MarkUnavailable.
		if hintMs, ok := ExtractRetryAfterHint(errorText); ok {
			return FallbackResult{ShouldFallback: true, CooldownMs: hintMs, NewBackoffLevel: newLevel}
		}
		return FallbackResult{ShouldFallback: true, CooldownMs: GetQuotaCooldown(backoffLevel), NewBackoffLevel: newLevel}
	case UpstreamTransient:
		if hintMs, ok := ExtractRetryAfterHint(errorText); ok {
			return FallbackResult{ShouldFallback: true, CooldownMs: hintMs, NewBackoffLevel: backoffLevel, ModelOnly: true}
		}
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownTransient, NewBackoffLevel: backoffLevel, ModelOnly: true}
	}

	if status == 404 {
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownNotFound, NewBackoffLevel: backoffLevel}
	}
	return FallbackResult{ShouldFallback: true, CooldownMs: CooldownTransient, NewBackoffLevel: backoffLevel}
}

// isSuspensionError reports whether the upstream marked the account as
// suspended/banned. These are not transient: the account cannot recover on its
// own, so it must be sidelined for a long window instead of being retried every
// few seconds. Mirrors Kiro's suspension signals.
func isSuspensionError(lower string) bool {
	hints := []string{
		"temporarily_suspended",
		"temporarily suspended",
		"temporarily is suspended",
		"account suspended",
		"account is suspended",
		"unusual user activity",
		// Zhipu/GLM bans the whole account and reports it as 403 with a Chinese
		// message. This used to be listed as a model-entitlement hint, which
		// locked only the model and left a banned account serving other models.
		"封禁",
		"被冻结",
		"账号被禁",
	}
	for _, hint := range hints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// isAuthRevokedError reports whether an auth failure is a hard credential
// revocation (refresh token dead, grant revoked) rather than a merely expired
// access token that an in-place refresh can fix. A dead refresh token cannot be
// recovered automatically, so the account should be sidelined for a long window
// rather than hammered every few seconds on the 5s unauthorized cooldown.
func isAuthRevokedError(lower string) bool {
	hints := []string{
		"invalid_grant",
		"invalid grant",
		"invalid refresh token",
		"refresh token expired",
		"refresh token is invalid",
		"refresh token invalid",
		"token has been revoked",
		"token revoked",
		"reauthenticate",
		"re-authenticate",
	}
	for _, hint := range hints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// IsAuthRevokedErrorText reports whether an error message indicates a hard
// credential revocation. Exported so services can sideline accounts whose
// refresh token the upstream no longer accepts.
func IsAuthRevokedErrorText(errorText string) bool {
	return isAuthRevokedError(strings.ToLower(errorText))
}

// isHardQuotaExhaustion reports whether a quota error is a long-window
// exhaustion (monthly/usage cap hit) rather than a short-term rate limit. A
// short-term "too many requests" recovers in seconds and suits exponential
// backoff; a monthly/usage-limit exhaustion will keep returning the same error
// for hours, so retrying the same account every 120s just burns request budget.
func isHardQuotaExhaustion(lower string) bool {
	hints := []string{
		"quota exceeded",
		"quota has been exceeded",
		"usage limit",
		"monthly limit",
		"monthly request",
		"free tier",
		"out of credits",
		"insufficient credits",
		"insufficient_quota",
		"insufficient balance",
		"credit balance",
		"no available resource package",
		"please recharge",
		"limit reached",
		"limit exceeded",
		// GLM/Zhipu reports an empty credit balance as a plain 400 BAD_REQUEST
		// with a Chinese message. Without these hints the request was treated as
		// a client error: no failover, no cooldown, so every following request
		// picked the same broke account again.
		"余额不足",
		"欠费",
		"无可用资源包",
		"请充值",
		"额度已用尽",
		"额度用尽",
		"资源包已用完",
	}
	for _, hint := range hints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// hardQuotaCooldownMs returns the account-level cooldown for a hard quota
// exhaustion. The default tier applies unless the upstream embedded a reset
// hint ("Your limit resets at 2026-08-25T07:21:09.207Z." — Command Code sends
// these on weekly/5-hour windows); when the stated reset is later than the
// default tier, cool down until then so the account does not re-enter rotation
// hourly for a multi-day window. Still clamped by MaxCooldownSeconds downstream.
func hardQuotaCooldownMs(lower string) int {
	t, ok := ExtractResetInstant(lower)
	if !ok {
		return CooldownQuotaExhausted
	}
	ms := int(time.Until(t).Milliseconds())
	if ms <= CooldownQuotaExhausted {
		return CooldownQuotaExhausted
	}
	return ms
}

// resetTimestampRe matches the timestamp shapes upstreams embed in quota
// messages: RFC3339 ("2026-08-25T07:21:09.207Z"), a space-separated naive form
// ("2026-10-26 20:04:25" — GLM/Zhipu), and optional numeric offsets.
var resetTimestampRe = regexp.MustCompile(
	`\d{4}-\d{2}-\d{2}[t ]\d{2}:\d{2}(?::\d{2})?(?:\.\d+)?(?:z|[+-]\d{2}:?\d{2})?`)

// providerNaiveZone is the zone assumed for a reset timestamp that carries no
// offset. Providers that emit naive timestamps (GLM/Zhipu) are Chinese and
// report Beijing time; assuming +08 keeps the cooldown aligned with the window
// instead of cooling for the wrong number of hours.
var providerNaiveZone = time.FixedZone("UTC+8", 8*60*60)

// ExtractResetInstant reports when a quota window resets according to a hint in
// the error text, if any. It accepts the RFC3339 form used by Command Code and
// the space-separated naive form used by GLM ("Your limit will reset at
// 2026-10-26 20:04:25", "额度将在 2026-10-26 20:04:25 重置"). It deliberately
// scans for a timestamp shape instead of matching one exact marker phrase,
// because providers phrase the hint differently ("reset at" vs "resets at") and
// some wrap it in another language.
func ExtractResetInstant(errorText string) (time.Time, bool) {
	lower := strings.ToLower(errorText)
	for _, match := range resetTimestampRe.FindAllString(lower, -1) {
		// CheckFallbackError lowercases the message; RFC3339 wants uppercase T/Z.
		normalized := strings.ToUpper(match)
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if t, err := time.Parse(layout, normalized); err == nil {
				return t, true
			}
		}
		for _, layout := range []string{
			"2006-01-02T15:04:05",
			"2006-01-02T15:04",
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
		} {
			if t, err := time.ParseInLocation(layout, normalized, providerNaiveZone); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func isModelEntitlementError(lower string) bool {
	entitlementHints := []string{
		"model_not_entitled",
		"not entitled",
		"not subscribed",
		"unavailable model",
		"未订阅",
	}
	for _, hint := range entitlementHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// IsStickyAccountCooldown reports whether a recorded error describes a state
// that a single later success cannot clear: an exhausted quota/credit balance, a
// suspended/banned account, or a revoked credential. All three need operator
// action (top up, appeal, re-auth), so routing must keep the account sidelined
// until the cooldown expires instead of trusting one lucky request.
func IsStickyAccountCooldown(errorText string) bool {
	if strings.TrimSpace(errorText) == "" {
		return false
	}
	lower := strings.ToLower(errorText)
	if isSuspensionError(lower) || isAuthRevokedError(lower) {
		return true
	}
	if !isHardQuotaExhaustion(lower) {
		return false
	}
	// A plain short-term limiter recovers on its own after seconds, so it must
	// stay clearable. Only phrase that sounds like an exhausted balance/quota
	// window is worth protecting from a later success.
	for _, hint := range []string{
		"rate limit", "ratelimit", "too many requests", "concurrency", "并发",
	} {
		if strings.Contains(lower, hint) &&
			!strings.Contains(lower, "balance") &&
			!strings.Contains(lower, "余额") {
			return false
		}
	}
	return true
}

// IsAccountUnavailable checks if a cooldown timestamp is still active.
// Corrupt timestamps are treated as unavailable (safe default) to prevent
// infinite retry loops against broken connections.
func IsAccountUnavailable(rateLimitedUntil string) bool {
	if rateLimitedUntil == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, rateLimitedUntil)
	if err != nil {
		return true
	}
	return t.After(time.Now())
}

// IsModelLockActive reports whether a per-model (or global "__all") lock is
// currently active for model.
// Corrupt timestamps are treated as active (safe default).
func IsModelLockActive(locks map[string]string, model string) bool {
	return ModelLockExpiry(locks, model) != ""
}

// ModelLockExpiry returns the expiry timestamp of the active lock covering
// model, or "" when the model is not locked. A corrupt timestamp counts as
// active (fail closed) and is returned as-is. Exported so callers can report
// when the model becomes usable again.
func ModelLockExpiry(locks map[string]string, model string) string {
	if locks == nil {
		return ""
	}
	// Check specific model lock
	if expiry, ok := locks[model]; ok {
		if t, err := time.Parse(time.RFC3339, expiry); err != nil || t.After(time.Now()) {
			return expiry
		}
	}
	// Check global lock
	if expiry, ok := locks["__all"]; ok {
		if t, err := time.Parse(time.RFC3339, expiry); err != nil || t.After(time.Now()) {
			return expiry
		}
	}
	return ""
}

// CooldownUntil returns an ISO timestamp cooldownMs from now.
func CooldownUntil(cooldownMs int) string {
	return time.Now().Add(time.Duration(cooldownMs) * time.Millisecond).UTC().Format(time.RFC3339)
}
