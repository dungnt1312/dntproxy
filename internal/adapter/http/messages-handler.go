package http

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
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// --- Anthropic Messages API types ---

type anthropicMessagesRequest struct {
	Model       string                 `json:"model"`
	Messages    []anthropicMsgItem     `json:"messages"`
	System      interface{}            `json:"system,omitempty"` // string or []content_block
	MaxTokens   int                    `json:"max_tokens"`
	Stream      *bool                  `json:"stream,omitempty"`
	Temperature *float64               `json:"temperature,omitempty"`
	TopP        *float64               `json:"top_p,omitempty"`
	TopK        *int                   `json:"top_k,omitempty"`
	StopSeqs    []string               `json:"stop_sequences,omitempty"`
	Tools       []json.RawMessage      `json:"tools,omitempty"`
	ToolChoice  interface{}            `json:"tool_choice,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type anthropicMsgItem struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []content_block
}

// --- OpenAI types for translation ---

type openaiChatMsg struct {
	Role       string             `json:"role"`
	Content    interface{}        `json:"content,omitempty"`
	ToolCalls  []openaiToolCallT  `json:"tool_calls,omitempty"`
	ToolCallID string             `json:"tool_call_id,omitempty"`
}

type openaiToolCallT struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openaiToolFnT  `json:"function"`
}

type openaiToolFnT struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiToolDef struct {
	Type     string        `json:"type"`
	Function openaiToolFn2 `json:"function"`
}

type openaiToolFn2 struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// messagesHandler handles POST /v1/messages (Anthropic Messages API compatible).
// Translates Anthropic format → OpenAI format → chatService → OpenAI SSE → Anthropic SSE.
func messagesHandler(chatService port.ChatService, store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.New().String()

		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxChatBodySize))
		if err != nil {
			writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read body")
			return
		}

		// Parse Anthropic Messages request
		var antReq anthropicMessagesRequest
		if err := json.Unmarshal(body, &antReq); err != nil {
			writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "Invalid JSON: "+err.Error())
			return
		}

		if antReq.Model == "" {
			writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "Missing model")
			return
		}

		if antReq.MaxTokens <= 0 {
			writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "max_tokens is required and must be > 0")
			return
		}

		isStream := antReq.Stream != nil && *antReq.Stream

		log.Printf("[MESSAGES] POST /v1/messages | model=%s | stream=%v", antReq.Model, isStream)

		// Translate Anthropic → OpenAI
		openaiBody, err := translateAnthropicToOpenAI(&antReq)
		if err != nil {
			writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "Translation error: "+err.Error())
			return
		}

		// Use chatService (always gets OpenAI SSE stream back)
		result := chatService.HandleChat(openaiBody, antReq.Model, requestID)

		logger.Get().AddEntry(domain.LogEntry{
			Level:      statusLevel(result.StatusCode),
			Provider:   "CLIENT",
			Direction:  "inbound",
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			StatusCode: result.StatusCode,
			DurationMs: time.Since(start).Milliseconds(),
			Model:      antReq.Model,
			RequestID:  requestID,
			Message:    "Client Anthropic messages request",
			Error:      result.Error,
			BodySize:   len(body),
		})

		if result.Stream == nil {
			writeAnthropicError(c, result.StatusCode, "api_error", result.Error)
			return
		}

		if !isStream {
			// Non-streaming: collect the full OpenAI SSE stream, then return a single Anthropic response
			handleNonStreamingMessages(c, result.Stream, antReq.Model, requestID)
			return
		}

		// Streaming: convert OpenAI SSE → Anthropic SSE
		handleStreamingMessages(c, result.Stream, antReq.Model, requestID)
	}
}

// handleStreamingMessages reads OpenAI SSE stream and writes Anthropic SSE events.
func handleStreamingMessages(c *gin.Context, stream io.ReadCloser, model string, requestID string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	defer stream.Close()

	msgID := fmt.Sprintf("msg_%s", requestID[:8])

	// Send message_start
	writeAnthropicSSE(c.Writer, "message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":      msgID,
			"type":    "message",
			"role":    "assistant",
			"model":   model,
			"content": []interface{}{},
			"usage": map[string]interface{}{
				"input_tokens":  0,
				"output_tokens": 0,
			},
			"stop_reason":   nil,
			"stop_sequence": nil,
		},
	})
	c.Writer.Flush()

	// State tracking
	contentBlockStarted := false
	blockIndex := 0
	var toolCallBlocks []toolCallBlock
	outputTokens := 0

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openaiSSEChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			// Check for usage in non-choice chunks
			if chunk.Usage != nil {
				outputTokens = chunk.Usage.CompletionTokens
			}
			continue
		}

		choice := chunk.Choices[0]

		// Handle tool calls
		if len(choice.Delta.ToolCalls) > 0 {
			for _, tc := range choice.Delta.ToolCalls {
				if tc.Function.Name != "" {
					// New tool call — close previous text block if open
					if contentBlockStarted {
						writeAnthropicSSE(c.Writer, "content_block_stop", map[string]interface{}{
							"type":  "content_block_stop",
							"index": blockIndex,
						})
						blockIndex++
						contentBlockStarted = false
					}

					// Start tool_use block
					writeAnthropicSSE(c.Writer, "content_block_start", map[string]interface{}{
						"type":  "content_block_start",
						"index": blockIndex,
						"content_block": map[string]interface{}{
							"type":  "tool_use",
							"id":    tc.ID,
							"name":  tc.Function.Name,
							"input": map[string]interface{}{},
						},
					})
					c.Writer.Flush()

					toolCallBlocks = append(toolCallBlocks, toolCallBlock{
						index: blockIndex,
						id:    tc.ID,
						name:  tc.Function.Name,
					})
				}

				if tc.Function.Arguments != "" {
					// Accumulate tool input JSON
					writeAnthropicSSE(c.Writer, "content_block_delta", map[string]interface{}{
						"type":  "content_block_delta",
						"index": blockIndex,
						"delta": map[string]interface{}{
							"type":         "input_json_delta",
							"partial_json": tc.Function.Arguments,
						},
					})
					c.Writer.Flush()
				}
			}
			continue
		}

		// Handle text content
		if choice.Delta.Content != "" {
			if !contentBlockStarted {
				// Start text content block
				writeAnthropicSSE(c.Writer, "content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": blockIndex,
					"content_block": map[string]interface{}{
						"type": "text",
						"text": "",
					},
				})
				c.Writer.Flush()
				contentBlockStarted = true
			}

			writeAnthropicSSE(c.Writer, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": blockIndex,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": choice.Delta.Content,
				},
			})
			c.Writer.Flush()
		}

		// Handle finish reason
		if choice.FinishReason != nil {
			// Close any open block
			if contentBlockStarted || len(toolCallBlocks) > 0 {
				writeAnthropicSSE(c.Writer, "content_block_stop", map[string]interface{}{
					"type":  "content_block_stop",
					"index": blockIndex,
				})
			}

			stopReason := mapOpenAIFinishToAnthropic(*choice.FinishReason)

			writeAnthropicSSE(c.Writer, "message_delta", map[string]interface{}{
				"type": "message_delta",
				"delta": map[string]interface{}{
					"stop_reason":   stopReason,
					"stop_sequence": nil,
				},
				"usage": map[string]interface{}{
					"output_tokens": outputTokens,
				},
			})
			c.Writer.Flush()
		}

		// Check for usage info
		if chunk.Usage != nil {
			outputTokens = chunk.Usage.CompletionTokens
		}

		select {
		case <-c.Request.Context().Done():
			return
		default:
		}
	}

	// Send message_stop
	writeAnthropicSSE(c.Writer, "message_stop", map[string]interface{}{
		"type": "message_stop",
	})
	c.Writer.Flush()
}

// handleNonStreamingMessages collects the full OpenAI SSE stream and returns a single Anthropic response.
func handleNonStreamingMessages(c *gin.Context, stream io.ReadCloser, model string, requestID string) {
	defer stream.Close()

	msgID := fmt.Sprintf("msg_%s", requestID[:8])
	var fullText strings.Builder
	var toolCalls []collectedToolCall
	finishReason := "end_turn"
	inputTokens := 0
	outputTokens := 0

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openaiSSEChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			inputTokens = chunk.Usage.PromptTokens
			outputTokens = chunk.Usage.CompletionTokens
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		if choice.Delta.Content != "" {
			fullText.WriteString(choice.Delta.Content)
		}

		for _, tc := range choice.Delta.ToolCalls {
			if tc.Function.Name != "" {
				toolCalls = append(toolCalls, collectedToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
					Args: tc.Function.Arguments,
				})
			} else if len(toolCalls) > 0 {
				toolCalls[len(toolCalls)-1].Args += tc.Function.Arguments
			}
		}

		if choice.FinishReason != nil {
			finishReason = mapOpenAIFinishToAnthropic(*choice.FinishReason)
		}
	}

	// Build Anthropic response
	var content []interface{}
	if fullText.Len() > 0 {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": fullText.String(),
		})
	}
	for _, tc := range toolCalls {
		var input interface{}
		if tc.Args != "" {
			json.Unmarshal([]byte(tc.Args), &input)
		}
		if input == nil {
			input = map[string]interface{}{}
		}
		content = append(content, map[string]interface{}{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Name,
			"input": input,
		})
	}

	if len(content) == 0 {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": "",
		})
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"id":            msgID,
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       content,
		"stop_reason":   finishReason,
		"stop_sequence": nil,
		"usage": map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	})
}

// --- Translation helpers ---

// translateAnthropicToOpenAI converts an Anthropic Messages request to OpenAI Chat Completions format.
func translateAnthropicToOpenAI(req *anthropicMessagesRequest) ([]byte, error) {
	openaiReq := map[string]interface{}{
		"model":  req.Model,
		"stream": true, // Always stream internally
	}

	// System message
	var messages []map[string]interface{}
	if req.System != nil {
		switch s := req.System.(type) {
		case string:
			if s != "" {
				messages = append(messages, map[string]interface{}{
					"role":    "system",
					"content": s,
				})
			}
		case []interface{}:
			// Array of content blocks — extract text
			var parts []string
			for _, block := range s {
				if m, ok := block.(map[string]interface{}); ok {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
			if len(parts) > 0 {
				messages = append(messages, map[string]interface{}{
					"role":    "system",
					"content": strings.Join(parts, "\n"),
				})
			}
		}
	}

	// Messages
	for _, msg := range req.Messages {
		converted, err := convertAnthropicMessage(msg)
		if err != nil {
			return nil, err
		}
		messages = append(messages, converted...)
	}

	openaiReq["messages"] = messages

	if req.MaxTokens > 0 {
		openaiReq["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		openaiReq["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		openaiReq["top_p"] = *req.TopP
	}
	if len(req.StopSeqs) > 0 {
		openaiReq["stop"] = req.StopSeqs
	}

	// Tools
	if len(req.Tools) > 0 {
		var tools []map[string]interface{}
		for _, rawTool := range req.Tools {
			var tool map[string]interface{}
			if err := json.Unmarshal(rawTool, &tool); err != nil {
				continue
			}
			name, _ := tool["name"].(string)
			desc, _ := tool["description"].(string)
			inputSchema, _ := tool["input_schema"]

			openaiTool := map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        name,
					"description": desc,
				},
			}
			if inputSchema != nil {
				openaiTool["function"].(map[string]interface{})["parameters"] = inputSchema
			}
			tools = append(tools, openaiTool)
		}
		if len(tools) > 0 {
			openaiReq["tools"] = tools
		}
	}

	return json.Marshal(openaiReq)
}

// convertAnthropicMessage converts a single Anthropic message to OpenAI message(s).
func convertAnthropicMessage(msg anthropicMsgItem) ([]map[string]interface{}, error) {
	switch msg.Role {
	case "user":
		return convertUserMessage(msg)
	case "assistant":
		return convertAssistantMessage(msg)
	default:
		// Pass through unknown roles
		content := ""
		if s, ok := msg.Content.(string); ok {
			content = s
		}
		return []map[string]interface{}{{
			"role":    msg.Role,
			"content": content,
		}}, nil
	}
}

func convertUserMessage(msg anthropicMsgItem) ([]map[string]interface{}, error) {
	switch c := msg.Content.(type) {
	case string:
		return []map[string]interface{}{{
			"role":    "user",
			"content": c,
		}}, nil
	case []interface{}:
		// Check if this contains tool_result blocks (need to split into tool messages)
		var toolResults []map[string]interface{}
		var contentParts []map[string]interface{}

		for _, item := range c {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)

			switch blockType {
			case "tool_result":
				toolCallID, _ := block["tool_use_id"].(string)
				content := ""
				if s, ok := block["content"].(string); ok {
					content = s
				} else if arr, ok := block["content"].([]interface{}); ok {
					// Extract text from content blocks
					var parts []string
					for _, cb := range arr {
						if m, ok := cb.(map[string]interface{}); ok {
							if text, ok := m["text"].(string); ok {
								parts = append(parts, text)
							}
						}
					}
					content = strings.Join(parts, "\n")
				}
				toolResults = append(toolResults, map[string]interface{}{
					"role":         "tool",
					"tool_call_id": toolCallID,
					"content":      content,
				})
			case "text":
				text, _ := block["text"].(string)
				contentParts = append(contentParts, map[string]interface{}{
					"type": "text",
					"text": text,
				})
			case "image":
				// Pass through image content
				contentParts = append(contentParts, block)
			}
		}

		var result []map[string]interface{}
		// Tool results become separate tool messages in OpenAI format
		result = append(result, toolResults...)

		if len(contentParts) > 0 {
			if len(contentParts) == 1 && contentParts[0]["type"] == "text" {
				result = append(result, map[string]interface{}{
					"role":    "user",
					"content": contentParts[0]["text"],
				})
			} else {
				result = append(result, map[string]interface{}{
					"role":    "user",
					"content": contentParts,
				})
			}
		}

		if len(result) == 0 {
			result = append(result, map[string]interface{}{
				"role":    "user",
				"content": "",
			})
		}

		return result, nil
	default:
		return []map[string]interface{}{{
			"role":    "user",
			"content": fmt.Sprintf("%v", msg.Content),
		}}, nil
	}
}

func convertAssistantMessage(msg anthropicMsgItem) ([]map[string]interface{}, error) {
	switch c := msg.Content.(type) {
	case string:
		return []map[string]interface{}{{
			"role":    "assistant",
			"content": c,
		}}, nil
	case []interface{}:
		var textContent string
		var toolCalls []map[string]interface{}

		for _, item := range c {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)

			switch blockType {
			case "text":
				text, _ := block["text"].(string)
				textContent += text
			case "tool_use":
				id, _ := block["id"].(string)
				name, _ := block["name"].(string)
				input := block["input"]
				args := "{}"
				if input != nil {
					if argsBytes, err := json.Marshal(input); err == nil {
						args = string(argsBytes)
					}
				}
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   id,
					"type": "function",
					"function": map[string]interface{}{
						"name":      name,
						"arguments": args,
					},
				})
			}
		}

		result := map[string]interface{}{
			"role": "assistant",
		}
		if textContent != "" {
			result["content"] = textContent
		}
		if len(toolCalls) > 0 {
			result["tool_calls"] = toolCalls
		}
		return []map[string]interface{}{result}, nil
	default:
		return []map[string]interface{}{{
			"role":    "assistant",
			"content": fmt.Sprintf("%v", msg.Content),
		}}, nil
	}
}

// --- SSE/response helpers ---

type openaiSSEChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Model   string            `json:"model"`
	Choices []openaiSSEChoice `json:"choices"`
	Usage   *openaiSSEUsage   `json:"usage,omitempty"`
}

type openaiSSEChoice struct {
	Index        int              `json:"index"`
	Delta        openaiSSEDelta   `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

type openaiSSEDelta struct {
	Role      string              `json:"role,omitempty"`
	Content   string              `json:"content,omitempty"`
	ToolCalls []openaiSSEToolCall `json:"tool_calls,omitempty"`
}

type openaiSSEToolCall struct {
	Index    int               `json:"index"`
	ID       string            `json:"id,omitempty"`
	Type     string            `json:"type,omitempty"`
	Function openaiSSEToolFn   `json:"function"`
}

type openaiSSEToolFn struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openaiSSEUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type toolCallBlock struct {
	index int
	id    string
	name  string
}

type collectedToolCall struct {
	ID   string
	Name string
	Args string
}

func writeAnthropicSSE(w gin.ResponseWriter, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(jsonData))
}

func writeAnthropicError(c *gin.Context, status int, errType string, message string) {
	c.JSON(status, map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    errType,
			"message": message,
		},
	})
}

func mapOpenAIFinishToAnthropic(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}
