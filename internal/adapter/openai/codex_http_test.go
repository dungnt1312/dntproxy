package openai

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCodexResponseHeaderTimeoutDefaultAndEnvOverride(t *testing.T) {
	t.Setenv("DNTPROXY_CODEX_RESPONSE_HEADER_TIMEOUT_MS", "")
	if got := codexResponseHeaderTimeout(); got != 120*time.Second {
		t.Fatalf("default timeout = %v, want 120s", got)
	}

	t.Setenv("DNTPROXY_CODEX_RESPONSE_HEADER_TIMEOUT_MS", "45000")
	if got := codexResponseHeaderTimeout(); got != 45*time.Second {
		t.Fatalf("env timeout = %v, want 45s", got)
	}

	t.Setenv("DNTPROXY_CODEX_RESPONSE_HEADER_TIMEOUT_MS", "bad")
	if got := codexResponseHeaderTimeout(); got != 120*time.Second {
		t.Fatalf("invalid env timeout = %v, want default 120s", got)
	}
}

func TestNewCodexHTTPClientUsesCodexHeaderTimeout(t *testing.T) {
	t.Setenv("DNTPROXY_CODEX_RESPONSE_HEADER_TIMEOUT_MS", "90000")

	client := newCodexHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client transport = %T, want *http.Transport", client.Transport)
	}
	if transport.ResponseHeaderTimeout != 90*time.Second {
		t.Fatalf("ResponseHeaderTimeout = %v, want 90s", transport.ResponseHeaderTimeout)
	}
	if client.Timeout != 0 {
		t.Fatalf("client Timeout = %v, want no overall timeout for streaming", client.Timeout)
	}
}

func TestShouldRetryCodexRequestOnlyForHeaderTimeout(t *testing.T) {
	if !shouldRetryCodexRequest(errors.New("Post \"https://chatgpt.com/backend-api/codex/responses\": http2: timeout awaiting response headers")) {
		t.Fatal("should retry response header timeout")
	}
	if shouldRetryCodexRequest(errors.New("context deadline exceeded")) {
		t.Fatal("should not retry generic context deadline")
	}
	if shouldRetryCodexRequest(errors.New("HTTP 401: token expired")) {
		t.Fatal("should not retry auth errors")
	}
	if shouldRetryCodexRequest(nil) {
		t.Fatal("should not retry nil error")
	}
}

func TestCodexRetryAttemptsDefaultAndEnvOverride(t *testing.T) {
	t.Setenv("DNTPROXY_CODEX_RETRY_ATTEMPTS", "")
	if got := codexRetryAttempts(); got != 1 {
		t.Fatalf("default retry attempts = %d, want 1", got)
	}

	t.Setenv("DNTPROXY_CODEX_RETRY_ATTEMPTS", "2")
	if got := codexRetryAttempts(); got != 2 {
		t.Fatalf("env retry attempts = %d, want 2", got)
	}

	t.Setenv("DNTPROXY_CODEX_RETRY_ATTEMPTS", "bad")
	if got := codexRetryAttempts(); got != 1 {
		t.Fatalf("invalid env retry attempts = %d, want default 1", got)
	}
}

func TestCodexRequestWithRetryRetriesHeaderTimeoutOnce(t *testing.T) {
	attempts := 0
	do := func(*http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("http2: timeout awaiting response headers")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	}

	req, err := http.NewRequest("POST", codexResponsesURL, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := doCodexRequestWithRetry(req, 1, 0, do)
	if err != nil {
		t.Fatalf("doCodexRequestWithRetry returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("response = %#v, want 200", resp)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestCodexRequestWithRetryDoesNotRetryNonHeaderTimeout(t *testing.T) {
	attempts := 0
	do := func(*http.Request) (*http.Response, error) {
		attempts++
		return nil, errors.New("connection refused")
	}

	req, err := http.NewRequest("POST", codexResponsesURL, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = doCodexRequestWithRetry(req, 1, 0, do)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
