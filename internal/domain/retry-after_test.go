package domain

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfterHeader_Seconds(t *testing.T) {
	ms, ok := ParseRetryAfterHeader("120")
	if !ok || ms != 120000 {
		t.Fatalf("ParseRetryAfterHeader(120) = %d,%v want 120000,true", ms, ok)
	}
}

func TestParseRetryAfterHeader_Empty(t *testing.T) {
	if _, ok := ParseRetryAfterHeader(""); ok {
		t.Fatal("empty header should not parse")
	}
	if _, ok := ParseRetryAfterHeader("0"); ok {
		t.Fatal("zero seconds should not parse")
	}
	if _, ok := ParseRetryAfterHeader("garbage"); ok {
		t.Fatal("garbage should not parse")
	}
}

func TestParseRetryAfterHeader_HTTPDate(t *testing.T) {
	future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	ms, ok := ParseRetryAfterHeader(future)
	if !ok {
		t.Fatal("future HTTP date should parse")
	}
	// Allow a wide band for scheduling jitter.
	if ms < 60000 || ms > 95000 {
		t.Fatalf("HTTP date ms = %d, want ~90000", ms)
	}

	past := time.Now().Add(-1 * time.Minute).UTC().Format(http.TimeFormat)
	if _, ok := ParseRetryAfterHeader(past); ok {
		t.Fatal("past HTTP date should not parse")
	}
}

func TestParseRetryAfterHeader_Capped(t *testing.T) {
	ms, ok := ParseRetryAfterHeader("99999") // 99999s > 1h cap
	if !ok || ms != retryAfterMaxMs {
		t.Fatalf("expected cap %d, got %d,%v", retryAfterMaxMs, ms, ok)
	}
}

func TestRetryAfterHintRoundTrip(t *testing.T) {
	base := "returned 429: too many requests"
	withHint := AppendRetryAfterHint(base, 45000)
	if withHint == base {
		t.Fatal("hint was not appended")
	}

	ms, ok := ExtractRetryAfterHint(withHint)
	if !ok || ms != 45000 {
		t.Fatalf("ExtractRetryAfterHint = %d,%v want 45000,true", ms, ok)
	}

	stripped := StripRetryAfterHint(withHint)
	if stripped != base {
		t.Fatalf("StripRetryAfterHint = %q, want %q", stripped, base)
	}
}

func TestAppendRetryAfterHint_NoopOnZero(t *testing.T) {
	base := "returned 500"
	if got := AppendRetryAfterHint(base, 0); got != base {
		t.Fatalf("zero ms should be a no-op, got %q", got)
	}
	if _, ok := ExtractRetryAfterHint(base); ok {
		t.Fatal("no hint present, extract should fail")
	}
	if got := StripRetryAfterHint(base); got != base {
		t.Fatalf("strip on hintless string changed it: %q", got)
	}
}

// A server-provided Retry-After overrides the guessed exponential backoff for a
// plain rate-limit 429, but stays subject to the operator clamp downstream.
func TestCheckFallbackError_HonorsRetryAfterHint(t *testing.T) {
	errText := AppendRetryAfterHint("returned 429: rate limit", 47000)
	fb := CheckFallbackError(429, errText, 0)
	if !fb.ShouldFallback {
		t.Fatal("want fallback")
	}
	if fb.CooldownMs != 47000 {
		t.Fatalf("CooldownMs = %d, want 47000 (honored Retry-After)", fb.CooldownMs)
	}
}

// Without a hint, 429 keeps the existing exponential backoff behavior.
func TestCheckFallbackError_NoHintKeepsBackoff(t *testing.T) {
	fb := CheckFallbackError(429, "returned 429: rate limit", 0)
	if fb.CooldownMs != GetQuotaCooldown(0) {
		t.Fatalf("CooldownMs = %d, want %d (backoff)", fb.CooldownMs, GetQuotaCooldown(0))
	}
}

// A hard-quota exhaustion is a long sideline regardless of a (typically absent)
// hint; the exhaustion branch takes precedence over per-request rate backoff.
func TestCheckFallbackError_HardQuotaBeatsBackoff(t *testing.T) {
	fb := CheckFallbackError(429, "returned 429: monthly limit exceeded", 0)
	if fb.CooldownMs != CooldownQuotaExhausted {
		t.Fatalf("CooldownMs = %d, want %d (hard quota)", fb.CooldownMs, CooldownQuotaExhausted)
	}
	if fb.NewBackoffLevel != BackoffMaxLevel {
		t.Fatalf("NewBackoffLevel = %d, want %d", fb.NewBackoffLevel, BackoffMaxLevel)
	}
}
