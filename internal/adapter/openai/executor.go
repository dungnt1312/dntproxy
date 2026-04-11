package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/logger"
)

const defaultOpenAIBaseURL = "https://api.openai.com"

// Executor handles making requests to OpenAI or OpenAI-compatible APIs.
// Since these APIs are already OpenAI-compatible, we just proxy the request
// with the appropriate auth header — no request/response translation needed.
type Executor struct{}

// NewExecutor creates a new OpenAI executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// isCodexOAuth returns true if this credential is an OpenAI OAuth token
// (from auth.openai.com) which needs to use the Codex Responses API.
func isCodexOAuth(credentials *domain.Credentials) bool {
	// If there's an API key, use standard API (not OAuth)
	if credentials.APIKey != "" {
		return false
	}
	// If there's a custom base URL, user configured their own endpoint
	if credentials.BaseURL != "" {
		return false
	}
	// Check provider-specific data for authMethod == "oauth"
	if credentials.ProviderSpecificData != nil {
		if method, ok := credentials.ProviderSpecificData["authMethod"].(string); ok {
			return method == "oauth"
		}
	}
	// If we have an access token but no API key, it's likely OAuth
	return credentials.AccessToken != "" && credentials.APIKey == ""
}

// Execute sends a request to OpenAI (or compatible) API and returns a streaming reader.
func (e *Executor) Execute(model string, body []byte, credentials *domain.Credentials, requestID string) (io.ReadCloser, int, error) {
	if isCodexOAuth(credentials) {
		return e.executeCodexResponses(model, body, credentials, requestID)
	}
	return e.executeStandard(model, body, credentials, requestID)
}

