package http

import (
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/compressor"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// slowStream stays silent for `delay`, then yields its lines.
type slowStream struct {
	delay time.Duration
	lines []string
	slept bool
}

func (s *slowStream) Read(p []byte) (int, error) {
	if !s.slept {
		time.Sleep(s.delay)
		s.slept = true
	}
	if len(s.lines) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.lines[0])
	s.lines = s.lines[1:]
	return n, nil
}

func (s *slowStream) Close() error { return nil }

type streamChatService struct{ stream func() io.ReadCloser }

func (s streamChatService) HandleChat(_ []byte, _ string, _ string, _ *port.APIKeyPolicy, _ ...port.RequestMetadata) *port.ChatResult {
	return &port.ChatResult{Stream: s.stream(), StatusCode: 200}
}

func withKeepAlive(t *testing.T, d time.Duration) {
	t.Helper()
	old := sseKeepAliveInterval
	sseKeepAliveInterval = d
	t.Cleanup(func() { sseKeepAliveInterval = old })
}

const openAIStream = `data: {"choices":[{"index":0,"delta":{"role":"assistant","content":"pong"}}]}` + "\n\n" +
	`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
	"data: [DONE]\n\n"

func serve(t *testing.T, path string, h gin.HandlerFunc, body string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST(path, h)
	req := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func TestChatStreamSendsKeepAliveWhileUpstreamIsSilent(t *testing.T) {
	withKeepAlive(t, 30*time.Millisecond)
	svc := streamChatService{stream: func() io.ReadCloser {
		return &slowStream{delay: 200 * time.Millisecond, lines: []string{openAIStream}}
	}}
	out := serve(t, "/v1/chat/completions", chatHandler(svc, nil, compressor.New(compressor.Options{})),
		`{"model":"kiro/claude-opus-5.5","stream":true,"messages":[{"role":"user","content":"hi"}]}`)

	n := strings.Count(out, sseKeepAliveLine)
	if n < 2 {
		t.Fatalf("expected keep-alives during a 200ms silence, got %d:\n%s", n, out)
	}
	// Keep-alives come before the real data, and the data is intact.
	if strings.Index(out, sseKeepAliveLine) > strings.Index(out, `"pong"`) {
		t.Fatalf("keep-alive should precede the first token:\n%s", out)
	}
	if !strings.Contains(out, `"content":"pong"`) || !strings.HasSuffix(strings.TrimSpace(out), "data: [DONE]") {
		t.Fatalf("stream payload damaged:\n%s", out)
	}
}

func TestChatStreamNoKeepAliveWhenDataFlows(t *testing.T) {
	withKeepAlive(t, 500*time.Millisecond)
	svc := streamChatService{stream: func() io.ReadCloser {
		return &slowStream{lines: []string{openAIStream}}
	}}
	out := serve(t, "/v1/chat/completions", chatHandler(svc, nil, compressor.New(compressor.Options{})),
		`{"model":"m","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	if strings.Contains(out, sseKeepAliveLine) {
		t.Fatalf("no keep-alive expected for a fast stream:\n%s", out)
	}
}

func TestKeepAliveDisabledWithZero(t *testing.T) {
	withKeepAlive(t, 0)
	svc := streamChatService{stream: func() io.ReadCloser {
		return &slowStream{delay: 100 * time.Millisecond, lines: []string{openAIStream}}
	}}
	out := serve(t, "/v1/chat/completions", chatHandler(svc, nil, compressor.New(compressor.Options{})),
		`{"model":"m","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	if strings.Contains(out, sseKeepAliveLine) {
		t.Fatalf("keep-alive must be off when the interval is 0:\n%s", out)
	}
}

func TestMessagesStreamSendsKeepAliveWhileUpstreamIsSilent(t *testing.T) {
	withKeepAlive(t, 30*time.Millisecond)
	svc := streamChatService{stream: func() io.ReadCloser {
		return &slowStream{delay: 200 * time.Millisecond, lines: []string{openAIStream}}
	}}
	out := serve(t, "/v1/messages", messagesHandler(svc, nil, compressor.New(compressor.Options{})),
		`{"model":"kiro/claude-opus-5.5","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hi"}]}`)

	if strings.Count(out, sseKeepAliveLine) < 2 {
		t.Fatalf("expected keep-alives on /v1/messages:\n%s", out)
	}
	// Anthropic event order is preserved around the comments.
	for _, ev := range []string{"event: message_start", "event: content_block_start", "event: content_block_delta", "event: message_stop"} {
		if !strings.Contains(out, ev) {
			t.Fatalf("missing %q:\n%s", ev, out)
		}
	}
	if strings.Index(out, "event: message_start") > strings.Index(out, sseKeepAliveLine) {
		t.Fatalf("message_start must come first:\n%s", out)
	}
	if strings.LastIndex(out, sseKeepAliveLine) > strings.Index(out, "event: message_stop") {
		t.Fatalf("no keep-alive may follow message_stop:\n%s", out)
	}
}
