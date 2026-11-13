package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/compressor"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// rateLimitedMessagesChatService fails selection the way the router does when
// every account is cooling: a 429 plus the earliest moment the pool reopens.
type rateLimitedMessagesChatService struct {
	retryAfter string
}

func (s *rateLimitedMessagesChatService) HandleChat(_ []byte, _ string, _ string, _ *port.APIKeyPolicy, _ ...port.RequestMetadata) *port.ChatResult {
	return &port.ChatResult{
		StatusCode: stdhttp.StatusTooManyRequests,
		Error:      "all accounts rate limited for provider: glm",
		RetryAfter: s.retryAfter,
	}
}

// The Anthropic route must forward the router's recovery hint as Retry-After,
// otherwise Claude Code retries immediately against a pool that is still cooling.
func TestMessagesHandler_RateLimitedPoolSetsRetryAfterHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	retryAfter := time.Now().Add(90 * time.Minute).UTC().Format(time.RFC3339)
	router := gin.New()
	router.POST("/v1/messages", messagesHandler(&rateLimitedMessagesChatService{retryAfter: retryAfter}, nil, compressor.New(compressor.Options{})))

	body := `{"model":"glm/glm-5.3-flash","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (body=%s)", rec.Code, rec.Body.String())
	}
	header := rec.Header().Get("Retry-After")
	if header == "" {
		t.Fatal("missing Retry-After header on a rate-limited pool")
	}
	secs, err := strconv.Atoi(header)
	if err != nil {
		t.Fatalf("Retry-After = %q, want delta-seconds", header)
	}
	// 90 minutes minus a little clock drift.
	if secs < 89*60 || secs > 90*60 {
		t.Fatalf("Retry-After = %d seconds, want ~5400", secs)
	}
	if !strings.Contains(rec.Body.String(), "rate limited") {
		t.Fatalf("error body = %s", rec.Body.String())
	}
}

// A selection error without a cooldown (e.g. no active credentials) must not
// invent a Retry-After header.
func TestMessagesHandler_NoRetryAfterWithoutHint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/v1/messages", messagesHandler(&rateLimitedMessagesChatService{}, nil, compressor.New(compressor.Options{})))

	body := `{"model":"glm/glm-5.3-flash","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Fatalf("Retry-After = %q, want empty when the router has no hint", got)
	}
}

// A Retry-After in the past is meaningless and must be dropped.
func TestWriteRetryAfterHeader_DropsPastAndUnparseableValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := map[string]string{
		"past":        time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
		"unparseable": "2026-10-26 20:04:25",
		"empty":       "",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			writeRetryAfterHeader(ctx, value)
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Fatalf("Retry-After = %q, want empty", got)
			}
		})
	}
}
