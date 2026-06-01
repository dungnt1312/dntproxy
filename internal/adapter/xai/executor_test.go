package xai

import (
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
	reqBody     string
	respBody    string
}

func (l *testRequestLogger) Upstream(url, method string, status int, duration time.Duration, err error) {
}
func (l *testRequestLogger) SetUsage(input, output int, source string) {
	l.usageInput = input
	l.usageOutput = output
	l.usageSource = source
}
func (l *testRequestLogger) SetBodies(reqBody, respBody string) {
	if reqBody != "" {
		l.reqBody = reqBody
	}
	if respBody != "" {
		l.respBody = respBody
	}
}

func TestExecutorExecuteStreamsResponsesAsOpenAIChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("path = %q, want /responses", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("Authorization = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"grok-4.3"`) {
			t.Fatalf("request body = %s", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hi\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":3,\"output_tokens\":1,\"total_tokens\":4}}}\n\n"))
	}))
	defer server.Close()

	logger := &testRequestLogger{}
	reader, status, err := NewExecutor().Execute("grok-4.3", []byte(`{"model":"grok/grok-4.3","stream":true,"messages":[{"role":"user","content":"hello"}]}`), &domain.Credentials{
		Provider:    "xai",
		AccessToken: "access-token",
		BaseURL:     server.URL,
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
	if !strings.Contains(string(out), `"content":"hi"`) {
		t.Fatalf("stream = %s", out)
	}
	if !strings.Contains(string(out), "data: [DONE]") {
		t.Fatalf("stream missing DONE = %s", out)
	}
	if logger.usageInput != 3 || logger.usageOutput != 1 || logger.usageSource != "xai_usage" {
		t.Fatalf("usage = %d/%d/%s", logger.usageInput, logger.usageOutput, logger.usageSource)
	}
}

func TestExecutorRejectsUnsupportedTool(t *testing.T) {
	reader, status, err := NewExecutor().Execute("grok-4.3", []byte(`{"model":"grok/grok-4.3","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"web_search"}]}`), &domain.Credentials{}, &testRequestLogger{})
	if reader != nil {
		reader.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "unsupported tool type") {
		t.Fatalf("error = %v, want unsupported tool type", err)
	}
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}
