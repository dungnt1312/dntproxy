package domain

import (
	"math"
	"strings"
	"time"
)

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

	// Error text patterns take priority
	if strings.Contains(lower, "no credentials") {
		return FallbackResult{true, CooldownNotFound, backoffLevel}
	}
	if strings.Contains(lower, "request not allowed") {
		return FallbackResult{true, CooldownRequestNotAlwd, backoffLevel}
	}
	if strings.Contains(lower, "improperly formed request") {
		return FallbackResult{true, CooldownPaymentReq, backoffLevel}
	}

	// Rate limit keywords → exponential backoff
	rateLimitKeywords := []string{"rate limit", "too many requests", "quota exceeded", "capacity", "overloaded"}
	for _, kw := range rateLimitKeywords {
		if strings.Contains(lower, kw) {
			newLevel := backoffLevel + 1
			if newLevel > BackoffMaxLevel {
				newLevel = BackoffMaxLevel
			}
			return FallbackResult{true, GetQuotaCooldown(backoffLevel), newLevel}
		}
	}

	// Status code based
	switch status {
	case 401:
		return FallbackResult{true, CooldownUnauthorized, backoffLevel}
	case 402, 403:
		return FallbackResult{true, CooldownPaymentReq, backoffLevel}
	case 404:
		return FallbackResult{true, CooldownNotFound, backoffLevel}
	case 429:
		newLevel := backoffLevel + 1
		if newLevel > BackoffMaxLevel {
			newLevel = BackoffMaxLevel
		}
		return FallbackResult{true, GetQuotaCooldown(backoffLevel), newLevel}
	case 406, 408, 500, 502, 503, 504:
		return FallbackResult{true, CooldownTransient, backoffLevel}
	}

	// Default: fallback with transient cooldown
	return FallbackResult{true, CooldownTransient, backoffLevel}
}

// IsAccountUnavailable checks if a cooldown timestamp is still active.
func IsAccountUnavailable(rateLimitedUntil string) bool {
	if rateLimitedUntil == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, rateLimitedUntil)
	if err != nil {
		return false
	}
	return t.After(time.Now())
}

// IsModelLockActive checks if a model lock on a connection is still active.
func IsModelLockActive(locks map[string]string, model string) bool {
	if locks == nil {
		return false
	}
	// Check specific model lock
	if expiry, ok := locks[model]; ok {
		if t, err := time.Parse(time.RFC3339, expiry); err == nil && t.After(time.Now()) {
			return true
		}
	}
	// Check global lock
	if expiry, ok := locks["__all"]; ok {
		if t, err := time.Parse(time.RFC3339, expiry); err == nil && t.After(time.Now()) {
			return true
		}
	}
	return false
}

// CooldownUntil returns an ISO timestamp cooldownMs from now.
func CooldownUntil(cooldownMs int) string {
	return time.Now().Add(time.Duration(cooldownMs) * time.Millisecond).UTC().Format(time.RFC3339)
}
