package openai

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

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

	req, err := http.NewRequest("POST", url, io.NopCloser(newBytesReader(body)))
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
		log.Printf("[OPENAI]     Authorization: Bearer %s", maskedToken(apiKey))
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
		RequestBody:    truncateBody(body, 8192),
	})

	start := time.Now()

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 15 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
	resp, err := client.Do(req)
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
			ResponseBody:   truncateBody(bodyBytes, 8192),
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

	req, err := http.NewRequest("POST", url, io.NopCloser(newBytesReader(translatedBody)))
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	// Codex CLI headers
	req.Header.Set("originator", "codex-cli")
	req.Header.Set("User-Agent", "codex-cli/1.0.18 (macOS; arm64)")

	log.Printf("[CODEX] --> %s | conn=%s | model=%s | body_size=%d", url, credentials.ConnectionName, model, len(translatedBody))
	log.Printf("[CODEX]     Authorization: Bearer %s", maskedToken(credentials.AccessToken))

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
		RequestBody:    truncateBody(translatedBody, 8192),
	})

	start := time.Now()

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 15 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
	resp, err := client.Do(req)
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
			ResponseBody:   truncateBody(bodyBytes, 8192),
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

// bytesReader wraps a byte slice as an io.Reader.
type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func maskedToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

// truncateBody returns at most maxBytes of body as a readable string.
func truncateBody(b []byte, maxBytes int) string {
	if len(b) <= maxBytes {
		return string(b)
	}
	return string(b[:maxBytes]) + fmt.Sprintf("... [truncated %d bytes]", len(b)-maxBytes)
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func extractUsage(body []byte) *openAIUsage {
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}
		var chunk struct {
			Usage *openAIUsage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err == nil && chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
			return chunk.Usage
		}
	}
	return nil
}

func extractResponsePreview(body []byte) (string, bool) {
	var builder strings.Builder
	const maxPreviewBytes = 4000
	appendContent := func(content string) bool {
		for _, r := range content {
			runeBytes := len(string(r))
			if builder.Len()+runeBytes > maxPreviewBytes {
				return true
			}
			builder.WriteRune(r)
		}
		return false
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			content := choice.Delta.Content
			if content == "" {
				content = choice.Message.Content
			}
			if content == "" {
				continue
			}
			if appendContent(content) {
				return strings.TrimSpace(builder.String()), true
			}
		}
	}

	if preview := strings.TrimSpace(builder.String()); preview != "" {
		return preview, false
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		OutputText string `json:"output_text"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", false
	}
	if response.OutputText != "" && appendContent(response.OutputText) {
		return strings.TrimSpace(builder.String()), true
	}
	for _, choice := range response.Choices {
		content := choice.Message.Content
		if content == "" {
			content = choice.Text
		}
		if appendContent(content) {
			return strings.TrimSpace(builder.String()), true
		}
	}
	return strings.TrimSpace(builder.String()), false
}

func openAIResponseLevel(status int) string {
	if status >= 400 {
		return "ERROR"
	}
	return "INFO"
}

type openaiStreamSniffer struct {
	io.ReadCloser
	bodyBuf []byte
	onClose func([]byte)
}

func (s *openaiStreamSniffer) Read(p []byte) (n int, err error) {
	n, err = s.ReadCloser.Read(p)
	if n > 0 {
		// keep up to maxBytes to avoid memory explosion if stream is huge
		if len(s.bodyBuf) < 100*1024 {
			s.bodyBuf = append(s.bodyBuf, p[:n]...)
		}
	}
	return n, err
}

func (s *openaiStreamSniffer) Close() error {
	err := s.ReadCloser.Close()
	if s.onClose != nil && len(s.bodyBuf) > 0 {
		s.onClose(s.bodyBuf)
	}
	return err
}