// executeStandard handles standard OpenAI API key requests (api.openai.com/v1/chat/completions).
func (e *Executor) executeStandard(model string, body []byte, credentials *domain.Credentials, requestID string) (io.ReadCloser, int, error) {
	baseURL := credentials.BaseURL
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}

	url := baseURL + "/v1/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
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

	log.Printf("[OPENAI] --> %s | conn=%s | model=%s | body_size=%d", url, credentials.ConnectionName, model, len(body))
	if apiKey != "" {
		log.Printf("[OPENAI]     Authorization: Bearer %s", shared.MaskedToken(apiKey))
	}
	appLogger := logger.Get()
	appLogger.AddEntry(domain.LogEntry{
		Provider:       "OPENAI",
		Direction:      "outbound",
		Method:         "POST",
		Path:           url,
		ConnectionID:   credentials.ConnectionID,
		ConnectionName: credentials.ConnectionName,
		Model:          model,
		RequestID:      requestID,
		Message:        "OpenAI-compatible request sent",
		BodySize:       len(body),
		RequestBody:    shared.TruncateBody(shared.SanitizeBody(body), 8192),
	})

	start := time.Now()

	// Execute request using shared client (connection reuse, no stream timeout)
	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		errMsg := fmt.Sprintf("openai request failed: %s", err)
		log.Printf("[OPENAI] <-- %s | conn=%s | model=%s | status=502 | duration=%s | error=%s",
			url, credentials.ConnectionName, model, duration, err)
		appLogger.AddEntry(domain.LogEntry{
			Level:          "ERROR",
			Provider:       "OPENAI",
			Direction:      "response",
			Path:           url,
			StatusCode:     502,
			DurationMs:     duration.Milliseconds(),
			ConnectionID:   credentials.ConnectionID,
			ConnectionName: credentials.ConnectionName,
			Model:          model,
			RequestID:      requestID,
			Message:        "OpenAI-compatible request failed",
			Error:          errMsg,
		})
		return nil, 502, fmt.Errorf("openai request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()

		respBodyStr := "Unknown error"
		if err == nil {
			respBodyStr = string(bodyBytes)
		}
		log.Printf("[OPENAI] <-- %s | conn=%s | model=%s | status=%d | duration=%s | error body_size=%d",
			url, credentials.ConnectionName, model, resp.StatusCode, duration, len(bodyBytes))
		log.Printf("[OPENAI] ERROR body: %s", respBodyStr)

		appLogger.AddEntry(domain.LogEntry{
			Level:          "ERROR",
			Provider:       "OPENAI",
			Direction:      "response",
			Path:           url,
			StatusCode:     resp.StatusCode,
			DurationMs:     duration.Milliseconds(),
			ConnectionID:   credentials.ConnectionID,
			ConnectionName: credentials.ConnectionName,
			Model:          model,
			RequestID:      requestID,
			Message:        "OpenAI-compatible request failed",
			BodySize:       len(bodyBytes),
			Error:          respBodyStr,
			ResponseBody:   shared.TruncateBody(shared.SanitizeBody(bodyBytes), 8192),
		})
		return nil, resp.StatusCode, fmt.Errorf("openai returned %d: %s", resp.StatusCode, respBodyStr)
	}

	log.Printf("[OPENAI] <-- %s | conn=%s | model=%s | status=%d | duration=%s | stream_started=true",
		url, credentials.ConnectionName, model, resp.StatusCode, duration)

	appLogger.AddEntry(domain.LogEntry{
		Level:          "INFO",
		Provider:       "OPENAI",
		Direction:      "response",
		Path:           url,
		StatusCode:     resp.StatusCode,
		DurationMs:     duration.Milliseconds(),
		ConnectionID:   credentials.ConnectionID,
		ConnectionName: credentials.ConnectionName,
		Model:          model,
		RequestID:      requestID,
		Message:        "OpenAI-compatible response stream started",
	})

	// Sniff stream to extract usage and preview at the end
	sniffer := &openaiStreamSniffer{
		ReadCloser: resp.Body,
		onClose: func(bodyBytes []byte) {
			if usage := extractUsage(bodyBytes); usage != nil {
				appLogger.AddUsage("OPENAI", requestID, credentials.ConnectionID, credentials.ConnectionName,
					model, usage.PromptTokens, usage.CompletionTokens, "sse_usage")
			}
			if preview, truncated := extractResponsePreview(bodyBytes); preview != "" {
				metadata, _ := json.Marshal(map[string]interface{}{
					"responsePreview": preview,
					"truncated":       truncated,
					"source":          "sse",
				})
				appLogger.AddEntry(domain.LogEntry{
					Provider:       "OPENAI",
					Direction:      "payload",
					ConnectionID:   credentials.ConnectionID,
					ConnectionName: credentials.ConnectionName,
					Model:          model,
					RequestID:      requestID,
					Message:        "Response payload captured",
					MetadataJSON:   string(metadata),
				})
			}
		},
	}

	return sniffer, 200, nil
}

