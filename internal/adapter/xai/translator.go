package xai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type chatRequest struct {
	Model               string        `json:"model"`
	Messages            []chatMessage `json:"messages"`
	Stream              bool          `json:"stream"`
	Temperature         *float64      `json:"temperature,omitempty"`
	TopP                *float64      `json:"top_p,omitempty"`
	MaxTokens           int           `json:"max_tokens,omitempty"`
	MaxCompletionTokens int           `json:"max_completion_tokens,omitempty"`
	Tools               []chatTool    `json:"tools,omitempty"`
}

type chatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type responsesRequest struct {
	Model           string                   `json:"model"`
	Input           []responsesInput         `json:"input"`
	Instructions    string                   `json:"instructions,omitempty"`
	Stream          bool                     `json:"stream"`
	Temperature     *float64                 `json:"temperature,omitempty"`
	TopP            *float64                 `json:"top_p,omitempty"`
	MaxOutputTokens int                      `json:"max_output_tokens,omitempty"`
	Tools           []map[string]interface{} `json:"tools,omitempty"`
}

type responsesInput struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type ResponseState struct {
	Model            string
	ID               string
	Created          int64
	FinishReasonSent bool
	Usage            Usage
}

func NewResponseState(model string) *ResponseState {
	now := time.Now()
	return &ResponseState{
		Model:   model,
		ID:      fmt.Sprintf("chatcmpl-xai-%d", now.UnixNano()),
		Created: now.Unix(),
	}
}

func TranslateChatToResponses(model string, body []byte) ([]byte, error) {
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse chat request: %w", err)
	}

	out := responsesRequest{
		Model:       model,
		Stream:      true,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}
	if req.MaxCompletionTokens > 0 {
		out.MaxOutputTokens = req.MaxCompletionTokens
	} else if req.MaxTokens > 0 {
		out.MaxOutputTokens = req.MaxTokens
	}

	for _, msg := range req.Messages {
		role := strings.TrimSpace(msg.Role)
		switch role {
		case "system", "developer":
			text := contentToText(msg.Content)
			if text != "" {
				if out.Instructions != "" {
					out.Instructions += "\n"
				}
				out.Instructions += text
			}
		case "user", "assistant":
			out.Input = append(out.Input, responsesInput{Role: role, Content: msg.Content})
		case "tool":
			out.Input = append(out.Input, responsesInput{Role: "user", Content: msg.Content})
		default:
			return nil, fmt.Errorf("unsupported message role %q", role)
		}
	}

	for _, tool := range req.Tools {
		if tool.Type != "function" {
			return nil, fmt.Errorf("unsupported tool type %q", tool.Type)
		}
		params := tool.Function.Parameters
		if params == nil {
			params = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		out.Tools = append(out.Tools, map[string]interface{}{
			"type":        "function",
			"name":        tool.Function.Name,
			"description": tool.Function.Description,
			"parameters":  params,
		})
	}

	return json.Marshal(out)
}

func contentToText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok || m["type"] != "text" {
				continue
			}
			if text, ok := m["text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func TranslateResponsesEvent(data []byte, state *ResponseState) string {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return ""
	}
	eventType, _ := event["type"].(string)
	switch eventType {
	case "response.output_text.delta":
		delta, _ := event["delta"].(string)
		return formatChunk(state, map[string]interface{}{"content": delta}, nil)
	case "response.completed", "response.incomplete":
		extractUsage(event, state)
		finish := "stop"
		if eventType == "response.incomplete" {
			finish = "length"
		}
		state.FinishReasonSent = true
		return formatChunk(state, map[string]interface{}{}, &finish) + "data: [DONE]\n\n"
	default:
		return ""
	}
}

func extractUsage(event map[string]interface{}, state *ResponseState) {
	response, _ := event["response"].(map[string]interface{})
	usage, _ := response["usage"].(map[string]interface{})
	state.Usage.PromptTokens = intFromAny(usage["input_tokens"])
	state.Usage.CompletionTokens = intFromAny(usage["output_tokens"])
	state.Usage.TotalTokens = intFromAny(usage["total_tokens"])
}

func intFromAny(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func formatChunk(state *ResponseState, delta map[string]interface{}, finish *string) string {
	choice := map[string]interface{}{
		"index": 0,
		"delta": delta,
	}
	if finish != nil {
		choice["finish_reason"] = *finish
	}
	chunk := map[string]interface{}{
		"id":      state.ID,
		"object":  "chat.completion.chunk",
		"created": state.Created,
		"model":   state.Model,
		"choices": []interface{}{choice},
	}
	data, _ := json.Marshal(chunk)
	return "data: " + string(data) + "\n\n"
}
