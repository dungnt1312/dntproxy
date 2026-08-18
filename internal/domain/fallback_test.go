package domain

import "testing"

func TestCheckFallbackErrorResponseHeaderTimeoutNoPenalty(t *testing.T) {
	// "http2: timeout awaiting response headers" is a transient upstream stall:
	// fall back for this request but never penalize the account.
	fb := CheckFallbackError(502, `Post "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse": http2: timeout awaiting response headers`, 3)
	if !fb.ShouldFallback {
		t.Fatal("ShouldFallback = false, want true")
	}
	if fb.CooldownMs != 0 {
		t.Fatalf("CooldownMs = %d, want 0 (no penalty)", fb.CooldownMs)
	}
}

func TestCheckFallbackError403(t *testing.T) {
	// model_not_entitled keeps the model-only 5-min cooldown.
	entitled := CheckFallbackError(403, "returned 403: model_not_entitled claude-4-opus", 0)
	if !entitled.ShouldFallback || !entitled.ModelOnly {
		t.Fatalf("entitlement error: ShouldFallback=%v ModelOnly=%v, want true/true", entitled.ShouldFallback, entitled.ModelOnly)
	}
	if entitled.CooldownMs != CooldownPaymentReq {
		t.Fatalf("entitlement CooldownMs = %d, want %d", entitled.CooldownMs, CooldownPaymentReq)
	}

	// Plain 403 (transient refusal) falls back without penalty.
	plain := CheckFallbackError(403, "returned 403: some generic refusal", 0)
	if !plain.ShouldFallback {
		t.Fatal("plain 403: ShouldFallback = false, want true")
	}
	if plain.CooldownMs != 0 {
		t.Fatalf("plain 403 CooldownMs = %d, want 0 (no penalty)", plain.CooldownMs)
	}
	if plain.ModelOnly {
		t.Fatal("plain 403: ModelOnly = true, want false")
	}

	// "not available" is too broad to treat as entitlement.
	broad := CheckFallbackError(403, "service not available", 0)
	if broad.ModelOnly {
		t.Fatal("403 'not available' must not lock the model for 5 minutes")
	}
}

func TestCheckFallbackErrorStatusCooldowns(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		errText    string
		wantFB     bool
		wantCool   int
	}{
		{name: "402 payment required", status: 402, errText: "", wantFB: true, wantCool: CooldownPaymentReq},
		{name: "401 unauthorized", status: 401, errText: "", wantFB: true, wantCool: CooldownUnauthorized},
		{name: "404 not found", status: 404, errText: "", wantFB: true, wantCool: CooldownNotFound},
		{name: "500 transient", status: 500, errText: "", wantFB: true, wantCool: CooldownTransient}, // ModelOnly checked below
		{name: "502 transient", status: 502, errText: "", wantFB: true, wantCool: CooldownTransient},
		{name: "429 backoff", status: 429, errText: "", wantFB: true, wantCool: GetQuotaCooldown(0)},
		{name: "400 client error no fallback", status: 400, errText: "invalid request", wantFB: false, wantCool: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := CheckFallbackError(tc.status, tc.errText, 0)
			if fb.ShouldFallback != tc.wantFB {
				t.Fatalf("ShouldFallback = %v, want %v", fb.ShouldFallback, tc.wantFB)
			}
			if fb.CooldownMs != tc.wantCool {
				t.Fatalf("CooldownMs = %d, want %d", fb.CooldownMs, tc.wantCool)
			}
		})
	}
}

func TestCheckFallbackErrorBackoffEscalation(t *testing.T) {
	fb := CheckFallbackError(429, "", 2)
	if fb.NewBackoffLevel != 3 {
		t.Fatalf("NewBackoffLevel = %d, want 3", fb.NewBackoffLevel)
	}
	// Timeout errors must not change the backoff level.
	tb := CheckFallbackError(502, "http2: timeout awaiting response headers", 5)
	if tb.NewBackoffLevel != 5 {
		t.Fatalf("timeout NewBackoffLevel = %d, want 5", tb.NewBackoffLevel)
	}

	five := CheckFallbackError(500, "", 0)
	if !five.ModelOnly {
		t.Fatal("5xx should lock the model only, not the whole account")
	}
}
