package commandcode

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

type testRequestLogger struct {
	usageInput  int
	usageOutput int
	usageSource string
}

func (l *testRequestLogger) Upstream(url, method string, status int, duration time.Duration, err error) {
}
func (l *testRequestLogger) SetUsage(input, output int, source string) {
	l.usageInput = input
	l.usageOutput = output
	l.usageSource = source
}
func (l *testRequestLogger) SetBodies(reqBody, respBody string) {}

func TestExecutorExecuteTranslatesNDJSON(t *testing.T) {
	var gotAuth, gotVersion, gotPath string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("x-command-code-version")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"type\":\"text-delta\",\"text\":\"hi\"}\n"))
		_, _ = w.Write([]byte("{\"type\":\"finish\",\"finishReason\":\"stop\",\"totalUsage\":{\"inputTokens\":4,\"outputTokens\":1}}\n"))
	}))
	defer server.Close()

	logger := &testRequestLogger{}
	reader, status, err := NewExecutor().Execute(context.Background(), "deepseek-v4-pro", []byte(`{"model":"cmc/deepseek-v4-pro","messages":[{"role":"user","content":"hello"}]}`), &domain.Credentials{
		Provider: "commandcode",
		APIKey:   "user_test",
		BaseURL:  server.URL,
	}, logger)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	defer reader.Close()
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if gotPath != "/alpha/generate" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer user_test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotVersion == "" {
		t.Fatal("missing x-command-code-version")
	}
	if !strings.Contains(gotBody, `"stream":true`) || !strings.Contains(gotBody, `"deepseek/deepseek-v4-pro"`) {
		t.Fatalf("body = %s", gotBody)
	}
	if !strings.Contains(string(out), `"content":"hi"`) || !strings.Contains(string(out), "data: [DONE]") {
		t.Fatalf("stream = %s", out)
	}
	if logger.usageInput != 4 || logger.usageOutput != 1 || logger.usageSource != "commandcode_usage" {
		t.Fatalf("usage = %+v", logger)
	}
}

func TestExecutorRequiresAPIKey(t *testing.T) {
	_, status, err := NewExecutor().Execute(context.Background(), "m", []byte(`{"messages":[{"role":"user","content":"hi"}]}`), &domain.Credentials{}, &testRequestLogger{})
	if err == nil || status != http.StatusUnauthorized {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

func TestExecutorPreContentStreamErrorReturns502(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"type\":\"reasoning\",\"text\":\"wait\"}\n"))
		_, _ = w.Write([]byte("{\"type\":\"error\",\"error\":{\"message\":\"Invalid error response format: Gateway request failed\"}}\n"))
	}))
	defer server.Close()

	reader, status, err := NewExecutor().Execute(context.Background(), "deepseek-v4-flash", []byte(`{"model":"cmc/deepseek/deepseek-v4-flash","messages":[{"role":"user","content":"hello"}]}`), &domain.Credentials{
		APIKey:  "user_test",
		BaseURL: server.URL,
	}, &testRequestLogger{})
	if reader != nil {
		reader.Close()
	}
	if status != http.StatusBadGateway {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if err == nil || !strings.Contains(err.Error(), "Gateway request failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecutorMidStreamErrorStillReturns200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"type\":\"text-delta\",\"text\":\"hi\"}\n"))
		_, _ = w.Write([]byte("{\"type\":\"error\",\"error\":{\"message\":\"Invalid error response format: Gateway request failed\"}}\n"))
	}))
	defer server.Close()

	reader, status, err := NewExecutor().Execute(context.Background(), "deepseek-v4-pro", []byte(`{"model":"cmc/deepseek-v4-pro","messages":[{"role":"user","content":"hello"}]}`), &domain.Credentials{
		APIKey:  "user_test",
		BaseURL: server.URL,
	}, &testRequestLogger{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	defer reader.Close()
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !strings.Contains(string(out), `"content":"hi"`) {
		t.Fatalf("missing content in %s", out)
	}
	if !strings.Contains(string(out), "Gateway request failed") {
		t.Fatalf("missing stream error in %s", out)
	}
}

func TestExecutorPropagatesUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer server.Close()
	_, status, err := NewExecutor().Execute(context.Background(), "m", []byte(`{"messages":[{"role":"user","content":"hi"}]}`), &domain.Credentials{
		APIKey:  "user_bad",
		BaseURL: server.URL,
	}, &testRequestLogger{})
	if err == nil || status != http.StatusUnauthorized {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("err = %v", err)
	}
}