// executeCodexResponses handles OpenAI OAuth tokens via the Codex Responses API.
// Translates: Chat Completions request → Codex Responses API request
// And:        Codex Responses API SSE → Chat Completions SSE
func (e *Executor) executeCodexResponses(model string, body []byte, credentials *domain.Credentials, requestID string) (io.ReadCloser, int, error) {
	// Translate request: Chat Completions → Codex Responses API
	translatedBody, err := TranslateChatToCodexResponses(body)
	if err != nil {
		return nil, 500, fmt.Errorf("translate request to codex format: %w", err)
	}

	url := codexResponsesURL

	req, err := http.NewRequest("POST", url, bytes.NewReader(translatedBody))
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	// Codex CLI headers
	req.Header.Set("originator", "codex-cli")
	req.Header.Set("User-Agent", "codex-cli/1.0.18 (macOS; arm64)")
	// Random session_id per request (matches real Codex CLI behavior)
	req.Header.Set("session_id", fmt.Sprintf("%d-%s", time.Now().UnixMilli(), randomAlphaNum(9)))

	log.Printf("[CODEX] --> %s | conn=%s | model=%s | body_size=%d", url, credentials.ConnectionName, model, len(translatedBody))
	log.Printf("[CODEX]     Authorization: Bearer %s", shared.MaskedToken(credentials.AccessToken))

	appLogger := logger.Get()
	appLogger.AddEntry(domain.LogEntry{
		Provider:       "CODEX",
		Direction:      "outbound",
		Method:         "POST",
		Path:           url,
		ConnectionID:   credentials.ConnectionID,
		ConnectionName: credentials.ConnectionName,
		Model:          model,
		RequestID:      requestID,
		Message:        "Codex Responses API request sent",
		BodySize:       len(translatedBody),
		RequestBody:    shared.TruncateBody(shared.SanitizeBody(translatedBody), 8192),
	})

	start := time.Now()

	// Execute request using shared client (connection reuse, no stream timeout)
	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		errMsg := fmt.Sprintf("codex request failed: %s", err)
		log.Printf("[CODEX] <-- %s | conn=%s | model=%s | status=502 | duration=%s | error=%s",
			url, credentials.ConnectionName, model, duration, err)
		appLogger.AddEntry(domain.LogEntry{
			Level:          "ERROR",
			Provider:       "CODEX",
			Direction:      "response",
			Path:           url,
			StatusCode:     502,
			DurationMs:     duration.Milliseconds(),
			ConnectionID:   credentials.ConnectionID,
			ConnectionName: credentials.ConnectionName,
			Model:          model,
			RequestID:      requestID,
			Message:        "Codex Responses API request failed",
			Error:          errMsg,
		})
		return nil, 502, fmt.Errorf("codex request failed: %w", err)
	}

	log.Printf("[CODEX] <-- %s | conn=%s | model=%s | status=%d | duration=%s",
		url, credentials.ConnectionName, model, resp.StatusCode, duration)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()
		respBodyStr := string(bodyBytes)
		log.Printf("[CODEX] ERROR body: %s", respBodyStr)

		appLogger.AddEntry(domain.LogEntry{
			Level:          "ERROR",
			Provider:       "CODEX",
			Direction:      "response",
			Path:           url,
			StatusCode:     resp.StatusCode,
			DurationMs:     duration.Milliseconds(),
			ConnectionID:   credentials.ConnectionID,
			ConnectionName: credentials.ConnectionName,
			Model:          model,
			RequestID:      requestID,
			Message:        "Codex Responses API error",
			Error:          respBodyStr,
			ResponseBody:   shared.TruncateBody(shared.SanitizeBody(bodyBytes), 8192),
			BodySize:       len(bodyBytes),
		})
		return nil, resp.StatusCode, fmt.Errorf("codex returned %d: %s", resp.StatusCode, respBodyStr)
	}

	appLogger.AddEntry(domain.LogEntry{
		Level:          "INFO",
		Provider:       "CODEX",
		Direction:      "response",
		Path:           url,
		StatusCode:     resp.StatusCode,
		DurationMs:     duration.Milliseconds(),
		ConnectionID:   credentials.ConnectionID,
		ConnectionName: credentials.ConnectionName,
		Model:          model,
		RequestID:      requestID,
		Message:        "Codex Responses API streaming started",
	})

	// Create a pipe to transform the Codex SSE stream into Chat Completions SSE
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer resp.Body.Close()
		state := NewCodexResponseState(model)
		scanner := bufio.NewScanner(resp.Body)
		// Increase scanner buffer for large SSE events
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		currentEvent := ""

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				dataStr := strings.TrimPrefix(line, "data: ")
				if currentEvent != "" {
					translated := TranslateCodexEvent(currentEvent, []byte(dataStr), state)
					if translated != "" {
						pw.Write([]byte(translated))
					}
					currentEvent = ""
				}
				continue
			}

			if line == "" {
				currentEvent = ""
			}
		}

		// If stream ended without response.completed, send finish
		if !state.FinishReasonSent {
			finishReason := "stop"
			if state.ToolCallIndex > 0 {
				finishReason = "tool_calls"
			}
			pw.Write([]byte(formatSSEChunk(state, map[string]interface{}{}, &finishReason)))
			pw.Write([]byte("data: [DONE]\n\n"))
		}
	}()

	return pr, 200, nil
}
