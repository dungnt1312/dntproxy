package domain

import "testing"

func TestClassifyUpstream(t *testing.T) {
	tests := []struct {
		status int
		body   string
		want   UpstreamClass
	}{
		{400, "invalid request", UpstreamNonFallback},
		{400, "improperly formed request", UpstreamNonFallback},
		{429, "rate limit exceeded", UpstreamQuota},
		// Body "overloaded" is a quota keyword and wins over bare status class.
		{503, "overloaded", UpstreamQuota},
		{503, "", UpstreamTransient},
		{401, "unauthorized", UpstreamAuth},
		{403, "model_not_entitled", UpstreamModelEntitlement},
		{500, "internal", UpstreamTransient},
		{408, "timeout", UpstreamTransient},
	}
	for _, tt := range tests {
		if got := ClassifyUpstream(tt.status, tt.body); got != tt.want {
			t.Fatalf("status=%d body=%q got=%s want=%s", tt.status, tt.body, got, tt.want)
		}
	}
}

func TestIsRetryableUpstream(t *testing.T) {
	if !IsRetryableUpstream(503, "") {
		t.Fatal("503 should be retryable")
	}
	if !IsRetryableUpstream(500, "internal") {
		t.Fatal("500 transient should be retryable")
	}
	if !IsRetryableUpstream(408, "timeout") {
		t.Fatal("408 transient should be retryable")
	}
	if IsRetryableUpstream(400, "invalid json") {
		t.Fatal("400 invalid should not be retryable")
	}
	if !IsRetryableUpstream(429, "quota exceeded") {
		t.Fatal("quota should still allow account/model failover")
	}
}
