package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// Executor handles making requests to Anthropic Messages API.
// Translates OpenAI Chat Completions format ↔ Anthropic Messages format.
type Executor struct{}

// NewExecutor creates a new Anthropic executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// anthropicMessage represents a message in Anthropic format.
type anthropicMessage struct {
	Role    string                 `json:"role"`
	Content interface{}            `json:"content"` // string or array of content blocks
}

// anthropicContentBlock represents a content block in Anthropic format.
type anthropicContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Usage     map[string]interface{} `json:"usage,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   interface{}            `json:"content,omitempty"`
}

// anthropicTool represents a tool in Anthropic format.
type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// anthropicRequest represents the Anthropic Messages API request body.
type anthropicRequest struct {
	Model       string                 `json:"model"`
	Messages    []anthropicMessage     `json:"messages"`
	System      string                 `json:"system,omitempty"`
	MaxTokens   int                    `json:"max_tokens"`
	Stream      bool                   `json:"stream"`
	Temperature *float64               `json:"temperature,omitempty"`
	TopP        *float64               `json:"top_p,omitempty"`
	Tools       []anthropicTool        `json:"tools,omitempty"`
	ToolChoice  map[string]interface{} `json:"tool_choice,omitempty"`
}

// anthropicStreamEvent represents a streaming event from Anthropic API.
type anthropicStreamEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`
	Delta json.RawMessage `json:"delta,omitempty"`
	ContentBlock json.RawMessage `json:"content_block,omitempty"`
	Usage   json.RawMessage `json:"usage,omitempty"`
}

// openaiChatRequest represents an OpenAI Chat Completions request.
type openaiChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stream      bool            `json:"stream"`
	Tools       []openaiTool    `json:"tools,omitempty"`
}

