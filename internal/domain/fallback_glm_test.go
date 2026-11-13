package domain

import (
	"testing"
	"time"
)

// GLM/Zhipu reports an empty credit balance as a plain 400 BAD_REQUEST with a
// Chinese message. It used to land in the non-fallback bucket: the request did
// not rotate to another account and nothing was recorded, so every following
// request picked the same broke account again.
func TestCheckFallbackError_PaymentExhaustionOn400(t *testing.T) {
	bodies := []string{
		`{"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`,
		`{"error":{"code":"1113","message":"Insufficient balance, please recharge."}}`,
		`{"error":{"code":"1113","message":"no available resource package"}}`,
		`{"error":{"code":"1113","message":"您的账户已欠费，请充值"}}`,
	}
	for _, body := range bodies {
		if got := ClassifyUpstream(400, body); got != UpstreamQuota {
			t.Fatalf("%q: class = %s, want quota", body, got)
		}
		if !IsRetryableUpstream(400, body) {
			t.Fatalf("%q: must rotate to another account", body)
		}
		r := CheckFallbackError(400, body, 0)
		if !r.ShouldFallback {
			t.Fatalf("%q: ShouldFallback = false, want true", body)
		}
		if r.CooldownMs < CooldownQuotaExhausted {
			t.Fatalf("%q: cooldown = %d, want >= %d (long window)", body, r.CooldownMs, CooldownQuotaExhausted)
		}
		if r.ModelOnly {
			t.Fatalf("%q: an exhausted balance must sideline the account, not just the model", body)
		}
		if r.NewBackoffLevel != BackoffMaxLevel {
			t.Fatalf("%q: NewBackoffLevel = %d, want %d", body, r.NewBackoffLevel, BackoffMaxLevel)
		}
		if !IsStickyAccountCooldown(body) {
			t.Fatalf("%q: must be sticky so a later success cannot clear it", body)
		}
	}
}

// A genuine 400 client error must stay non-fallback: the same reply would come
// from every account, so rotating only burns budget.
func TestCheckFallbackError_GenuineClient400StaysNonFallback(t *testing.T) {
	body := `{"error":{"code":"1211","message":"模型不存在"}}`
	if got := ClassifyUpstream(400, body); got != UpstreamNonFallback {
		t.Fatalf("class = %s, want non_fallback", got)
	}
	if IsRetryableUpstream(400, body) {
		t.Fatal("unknown-model 400 must not rotate accounts")
	}
	if IsStickyAccountCooldown(body) {
		t.Fatal("a client error is not a sticky account cooldown")
	}
}

// Zhipu bans a whole account and answers 403 "您的账户已被封禁". That used to be
// listed as a model-entitlement hint, so only the model was locked for 5 min and
// the banned account kept serving other models.
func TestCheckFallbackError_AccountBannedSidelinesAccount(t *testing.T) {
	bodies := []string{
		`{"error":{"code":"1112","message":"您的账户已被封禁"}}`,
		`{"error":{"code":"1112","message":"账号被禁，请联系客服"}}`,
	}
	for _, body := range bodies {
		if got := ClassifyUpstream(403, body); got != UpstreamSuspended {
			t.Fatalf("%q: class = %s, want suspended", body, got)
		}
		r := CheckFallbackError(403, body, 0)
		if !r.ShouldFallback {
			t.Fatalf("%q: want fallback", body)
		}
		if r.ModelOnly {
			t.Fatalf("%q: a ban is account-level, not model-level", body)
		}
		if r.CooldownMs != CooldownSuspended {
			t.Fatalf("%q: cooldown = %d, want %d", body, r.CooldownMs, CooldownSuspended)
		}
		if !IsStickyAccountCooldown(body) {
			t.Fatalf("%q: a ban is sticky", body)
		}
	}
}

