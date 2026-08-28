package domain

import (
	"fmt"
	"math"
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
		// A monthly/usage-cap exhaustion keeps returning the same error for a long
		// window; short exponential backoff just burns request budget re-selecting
		// an account that cannot serve. Sideline it for a long account-level window.
		if isHardQuotaExhaustion(lower) {
			return FallbackResult{ShouldFallback: true, CooldownMs: CooldownQuotaExhausted, NewBackoffLevel: BackoffMaxLevel}
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
		"limit reached",
		"limit exceeded",
	}
	for _, hint := range hints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

func isModelEntitlementError(lower string) bool {
	entitlementHints := []string{
		"model_not_entitled",
		"not entitled",
		"not subscribed",
		"unavailable model",
		"未订阅",
		"封禁",
	}
	for _, hint := range entitlementHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
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

// IsModelLockActive checks if a model lock on a connection is still active.
// Corrupt timestamps are treated as active (safe default).
func IsModelLockActive(locks map[string]string, model string) bool {
	if locks == nil {
		return false
	}
	// Check specific model lock
	if expiry, ok := locks[model]; ok {
		if t, err := time.Parse(time.RFC3339, expiry); err != nil || t.After(time.Now()) {
			return true
		}
	}
	// Check global lock
	if expiry, ok := locks["__all"]; ok {
		if t, err := time.Parse(time.RFC3339, expiry); err != nil || t.After(time.Now()) {
			return true
		}
	}
	return false
}

// CooldownUntil returns an ISO timestamp cooldownMs from now.
func CooldownUntil(cooldownMs int) string {
	return time.Now().Add(time.Duration(cooldownMs) * time.Millisecond).UTC().Format(time.RFC3339)
}
