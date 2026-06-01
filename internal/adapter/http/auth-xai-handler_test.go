package http

import "testing"

func TestParseXAICallbackAcceptsRawCode(t *testing.T) {
	code, state, err := parseXAICallback("", "", "unWqpflPgW-JPi3ywvkwamyYTMbQI0k8mlwnXwR6chgdk0_v4Gsl5NKiFN8WoKZOFnaZgUZPg7702VcVGePHxw")
	if err != nil {
		t.Fatalf("parseXAICallback() error = %v", err)
	}
	if code != "unWqpflPgW-JPi3ywvkwamyYTMbQI0k8mlwnXwR6chgdk0_v4Gsl5NKiFN8WoKZOFnaZgUZPg7702VcVGePHxw" {
		t.Fatalf("code = %q", code)
	}
	if state != "" {
		t.Fatalf("state = %q, want empty", state)
	}
}

func TestParseXAICallbackExtractsCodeAndStateFromURL(t *testing.T) {
	code, state, err := parseXAICallback("", "", "http://127.0.0.1:56121/callback?code=abc123&state=state456")
	if err != nil {
		t.Fatalf("parseXAICallback() error = %v", err)
	}
	if code != "abc123" {
		t.Fatalf("code = %q, want abc123", code)
	}
	if state != "state456" {
		t.Fatalf("state = %q, want state456", state)
	}
}

func TestParseXAICallbackPrefersExplicitCode(t *testing.T) {
	code, state, err := parseXAICallback("direct-code", "direct-state", "")
	if err != nil {
		t.Fatalf("parseXAICallback() error = %v", err)
	}
	if code != "direct-code" {
		t.Fatalf("code = %q, want direct-code", code)
	}
	if state != "direct-state" {
		t.Fatalf("state = %q, want direct-state", state)
	}
}
