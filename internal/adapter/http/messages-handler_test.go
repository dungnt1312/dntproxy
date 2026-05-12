package http

import (
	"context"
	"errors"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/compressor"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

type fakeMessagesChatService struct {
	called bool
}

func (s *fakeMessagesChatService) HandleChat(_ []byte, _ string, _ string, _ *port.APIKeyPolicy, _ ...port.RequestMetadata) *port.ChatResult {
	s.called = true
	return &port.ChatResult{StatusCode: stdhttp.StatusServiceUnavailable, Error: "unexpected call"}
}

type readErrorStream struct {
	lines []string
	err   error
}

func (s *readErrorStream) Read(p []byte) (int, error) {
	if len(s.lines) == 0 {
		return 0, s.err
	}
	line := s.lines[0]
	s.lines = s.lines[1:]
	copy(p, line)
	return len(line), nil
}

func (s *readErrorStream) Close() error {
	return nil
}

func TestMessagesHandler_OversizedBodyReturns413(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatSvc := &fakeMessagesChatService{}
	router := gin.New()
	router.POST("/v1/messages", messagesHandler(chatSvc, nil, compressor.New(compressor.Options{})))

	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/messages", strings.NewReader(strings.Repeat("x", maxChatBodySize+1)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d body=%s", rec.Code, rec.Body.String())
	}
	if chatSvc.called {
		t.Fatal("chat service must not be called for oversized bodies")
	}
}

func TestHandleNonStreamingMessages_StreamReadErrorReturns502(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	stream := &readErrorStream{
		lines: []string{`data: {"choices":[{"delta":{"content":"partial"}}]}` + "\n"},
		err:   errors.New("read failed"),
	}

	handleNonStreamingMessages(ctx, stream, "anthropic/claude", "12345678-test")

	if rec.Code != stdhttp.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Stream read failed") {
		t.Fatalf("expected stream error response, body=%s", rec.Body.String())
	}
}

func TestHandleStreamingMessages_StreamReadErrorEmitsErrorEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(stdhttp.MethodPost, "/v1/messages", nil)
	stream := &readErrorStream{err: io.ErrUnexpectedEOF}

	handleStreamingMessages(ctx, stream, "anthropic/claude", "12345678-test")

	body := rec.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Fatalf("expected error event, body=%s", body)
	}
	if strings.Contains(body, "event: message_stop") {
		t.Fatalf("must not send normal message_stop after read error, body=%s", body)
	}
}

func TestReadOpenAISSEData_RejectsOversizedLine(t *testing.T) {
	err := readOpenAISSEData(
		strings.NewReader("data: "+strings.Repeat("x", maxOpenAISSELineSize+1)),
		func(_ string) error { return nil },
	)

	if err == nil || !strings.Contains(err.Error(), "SSE line exceeds") {
		t.Fatalf("expected oversized line error, got %v", err)
	}
}

func TestHandleStreamingMessages_CanceledRequestDoesNotEmitMessageStop(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()
	ctx.Request = httptest.NewRequest(stdhttp.MethodPost, "/v1/messages", nil).WithContext(reqCtx)
	stream := &readErrorStream{
		lines: []string{`data: {"choices":[{"delta":{"content":"partial"}}]}` + "\n"},
		err:   io.EOF,
	}

	handleStreamingMessages(ctx, stream, "anthropic/claude", "12345678-test")

	body := rec.Body.String()
	if strings.Contains(body, "event: message_stop") {
		t.Fatalf("must not send message_stop after canceled request, body=%s", body)
	}
	if strings.Contains(body, "event: error") {
		t.Fatalf("must not emit error event after client cancellation, body=%s", body)
	}
}

func TestHandleStreamingMessages_CanceledRequestAfterDoneDoesNotEmitMessageStop(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()
	ctx.Request = httptest.NewRequest(stdhttp.MethodPost, "/v1/messages", nil).WithContext(reqCtx)
	stream := &readErrorStream{
		lines: []string{"data: [DONE]\n"},
		err:   io.EOF,
	}

	handleStreamingMessages(ctx, stream, "anthropic/claude", "12345678-test")

	body := rec.Body.String()
	if strings.Contains(body, "event: message_stop") {
		t.Fatalf("must not send message_stop after canceled done path, body=%s", body)
	}
}