type openaiMessage struct {
	Role      string          `json:"role"`
	Content   interface{}     `json:"content"`
	Name      string          `json:"name,omitempty"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type openaiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function openaiFunction  `json:"function"`
}

type openaiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiTool struct {
	Type     string              `json:"type"`
	Function openaiToolFunction  `json:"function"`
}

type openaiToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Execute sends a request to Anthropic Messages API and returns a streaming reader.
func (e *Executor) Execute(model string, body []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
	// Parse OpenAI request
	var openaiReq openaiChatRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, 400, fmt.Errorf("parse request: %w", err)
	}

	// Translate to Anthropic format
	anthropicReq, err := translateToAnthropic(openaiReq, model)
	if err != nil {
		return nil, 500, fmt.Errorf("translate request: %w", err)
	}

	// Build request body
	anthropicBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, 500, fmt.Errorf("marshal request: %w", err)
	}

	// Build URL
	baseURL := resolveBaseURL(credentials)
	cfg := domain.GetProviderConfig("anthropic")
	url := baseURL + cfg.ChatPath

	req, err := http.NewRequest("POST", url, bytes.NewReader(anthropicBody))
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("x-api-key", credentials.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	reqlog.SetBodies(string(shared.TruncateBody(shared.SanitizeBody(anthropicBody), 8192)), "")

	start := time.Now()

	// Execute request
	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(url, "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("anthropic request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, errRead := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()

		respBodyStr := "Unknown error"
		if errRead == nil {
			respBodyStr = string(bodyBytes)
		}
		
		reqlog.SetBodies("", string(shared.TruncateBody(shared.SanitizeBody(bodyBytes), 8192)))
		errUpstream := fmt.Errorf("%s", respBodyStr)
		reqlog.Upstream(url, "POST", resp.StatusCode, duration, errUpstream)
		return nil, resp.StatusCode, fmt.Errorf("anthropic returned %d: %s", resp.StatusCode, respBodyStr)
	}

	reqlog.Upstream(url, "POST", resp.StatusCode, duration, nil)

	// Create pipe to transform Anthropic SSE to OpenAI SSE
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer resp.Body.Close()

		state := NewAnthropicStreamState(model)
		scanner := bufio.NewScanner(resp.Body)
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
				if currentEvent == "message_start" || currentEvent == "content_block_delta" ||
					currentEvent == "content_block_start" || currentEvent == "content_block_stop" ||
					currentEvent == "message_delta" || currentEvent == "message_stop" {
					translated := translateAnthropicEvent(currentEvent, []byte(dataStr), state)
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

		// Send final DONE
		if !state.FinishReasonSent {
			finishReason := "stop"
			if state.ToolCallIndex > 0 {
				finishReason = "tool_calls"
			}
			chunk := formatOpenAIChunk(state, map[string]interface{}{}, &finishReason)
			pw.Write([]byte(chunk))
			pw.Write([]byte("data: [DONE]\n\n"))
		}

		// Log usage if available
		if state.Usage.PromptTokens > 0 || state.Usage.CompletionTokens > 0 {
			reqlog.SetUsage(state.Usage.PromptTokens, state.Usage.CompletionTokens, "anthropic_usage")
		}
	}()

	return pr, 200, nil
}

// resolveBaseURL returns the appropriate base URL for Anthropic.
func resolveBaseURL(credentials *domain.Credentials) string {
	if credentials.BaseURL != "" {
		return domain.StripVersionSuffix(credentials.BaseURL)
	}

	cfg := domain.GetProviderConfig("anthropic")
	return domain.StripVersionSuffix(cfg.DefaultBaseURL)
}

// translateToAnthropic converts OpenAI Chat Completions format to Anthropic Messages format.
func translateToAnthropic(openaiReq openaiChatRequest, model string) (*anthropicRequest, error) {
	// Extract system message
	var systemMsg string
	var messages []anthropicMessage

	for _, msg := range openaiReq.Messages {
		switch msg.Role {
		case "system":
			if str, ok := msg.Content.(string); ok {
				systemMsg = str
			}
		case "user":
			content := translateContent(msg.Content, "text")
			messages = append(messages, anthropicMessage{
				Role:    "user",
				Content: content,
			})
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				// Tool calls
				var contentBlocks []anthropicContentBlock
				for _, tc := range msg.ToolCalls {
					var input map[string]interface{}
					if tc.Function.Arguments != "" {
						json.Unmarshal([]byte(tc.Function.Arguments), &input)
					}
					contentBlocks = append(contentBlocks, anthropicContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: input,
					})
				}
				messages = append(messages, anthropicMessage{
					Role:    "assistant",
					Content: contentBlocks,
				})
			} else if str, ok := msg.Content.(string); ok {
				messages = append(messages, anthropicMessage{
					Role:    "assistant",
					Content: str,
				})
			}
		case "tool":
			// Tool results — Anthropic requires tool_result blocks with tool_use_id
			toolContent := translateContent(msg.Content, "text")
			resultBlock := anthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   toolContent,
			}
			// Merge consecutive tool results into the same user message
			if len(messages) > 0 && messages[len(messages)-1].Role == "user" {
				if blocks, ok := messages[len(messages)-1].Content.([]anthropicContentBlock); ok {
					messages[len(messages)-1].Content = append(blocks, resultBlock)
					continue
				}
			}
			messages = append(messages, anthropicMessage{
				Role:    "user",
				Content: []anthropicContentBlock{resultBlock},
			})
		}
	}

	// Build request
	req := &anthropicRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: 4096,
		Stream:    true,
	}

	if systemMsg != "" {
		req.System = systemMsg
	}

	if openaiReq.MaxTokens > 0 {
		req.MaxTokens = openaiReq.MaxTokens
	}

	if openaiReq.Temperature != nil {
		req.Temperature = openaiReq.Temperature
	}

	if openaiReq.TopP != nil {
		req.TopP = openaiReq.TopP
	}

	// Translate tools
	if len(openaiReq.Tools) > 0 {
		var tools []anthropicTool
		for _, t := range openaiReq.Tools {
			if t.Type == "function" {
				tools = append(tools, anthropicTool{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					InputSchema: t.Function.Parameters,
				})
			}
		}
		if len(tools) > 0 {
			req.Tools = tools
		}
	}

	return req, nil
}

// translateContent converts OpenAI content to Anthropic content format.
func translateContent(content interface{}, fallbackType string) interface{} {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var blocks []anthropicContentBlock
		for _, item := range c {
			if m, ok := item.(map[string]interface{}); ok {
				switch m["type"] {
				case "text":
					if text, ok := m["text"].(string); ok {
						blocks = append(blocks, anthropicContentBlock{
							Type: "text",
							Text: text,
						})
					}
				case "image_url":
					if img, ok := m["image_url"].(map[string]interface{}); ok {
						if url, ok := img["url"].(string); ok {
							blocks = append(blocks, anthropicContentBlock{
								Type: "image",
								Text: url,
							})
						}
					}
				}
			}
		}
		if len(blocks) > 0 {
			return blocks
		}
	}
	return content
}

// NewAnthropicStreamState creates a new stream state.
func NewAnthropicStreamState(model string) *AnthropicStreamState {
	return &AnthropicStreamState{
		Model: model,
	}
}

// AnthropicStreamState tracks state during streaming.
type AnthropicStreamState struct {
	Model            string
	FinishReasonSent bool
	ToolCallIndex    int
	Usage            struct {
		PromptTokens     int
		CompletionTokens int
	}
}

// translateAnthropicEvent converts an Anthropic SSE event to OpenAI format.
func translateAnthropicEvent(eventType string, data []byte, state *AnthropicStreamState) string {
	var streamEvent anthropicStreamEvent
	if err := json.Unmarshal(data, &streamEvent); err != nil {
		return ""
	}

	switch eventType {
	case "message_start":
		// Message started
		return ""

	case "content_block_start":
		// Content block started
		var block anthropicContentBlock
		if err := json.Unmarshal(streamEvent.ContentBlock, &block); err == nil {
			if block.Type == "tool_use" {
				state.ToolCallIndex++
				// Send tool call start chunk
				choice := map[string]interface{}{
					"index": state.ToolCallIndex - 1,
					"id":    block.ID,
					"type":  "function",
					"function": map[string]interface{}{
						"name":      block.Name,
						"arguments": "",
					},
					"delta": map[string]interface{}{
						"role":    "assistant",
						"content": nil,
					},
				}
				return formatSSEChunk(state, choice, nil)
			}
		}
		return ""

	case "content_block_delta":
		// Content block delta (text or tool input)
		var delta map[string]interface{}
		if err := json.Unmarshal(streamEvent.Delta, &delta); err == nil {
			if text, ok := delta["text"].(string); ok && text != "" {
				// Text delta
				choice := map[string]interface{}{
					"index": 0,
					"delta": map[string]interface{}{
						"role":    "assistant",
						"content": text,
					},
				}
				return formatSSEChunk(state, choice, nil)
			}
			if partialJSON, ok := delta["partial_json"].(string); ok {
				// Tool use delta
				if state.ToolCallIndex > 0 {
					choice := map[string]interface{}{
						"index": state.ToolCallIndex - 1,
						"delta": map[string]interface{}{
							"function": map[string]interface{}{
								"arguments": partialJSON,
							},
						},
					}
					return formatSSEChunk(state, choice, nil)
				}
			}
		}
		return ""

	case "content_block_stop":
		// Content block stopped
		return ""

	case "message_delta":
		// Message delta (usage, stop reason)
		var delta struct {
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(data, &delta); err == nil {
			state.Usage.CompletionTokens = delta.Usage.OutputTokens
			if delta.Delta.StopReason != "" {
				finishReason := mapStopReason(delta.Delta.StopReason)
				choice := map[string]interface{}{
					"index":        0,
					"finish_reason": finishReason,
					"delta":         map[string]interface{}{},
				}
				state.FinishReasonSent = true
				return formatSSEChunk(state, choice, &finishReason)
			}
		}
		return ""

	case "message_stop":
		// Message completed
		return ""
	}

	return ""
}

// mapStopReason maps Anthropic stop reasons to OpenAI.
func mapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

// formatSSEChunk formats a chunk as Server-Sent Event.
func formatSSEChunk(state *AnthropicStreamState, choice map[string]interface{}, finishReason *string) string {
	choice["finish_reason"] = finishReason

	payload := map[string]interface{}{
		"id":                fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli()),
		"object":            "chat.completion.chunk",
		"created":           time.Now().Unix(),
		"model":             state.Model,
		"system_fingerprint": nil,
		"choices":           []interface{}{choice},
	}

	data, _ := json.Marshal(payload)
	return fmt.Sprintf("data: %s\n\n", string(data))
}

// formatOpenAIChunk formats an OpenAI chat completion chunk.
func formatOpenAIChunk(state *AnthropicStreamState, choice map[string]interface{}, finishReason *string) string {
	return formatSSEChunk(state, choice, finishReason)
}