// GLM's 5-hour quota message states the reset as "Your limit will reset at
// 2026-10-26 20:04:25" — space separated, naive, and phrased "reset at" (not
// "resets at"). The parser only understood RFC3339 after "resets at ", so the
// account was cooled for a flat hour and re-entered rotation long before the
// window actually reopened.
func TestCheckFallbackError_HonorsGLMNaiveResetHint(t *testing.T) {
	reset := time.Now().In(providerNaiveZone).Add(3 * time.Hour).Truncate(time.Second)
	stamp := reset.Format("2006-01-02 15:04:05")

	cases := []string{
		`{"error":{"code":"1308","message":"Usage limit reached for 5 hour. Your limit will reset at ` + stamp + `"}}`,
		`{"error":{"code":"1308","message":"您的使用额度已用尽，额度将在 ` + stamp + ` 重置"}}`,
	}
	for _, body := range cases {
		r := CheckFallbackError(429, body, 0)
		if !r.ShouldFallback {
			t.Fatalf("%q: want fallback", body)
		}
		if want := int((3 * time.Hour).Milliseconds()); r.CooldownMs < want-60_000 {
			t.Fatalf("%q: cooldown = %d, want ≈ %d", body, r.CooldownMs, want)
		}
		if r.ModelOnly {
			t.Fatalf("%q: quota exhaustion must sideline the account", body)
		}
	}
}

// An unresolvable or already-past reset keeps the default tier, and a missing
// hint must not shorten it below the tier either.
func TestCheckFallbackError_GLMResetFallbacks(t *testing.T) {
	past := time.Now().In(providerNaiveZone).Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
	cases := []string{
		`{"error":{"code":"1308","message":"Usage limit reached. Your limit will reset at ` + past + `"}}`,
		`{"error":{"code":"1308","message":"Usage limit reached for 5 hour."}}`,
	}
	for _, body := range cases {
		if r := CheckFallbackError(429, body, 0); r.CooldownMs != CooldownQuotaExhausted {
			t.Fatalf("%q: cooldown = %d, want default %d", body, r.CooldownMs, CooldownQuotaExhausted)
		}
	}
}

// Concurrency limits ("并发数过高") are a short-term rate limit, not a spent
// quota: they must keep the short exponential backoff.
func TestCheckFallbackError_GLMConcurrencyStaysShortBackoff(t *testing.T) {
	for _, body := range []string{
		`{"error":{"code":"1302","message":"您当前使用该API的并发数过高，请降低并发"}}`,
		`{"error":{"code":"1305","message":"并发数过高"}}`,
	} {
		r := CheckFallbackError(429, body, 0)
		if r.CooldownMs != GetQuotaCooldown(0) {
			t.Fatalf("%q: cooldown = %d, want short backoff %d", body, r.CooldownMs, GetQuotaCooldown(0))
		}
		if IsStickyAccountCooldown(body) {
			t.Fatal("a concurrency limit is not sticky")
		}
	}
}

// GLM sends business errors with client-error statuses. A ban reported as 400
// must still be recognised as a suspension; otherwise the non-fallback gate
// classifies it as a client error, no account is sidelined, and every request
// keeps hitting the banned account.
func TestCheckFallbackError_SuspensionBeatsClientErrorStatus(t *testing.T) {
	bodies := []string{
		`{"error":{"code":"1112","message":"您的账户已被封禁"}}`,
		`{"error":{"code":"1112","message":"account suspended"}}`,
	}
	for _, body := range bodies {
		if got := ClassifyUpstream(400, body); got != UpstreamSuspended {
			t.Fatalf("%q: class = %s, want suspended", body, got)
		}
		if !IsRetryableUpstream(400, body) {
			t.Fatalf("%q: a ban must rotate off the account", body)
		}
		r := CheckFallbackError(400, body, 0)
		if !r.ShouldFallback || r.CooldownMs != CooldownSuspended || r.ModelOnly {
			t.Fatalf("%q: result = %+v, want account-level %dms", body, r, CooldownSuspended)
		}
		if !IsStickyAccountCooldown(body) {
			t.Fatalf("%q: a ban is sticky", body)
		}
	}
}

func TestExtractResetInstant(t *testing.T) {
	if _, ok := ExtractResetInstant("no timestamp here"); ok {
		t.Fatal("want no reset instant")
	}
	// RFC3339 with an explicit offset keeps the stated instant.
	got, ok := ExtractResetInstant("your limit resets at 2030-01-02T03:04:05Z")
	if !ok || !got.Equal(time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("RFC3339 instant = %v ok=%v", got, ok)
	}
	// A naive stamp is interpreted in the provider's zone, not UTC.
	got, ok = ExtractResetInstant("reset at 2030-01-02 03:04:05")
	want := time.Date(2030, 1, 2, 3, 4, 5, 0, providerNaiveZone)
	if !ok || !got.Equal(want) {
		t.Fatalf("naive instant = %v, want %v", got, want)
	}
}
