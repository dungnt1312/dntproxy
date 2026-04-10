package openai

import (
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

// Execute sends a request to OpenAI (or compatible) API and returns a streaming reader.
func (e *Executor) Execute(model string, body []byte, credentials *domain.Credentials, requestID string) (io.ReadCloser, int, error) {
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

	log.Printf("[OPENAI] --> %s | conn=%s | model=%s", url, credentials.ConnectionName, model)
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
	})

	start := time.Now()

	client := &http.Client{}
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

	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	log.Printf("[OPENAI] <-- %s | conn=%s | model=%s | status=%d | duration=%s | body_size=%d",
		url, credentials.ConnectionName, model, resp.StatusCode, duration, len(bodyBytes))
	responseEntry := domain.LogEntry{
		Level:          openAIResponseLevel(resp.StatusCode),
		Provider:       "OPENAI",
		Direction:      "response",
		Path:           url,
		StatusCode:     resp.StatusCode,
		DurationMs:     duration.Milliseconds(),
		ConnectionID:   credentials.ConnectionID,
		ConnectionName: credentials.ConnectionName,
		Model:          model,
		RequestID:      requestID,
		Message:        "OpenAI-compatible response received",
		BodySize:       len(bodyBytes),
	}

	if resp.StatusCode != http.StatusOK {
		responseEntry.Error = string(bodyBytes)
		appLogger.AddEntry(responseEntry)
		return nil, resp.StatusCode, fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(bodyBytes))
	}
	appLogger.AddEntry(responseEntry)

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

	// Restore body for streaming
	resp.Body = io.NopCloser(newBytesReader(bodyBytes))

	// OpenAI-compatible responses are already SSE formatted, pass through directly
	return resp.Body, 200, nil
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
