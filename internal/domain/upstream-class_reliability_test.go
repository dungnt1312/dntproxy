package domain

import "testing"

// A short-term rate limit (429 "too many requests") must keep the existing
// exponential backoff: it recovers in seconds, so a long sideline would waste
// a healthy account.
func TestCheckFallbackError_RateLimitStaysShortBackoff(t *testing.T) {
	r := CheckFallbackError(429, "429 too many requests, please slow down", 0)
	if !r.ShouldFallback {
		t.Fatal("want fallback")
	}
	if r.CooldownMs != GetQuotaCooldown(0) {
		t.Fatalf("rate-limit cooldown = %d, want short backoff %d", r.CooldownMs, GetQuotaCooldown(0))
	}
	if r.NewBackoffLevel != 1 {
		t.Fatalf("NewBackoffLevel = %d, want 1", r.NewBackoffLevel)
	}
}

// A monthly/usage-cap exhaustion returns the same error for a long window, so
// it must sideline the account with a long cooldown instead of the short
// backoff that a transient rate limit gets.
func TestCheckFallbackError_HardQuotaExhaustionLongCooldown(t *testing.T) {
	cases := []string{
		"429: quota exceeded for this month",
		"monthly limit reached",
		"you are out of credits",
		"insufficient_quota",
	}
	for _, msg := range cases {
		r := CheckFallbackError(429, msg, 0)
		if !r.ShouldFallback {
			t.Fatalf("%q: want fallback", msg)
		}
		if r.CooldownMs != CooldownQuotaExhausted {
			t.Fatalf("%q: cooldown = %d, want long quota-exhausted %d", msg, r.CooldownMs, CooldownQuotaExhausted)
		}
		if r.NewBackoffLevel != BackoffMaxLevel {
			t.Fatalf("%q: NewBackoffLevel = %d, want max %d", msg, r.NewBackoffLevel, BackoffMaxLevel)
		}
		if r.ModelOnly {
			t.Fatalf("%q: quota exhaustion must sideline the account, not just the model", msg)
		}
	}
}

// An account suspended/banned by upstream cannot recover on its own; it must be
// sidelined for a long window rather than retried every few seconds.
func TestCheckFallbackError_SuspensionLongCooldown(t *testing.T) {
	cases := []string{
		"403: account temporarily_suspended",
		"account suspended due to unusual user activity",
		"your account is suspended",
	}
	for _, msg := range cases {
		if ClassifyUpstream(500, msg) != UpstreamSuspended {
			t.Fatalf("%q: want UpstreamSuspended classification", msg)
		}
		r := CheckFallbackError(403, msg, 0)
		if !r.ShouldFallback {
			t.Fatalf("%q: want fallback", msg)
		}
		if r.CooldownMs != CooldownSuspended {
			t.Fatalf("%q: cooldown = %d, want suspended %d", msg, r.CooldownMs, CooldownSuspended)
		}
		if r.NewBackoffLevel != BackoffMaxLevel {
			t.Fatalf("%q: NewBackoffLevel = %d, want max %d", msg, r.NewBackoffLevel, BackoffMaxLevel)
		}
	}
}

// A revoked/dead refresh token cannot be fixed by an in-place refresh, so the
// account gets a long cooldown instead of the 5s unauthorized retry.
func TestCheckFallbackError_AuthRevokedLongCooldown(t *testing.T) {
	revoked := CheckFallbackError(401, "invalid_grant: refresh token expired", 0)
	if !revoked.ShouldFallback {
		t.Fatal("want fallback")
	}
	if revoked.CooldownMs != CooldownAuthRevoked {
		t.Fatalf("revoked cooldown = %d, want %d", revoked.CooldownMs, CooldownAuthRevoked)
	}
	if revoked.NewBackoffLevel != BackoffMaxLevel {
		t.Fatalf("NewBackoffLevel = %d, want max %d", revoked.NewBackoffLevel, BackoffMaxLevel)
	}

	// A plain expired access token (no revocation markers) stays on the short
	// unauthorized cooldown, because chat-service refreshes it in place first.
	plain := CheckFallbackError(401, "401 unauthorized", 0)
	if plain.CooldownMs != CooldownUnauthorized {
		t.Fatalf("plain 401 cooldown = %d, want short %d", plain.CooldownMs, CooldownUnauthorized)
	}
}
