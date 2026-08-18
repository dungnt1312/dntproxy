// Package cline implements the ClinePass provider executor.
// ClinePass provides OpenAI-compatible API at api.cline.bot.
// Models use the cline-pass/ prefix internally, so we just strip the dntproxy
// routing prefix (cline/ or cl/) and forward the rest unchanged.
package cline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// Executor handles forwarding requests to the ClinePass API.
type Executor struct{}

// NewExecutor creates a new ClinePass executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Execute sends a request to ClinePass API.
// Strips the "cl/" routing prefix from the model name before forwarding.
func (e *Executor) Execute(ctx context.Context, model string, body []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	baseURL := credentials.BaseURL
	if baseURL == "" {
		cfg := domain.GetProviderConfig(credentials.Provider)
		baseURL = cfg.DefaultBaseURL
	}
	baseURL = domain.StripVersionSuffix(baseURL)

	cfg := domain.GetProviderConfig(credentials.Provider)
	chatPath := cfg.ChatPath
	url := baseURL + chatPath

	// Strip the dntproxy routing prefix from model name in the request body.
	// Cline API expects models as "cline-pass/glm-5.2" etc.
	forwardBody, err := stripClinePrefix(body)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(forwardBody))
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("create cline request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Auth: API key takes priority
	apiKey := credentials.APIKey
	if apiKey == "" {
		apiKey = credentials.AccessToken
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	reqlog.SetBodies(shared.PrepareLoggedBody(forwardBody), "")

	start := time.Now()
	resp, err := shared.HTTP1StreamingClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(url, http.MethodPost, http.StatusBadGateway, duration, err)
		return nil, http.StatusBadGateway, fmt.Errorf("cline request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, errRead := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()

		respBodyStr := "Unknown error"
		if errRead == nil {
			respBodyStr = string(bodyBytes)
		}

		reqlog.SetBodies("", shared.PrepareLoggedBody(bodyBytes))
		errUpstream := fmt.Errorf("%s", respBodyStr)
		reqlog.Upstream(url, http.MethodPost, resp.StatusCode, duration, errUpstream)

		return nil, resp.StatusCode, fmt.Errorf("cline returned %d: %s", resp.StatusCode, respBodyStr)
	}

	reqlog.Upstream(url, http.MethodPost, resp.StatusCode, duration, nil)

	// Sniff stream to extract usage and preview at the end
	sniffer := &clineStreamSniffer{
		ReadCloser: resp.Body,
		onClose: func(bodyBytes []byte) {
			// Log raw SSE tail for debugging when usage not found
			if usage := extractUsage(bodyBytes); usage != nil {
				reqlog.SetUsage(usage.PromptTokens, usage.CompletionTokens, "sse_usage")
			} else {
				// Log last 2KB of stream to debug missing usage
				tail := bodyBytes
				if len(tail) > 2048 {
					tail = tail[len(tail)-2048:]
				}
				log.Printf("[CLINE] usage not found in stream, tail: %s", string(tail))
			}
			if preview, _ := extractResponsePreview(bodyBytes); preview != "" {
				reqlog.SetBodies("", preview)
			}
		},
	}

	return sniffer, http.StatusOK, nil
}

// stripClinePrefix removes the dntproxy routing prefix from the model field.
// E.g. "cline/cline-pass/glm-5.2" -> "cline-pass/glm-5.2".
func stripClinePrefix(body []byte) ([]byte, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse request body: %w", err)
	}

	if model, ok := payload["model"].(string); ok {
		model = strings.TrimPrefix(model, "cline/")
		model = strings.TrimPrefix(model, "cl/")
		payload["model"] = model
	}
	payload["stream"] = true

	rewritten, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal rewritten body: %w", err)
	}
	return rewritten, nil
}

// clineStreamSniffer wraps a ReadCloser to capture the full SSE stream for
// usage extraction after the stream is consumed.
type clineStreamSniffer struct {
	io.ReadCloser
	onClose func([]byte)
	mu      sync.Mutex
	buf     bytes.Buffer
	closed  bool
}

const clineMaxBuffer = 100 * 1024 // 100KB cap like OpenAI sniffer

func (s *clineStreamSniffer) Read(p []byte) (int, error) {
	n, err := s.ReadCloser.Read(p)
	if n > 0 {
		s.mu.Lock()
		if s.buf.Len() < clineMaxBuffer {
			s.buf.Write(p[:n])
		}
		s.mu.Unlock()
	}
	return n, err
}

func (s *clineStreamSniffer) Close() error {
	err := s.ReadCloser.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && s.onClose != nil {
		s.closed = true
		s.onClose(s.buf.Bytes())
	}
	return err
}

// usageData holds token usage extracted from SSE data.
type usageData struct {
	PromptTokens     int
	CompletionTokens int
}

// extractUsage parses SSE data lines to find usage information.
// Tries multiple formats: OpenAI standard usage object, last chunk with usage,
// and x-kimi / x-usage headers.
func extractUsage(data []byte) *usageData {
	lines := strings.Split(string(data), "\n")

	// Try OpenAI standard: {"usage": {"prompt_tokens": ..., "completion_tokens": ...}}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		dataStr := strings.TrimPrefix(line, "data: ")
		if dataStr == "[DONE]" {
			continue
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			continue
		}
		if usage, ok := chunk["usage"].(map[string]interface{}); ok {
			u := &usageData{}
			if pt, ok := usage["prompt_tokens"].(float64); ok {
				u.PromptTokens = int(pt)
			}
			if ct, ok := usage["completion_tokens"].(float64); ok {
				u.CompletionTokens = int(ct)
			}
			// Also try camelCase variants
			if pt, ok := usage["promptTokens"].(float64); ok && u.PromptTokens == 0 {
				u.PromptTokens = int(pt)
			}
			if ct, ok := usage["completionTokens"].(float64); ok && u.CompletionTokens == 0 {
				u.CompletionTokens = int(ct)
			}
			// Also try input_tokens/output_tokens
			if pt, ok := usage["input_tokens"].(float64); ok && u.PromptTokens == 0 {
				u.PromptTokens = int(pt)
			}
			if ct, ok := usage["output_tokens"].(float64); ok && u.CompletionTokens == 0 {
				u.CompletionTokens = int(ct)
			}
			// Also try total_tokens (estimate split)
			if tt, ok := usage["total_tokens"].(float64); ok && u.PromptTokens == 0 && u.CompletionTokens == 0 {
				u.PromptTokens = int(tt*0.8)
				u.CompletionTokens = int(tt*0.2)
			}
			if u.PromptTokens > 0 || u.CompletionTokens > 0 {
				return u
			}
		}
	}

	return nil
}

// extractResponsePreview extracts a preview of the assistant's content.
func extractResponsePreview(data []byte) (string, bool) {
	lines := strings.Split(string(data), "\n")
	var content strings.Builder
	maxLen := 512
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		dataStr := strings.TrimPrefix(line, "data: ")
		if dataStr == "[DONE]" {
			continue
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			continue
		}
		choices, _ := chunk["choices"].([]interface{})
		if len(choices) == 0 {
			continue
		}
		first, _ := choices[0].(map[string]interface{})
		delta, _ := first["delta"].(map[string]interface{})
		text, _ := delta["content"].(string)
		if text != "" {
			for _, r := range text {
				if content.Len() >= maxLen {
					return content.String(), true
				}
				content.WriteRune(r)
			}
		}
	}
	if content.Len() > 0 {
		return content.String(), true
	}
	return "", false
}
