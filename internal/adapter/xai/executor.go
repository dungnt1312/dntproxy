package xai

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// Executor handles Grok Build OAuth requests through xAI's Responses API.
type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
}

func (e *Executor) Execute(ctx context.Context, model string, body []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	translatedBody, err := TranslateChatToResponses(model, body)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	baseURL := strings.TrimRight(credentials.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.x.ai/v1"
	}
	targetURL := baseURL + "/responses"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(translatedBody))
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("create xai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if credentials.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	}

	reqlog.SetBodies(shared.PrepareLoggedBody(translatedBody), "")
	start := time.Now()
	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		reqlog.Upstream(targetURL, http.MethodPost, http.StatusBadGateway, duration, err)
		return nil, http.StatusBadGateway, fmt.Errorf("xai request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, errRead := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()
		respBody := "Unknown error"
		if errRead == nil {
			respBody = string(bodyBytes)
		}
		reqlog.SetBodies("", shared.PrepareLoggedBody(bodyBytes))
		errUpstream := fmt.Errorf("%s", respBody)
		reqlog.Upstream(targetURL, http.MethodPost, resp.StatusCode, duration, errUpstream)
		return nil, resp.StatusCode, fmt.Errorf("xai returned %d: %s", resp.StatusCode, respBody)
	}
	reqlog.Upstream(targetURL, http.MethodPost, resp.StatusCode, duration, nil)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer resp.Body.Close()

		state := NewResponseState(model)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 50*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				continue
			}
			translated := TranslateResponsesEvent([]byte(data), state)
			if translated != "" {
				_, _ = pw.Write([]byte(translated))
			}
		}
		if err := scanner.Err(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if !state.FinishReasonSent {
			finish := "stop"
			_, _ = pw.Write([]byte(formatChunk(state, map[string]interface{}{}, &finish)))
			_, _ = pw.Write([]byte("data: [DONE]\n\n"))
		}
		if state.Usage.PromptTokens > 0 || state.Usage.CompletionTokens > 0 {
			reqlog.SetUsage(state.Usage.PromptTokens, state.Usage.CompletionTokens, "xai_usage")
		}
	}()

	return pr, http.StatusOK, nil
}
