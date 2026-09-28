package domain

import (
	"testing"
	"time"
)

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

// Command Code reports exhausted credits as 400 BAD_REQUEST. The quota body must
// win over the non-fallback status so the request rotates to another account
// and the exhausted one is sidelined, instead of surfacing a client error.
func TestCommandCodeInsufficientCreditsRotatesDespite400(t *testing.T) {
	body := `commandcode returned 400: {"success":false,"error":{"code":"BAD_REQUEST","status":400,"message":"You have insufficient credits to make this request. Please purchase more credits to continue."}}`

	if got := ClassifyUpstream(400, body); got != UpstreamQuota {
		t.Fatalf("classification = %s, want quota (400 insufficient credits)", got)
	}
	if !IsRetryableUpstream(400, body) {
		t.Fatal("insufficient-credits 400 must be retryable on another account")
	}
	r := CheckFallbackError(400, body, 0)
	if !r.ShouldFallback {
		t.Fatal("want fallback to next account")
	}
	if r.CooldownMs != CooldownQuotaExhausted {
		t.Fatalf("cooldown = %d, want long quota-exhausted %d", r.CooldownMs, CooldownQuotaExhausted)
	}
	if r.NewBackoffLevel != BackoffMaxLevel {
		t.Fatalf("NewBackoffLevel = %d, want max %d", r.NewBackoffLevel, BackoffMaxLevel)
	}

	// Guard the false-positive direction: genuine 400 client errors stay
	// non-fallback (same reply on every account).
	ctxLen := "This model's maximum context length is 1048576 tokens. However, you requested 1186975 tokens. Please reduce your prompt."
	if got := ClassifyUpstream(400, ctxLen); got != UpstreamNonFallback {
		t.Fatalf("context-length 400 classification = %s, want non_fallback", got)
	}
	if IsRetryableUpstream(400, ctxLen) {
		t.Fatal("context-length 400 must not rotate accounts")
	}
}

// An upstream that states when the quota window resets (Command Code weekly and
// 5-hour limits) gets a cooldown that covers the stated window instead of
// re-entering rotation hourly for days.
func TestHardQuotaCooldownHonorsResetHint(t *testing.T) {
	reset := time.Now().UTC().Add(6 * time.Hour).Format("2006-01-02T15:04:05.000Z")
	msg := `You've reached your weekly usage limit for your plan. Your limit resets at ` + reset + `. Please wait for the window to reset or upgrade your plan to continue.`

	r := CheckFallbackError(429, msg, 0)
	if !r.ShouldFallback {
		t.Fatal("want fallback")
	}
	if r.CooldownMs <= CooldownQuotaExhausted {
		t.Fatalf("cooldown = %d, want > default tier %d (should cover the reset window)", r.CooldownMs, CooldownQuotaExhausted)
	}
	if want := int((6 * time.Hour).Milliseconds()); r.CooldownMs < want-60_000 {
		t.Fatalf("cooldown = %d, want ≈ %d", r.CooldownMs, want)
	}

	// A reset time already in the past falls back to the default tier.
	past := `You've reached your 5-hour usage limit. Your limit resets at 2020-01-01T00:00:00Z. Please wait.`
	r = CheckFallbackError(429, past, 0)
	if r.CooldownMs != CooldownQuotaExhausted {
		t.Fatalf("past-reset cooldown = %d, want default tier %d", r.CooldownMs, CooldownQuotaExhausted)
	}

	// No hint at all: default tier.
	if r := CheckFallbackError(402, "payment required, top up your balance", 0); r.CooldownMs != CooldownPaymentReq {
		t.Fatalf("unqualified 402 cooldown = %d, want %d", r.CooldownMs, CooldownPaymentReq)
	}
}
