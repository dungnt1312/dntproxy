package domain

import "testing"

func TestCheckFallbackError_ImproperlyFormedIsNonFallback(t *testing.T) {
	r := CheckFallbackError(400, "improperly formed request", 0)
	if r.ShouldFallback || r.CooldownMs != 0 {
		t.Fatalf("want no fallback/cooldown, got %+v", r)
	}
}

func TestCheckFallbackError_503OverloadedIsQuotaBackoff(t *testing.T) {
	r := CheckFallbackError(503, "overloaded", 0)
	if !r.ShouldFallback {
		t.Fatal("want fallback")
	}
	if r.CooldownMs != GetQuotaCooldown(0) {
		t.Fatalf("want quota cooldown %d got %d", GetQuotaCooldown(0), r.CooldownMs)
	}
	if r.NewBackoffLevel != 1 {
		t.Fatalf("want backoff 1 got %d", r.NewBackoffLevel)
	}
}

func TestCheckFallbackError_503EmptyIsTransient(t *testing.T) {
	r := CheckFallbackError(503, "", 0)
	if !r.ShouldFallback || r.CooldownMs != CooldownTransient {
		t.Fatalf("want transient fallback, got %+v", r)
	}
}

func TestCheckFallbackError_ModelEntitlement(t *testing.T) {
	r := CheckFallbackError(403, "model_not_entitled", 2)
	if !r.ShouldFallback || !r.ModelOnly || r.CooldownMs != CooldownPaymentReq {
		t.Fatalf("want model-only payment cooldown, got %+v", r)
	}
}

func TestIsQuotaExceeded(t *testing.T) {
	if !IsQuotaExceeded(429, "rate limit") {
		t.Fatal("429 should be quota")
	}
	if !IsQuotaExceeded(403, "model_not_entitled") {
		t.Fatal("entitlement should count as quota for IsQuotaExceeded")
	}
	if IsQuotaExceeded(400, "invalid request") {
		t.Fatal("non_fallback is not quota")
	}
}
