package domain

import (
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
	CooldownPaymentReq     = 300000 // 5 min
	CooldownNotFound       = 1000
	CooldownTransient      = 2000
	CooldownRequestNotAlwd = 60000
	BackoffBase            = 1000
	BackoffMax             = 120000 // 2 min
	BackoffMaxLevel        = 7
)

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
	if strings.Contains(lower, "no credentials") {
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownNotFound, NewBackoffLevel: backoffLevel}
	}
	if strings.Contains(lower, "request not allowed") {
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownRequestNotAlwd, NewBackoffLevel: backoffLevel}
	}

	switch class {
	case UpstreamAuth:
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
		newLevel := min(backoffLevel+1, BackoffMaxLevel)
		return FallbackResult{ShouldFallback: true, CooldownMs: GetQuotaCooldown(backoffLevel), NewBackoffLevel: newLevel}
	case UpstreamTransient:
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownTransient, NewBackoffLevel: backoffLevel, ModelOnly: true}
	}

	if status == 404 {
		return FallbackResult{ShouldFallback: true, CooldownMs: CooldownNotFound, NewBackoffLevel: backoffLevel}
	}
	return FallbackResult{ShouldFallback: true, CooldownMs: CooldownTransient, NewBackoffLevel: backoffLevel}
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
