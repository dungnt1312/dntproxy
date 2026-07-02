package openai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Codex Responses API translators
// OpenAI OAuth tokens (from auth.openai.com) use the ChatGPT backend:
//   POST https://chatgpt.com/backend-api/codex/responses
// This is the "Responses API" format, NOT the standard /v1/chat/completions format.
//
// Request:  { input: [...], instructions: "...", model, stream, store }
// Response: SSE with event types like response.output_text.delta, response.completed, etc.

const codexResponsesURL = "https://chatgpt.com/backend-api/codex/responses"

// --- Request Translation: Chat Completions → Codex Responses API ---

// responsesRequest is the Codex Responses API request format.
type responsesRequest struct {
	Model        string          `json:"model"`
	Input        []interface{}   `json:"input"`
	Instructions string          `json:"instructions"`
	Stream       bool            `json:"stream"`
	Store        bool            `json:"store"`
	Tools        []interface{}   `json:"tools,omitempty"`
	Reasoning    *codexReasoning `json:"reasoning,omitempty"`
	Include      []string        `json:"include,omitempty"`
}

// codexReasoning configures the thinking/reasoning effort for Codex models.
type codexReasoning struct {
	Effort  string `json:"effort"`
	Summary string `json:"summary"`
}

// responsesInputMessage is a message item in the Responses API input.
type responsesInputMessage struct {
	Type    string        `json:"type"`
	Role    string        `json:"role"`
	Content []interface{} `json:"content"`
}

// responsesFunctionCall is a function_call item in Responses API input.
type responsesFunctionCall struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// responsesFunctionCallOutput is a function_call_output item in Responses API input.
type responsesFunctionCallOutput struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

// responsesTool is the Responses API tool format.
type responsesTool struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
	Strict      *bool       `json:"strict,omitempty"`
}

// codexEffortLevels are valid thinking effort suffixes for Codex model names.
var codexEffortLevels = []string{"none", "low", "medium", "high", "xhigh"}

// TranslateChatToCodexResponses converts a Chat Completions request body
// to a Codex Responses API request body.
func TranslateChatToCodexResponses(chatBody []byte) ([]byte, error) {
	var chat map[string]interface{}
	if err := json.Unmarshal(chatBody, &chat); err != nil {
		return nil, fmt.Errorf("parse chat body: %w", err)
	}

	req := responsesRequest{
		Stream: true,
		Store:  false,
	}

	model, _ := chat["model"].(string)
	req.Model = model

	// Extract effort level from model name suffix (e.g. gpt-5.3-codex-high → high)
	modelEffort := ""
	for _, level := range codexEffortLevels {
		suffix := "-" + level
		if strings.HasSuffix(model, suffix) {
			modelEffort = level
			req.Model = strings.TrimSuffix(model, suffix)
			break
		}
	}

	// Extract messages
	messages, _ := chat["messages"].([]interface{})
	req.Input = make([]interface{}, 0)

	for _, msgRaw := range messages {
		msg, ok := msgRaw.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)

		switch role {
		case "system":
			// First system message → instructions
			if req.Instructions == "" {
				if content, ok := msg["content"].(string); ok {
					req.Instructions = content
				}
			}

		case "user", "assistant":
			contentType := "input_text"
			if role == "assistant" {
				contentType = "output_text"
			}

			var contentParts []interface{}

			switch content := msg["content"].(type) {
			case string:
				if content != "" {
					contentParts = append(contentParts, map[string]interface{}{
						"type": contentType,
						"text": content,
					})
				}
			case []interface{}:
				for _, part := range content {
					partMap, ok := part.(map[string]interface{})
					if !ok {
						continue
					}
					partType, _ := partMap["type"].(string)
					switch partType {
					case "text":
						contentParts = append(contentParts, map[string]interface{}{
							"type": contentType,
							"text": partMap["text"],
						})
					case "image_url":
						// Convert Chat Completions image_url → Responses API input_image
						imgURL := ""
						if urlObj, ok := partMap["image_url"].(map[string]interface{}); ok {
							imgURL, _ = urlObj["url"].(string)
						} else if urlStr, ok := partMap["image_url"].(string); ok {
							imgURL = urlStr
						}
						contentParts = append(contentParts, map[string]interface{}{
							"type":      "input_image",
							"image_url": imgURL,
							"detail":    "auto",
						})
					default:
						// Pass through or convert to text
						text := ""
						if t, ok := partMap["text"].(string); ok {
							text = t
						}
						if text != "" {
							contentParts = append(contentParts, map[string]interface{}{
								"type": contentType,
								"text": text,
							})
						}
					}
				}
			}

			// Only add message if content is non-empty
			if len(contentParts) > 0 {
				req.Input = append(req.Input, map[string]interface{}{
					"type":    "message",
					"role":    role,
					"content": contentParts,
				})
			}

			// Convert tool_calls from assistant message
			if role == "assistant" {
				if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
					for _, tcRaw := range toolCalls {
						tc, ok := tcRaw.(map[string]interface{})
						if !ok {
							continue
						}
						callID, _ := tc["id"].(string)
						fn, _ := tc["function"].(map[string]interface{})
						name, _ := fn["name"].(string)
						args, _ := fn["arguments"].(string)

						// Clamp call_id to 64 chars (Responses API limit)
						if len(callID) > 64 {
							callID = callID[:64]
						}

						req.Input = append(req.Input, map[string]interface{}{
							"type":      "function_call",
							"call_id":   callID,
							"name":      name,
							"arguments": args,
						})
					}
				}
			}

		case "tool":
			// Convert tool result
			callID, _ := msg["tool_call_id"].(string)
			if len(callID) > 64 {
				callID = callID[:64]
			}
			output := ""
			switch content := msg["content"].(type) {
			case string:
				output = content
			default:
				b, _ := json.Marshal(content)
				output = string(b)
			}

			req.Input = append(req.Input, map[string]interface{}{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  output,
			})
		}
	}

	// Inject default Codex instructions when none provided
	if req.Instructions == "" {
		req.Instructions = codexDefaultInstructions
	}

	// Guard empty input — Codex API rejects empty input array
	if len(req.Input) == 0 {
		req.Input = []interface{}{
			map[string]interface{}{
				"type": "message",
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "input_text", "text": "..."},
				},
			},
		}
	}

	// Convert tools format (only include tools with valid names)
	if tools, ok := chat["tools"].([]interface{}); ok {
		var convertedTools []interface{}
		for _, toolRaw := range tools {
			tool, ok := toolRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if fn, ok := tool["function"].(map[string]interface{}); ok {
				name, _ := fn["name"].(string)
				if name == "" {
					continue // Skip hosted tools without names
				}
				desc, _ := fn["description"].(string)
				params := fn["parameters"]

				// Ensure object schema has properties
				if params == nil {
					params = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
				} else if paramsMap, ok := params.(map[string]interface{}); ok {
					if paramsMap["type"] == "object" && paramsMap["properties"] == nil {
						paramsMap["properties"] = map[string]interface{}{}
					}
				}

				ct := map[string]interface{}{
					"type":        "function",
					"name":        name,
					"description": desc,
					"parameters":  params,
				}
				if strict, ok := fn["strict"]; ok {
					ct["strict"] = strict
				}
				convertedTools = append(convertedTools, ct)
			}
		}
		if len(convertedTools) > 0 {
			req.Tools = convertedTools
		}
	}

	// Set reasoning effort: explicit reasoning > reasoning_effort param > model suffix > default (medium)
	effort := "medium"
	if re, ok := chat["reasoning_effort"].(string); ok && re != "" {
		effort = re
	}
	if modelEffort != "" {
		effort = modelEffort
	}
	if re, ok := chat["reasoning"].(map[string]interface{}); ok {
		if e, ok := re["effort"].(string); ok && e != "" {
			effort = e
		}
	}
	req.Reasoning = &codexReasoning{Effort: effort, Summary: "auto"}

	// Include reasoning encrypted content (required for reasoning models)
	if effort != "none" {
		req.Include = []string{"reasoning.encrypted_content"}
	}

	// NOTE: temperature, top_p, max_tokens, frequency_penalty, etc. are NOT supported
	// by the Codex Responses API — do NOT pass them through.

	return json.Marshal(req)
}

// --- Response Translation: Codex Responses API SSE → Chat Completions SSE ---

// CodexResponseState tracks state during response translation.
type CodexResponseState struct {
	Started          bool
	ChatID           string
	Created          int
	Model            string
	ToolCallIndex    int
	CurrentCallID    string
	FinishReasonSent bool
	Usage            *codexUsage
	PromptTokens     int
	CompletionTokens int
}

type codexUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
}

// NewCodexResponseState creates a new state for response translation.
func NewCodexResponseState(model string) *CodexResponseState {
	return &CodexResponseState{
		Model: model,
	}
}

// TranslateCodexEvent translates a single Codex Responses API SSE event
// to an OpenAI Chat Completions SSE line.
// Returns the SSE line(s) to send, or empty string if event should be skipped.
func TranslateCodexEvent(eventType string, data []byte, state *CodexResponseState) string {
	if !state.Started {
		state.Started = true
		state.ChatID = fmt.Sprintf("chatcmpl-%d", currentTimeUnix())
		state.Created = int(currentTimeUnix())
	}

	switch eventType {
	case "response.output_text.delta":
		var evt struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal(data, &evt) != nil || evt.Delta == "" {
			return ""
		}
		return formatSSEChunk(state, map[string]interface{}{
			"content": evt.Delta,
		}, nil)

	case "response.output_item.added":
		var evt struct {
			Item struct {
				Type   string `json:"type"`
				CallID string `json:"call_id"`
				Name   string `json:"name"`
			} `json:"item"`
		}
		if json.Unmarshal(data, &evt) != nil {
			return ""
		}
		if evt.Item.Type != "function_call" && evt.Item.Type != "custom_tool_call" {
			return ""
		}

		state.CurrentCallID = evt.Item.CallID
		if state.CurrentCallID == "" {
			state.CurrentCallID = fmt.Sprintf("call_%d", currentTimeUnix())
		}

		return formatSSEChunk(state, map[string]interface{}{
			"tool_calls": []map[string]interface{}{
				{
					"index": state.ToolCallIndex,
					"id":    state.CurrentCallID,
					"type":  "function",
					"function": map[string]interface{}{
						"name":      evt.Item.Name,
						"arguments": "",
					},
				},
			},
		}, nil)

	case "response.function_call_arguments.delta", "response.custom_tool_call_input.delta":
		var evt struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal(data, &evt) != nil || evt.Delta == "" {
			return ""
		}
		return formatSSEChunk(state, map[string]interface{}{
			"tool_calls": []map[string]interface{}{
				{
					"index":    state.ToolCallIndex,
					"function": map[string]interface{}{"arguments": evt.Delta},
				},
			},
		}, nil)

	case "response.output_item.done":
		var evt struct {
			Item struct {
				Type string `json:"type"`
			} `json:"item"`
		}
		if json.Unmarshal(data, &evt) != nil {
			return ""
		}
		if evt.Item.Type == "function_call" || evt.Item.Type == "custom_tool_call" {
			state.ToolCallIndex++
		}
		return ""

	case "response.completed":
		if state.FinishReasonSent {
			return ""
		}
		state.FinishReasonSent = true

		// Extract usage
		var evt struct {
			Response struct {
				Usage *struct {
					InputTokens         int `json:"input_tokens"`
					OutputTokens        int `json:"output_tokens"`
					PromptTokens        int `json:"prompt_tokens"`
					CompletionTokens    int `json:"completion_tokens"`
					CacheReadTokens     int `json:"cache_read_input_tokens"`
					CacheCreationTokens int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"response"`
		}
		json.Unmarshal(data, &evt)

		finishReason := "stop"
		if state.ToolCallIndex > 0 || state.CurrentCallID != "" {
			finishReason = "tool_calls"
		}

		var usage map[string]interface{}
		if evt.Response.Usage != nil {
			u := evt.Response.Usage
			promptTokens := u.InputTokens + u.CacheReadTokens + u.CacheCreationTokens
			if promptTokens == 0 {
				promptTokens = u.PromptTokens
			}
			completionTokens := u.OutputTokens
			if completionTokens == 0 {
				completionTokens = u.CompletionTokens
			}
			usage = map[string]interface{}{
				"prompt_tokens":     promptTokens,
				"completion_tokens": completionTokens,
				"total_tokens":      promptTokens + completionTokens,
			}

			state.PromptTokens = promptTokens
			state.CompletionTokens = completionTokens

			// Add prompt_tokens_details if cache tokens exist
			if u.CacheReadTokens > 0 || u.CacheCreationTokens > 0 {
				details := map[string]interface{}{}
				if u.CacheReadTokens > 0 {
					details["cached_tokens"] = u.CacheReadTokens
				}
				if u.CacheCreationTokens > 0 {
					details["cache_creation_tokens"] = u.CacheCreationTokens
				}
				usage["prompt_tokens_details"] = details
			}
		}

		result := formatSSEChunk(state, map[string]interface{}{}, &finishReason)
		if usage != nil {
			// Re-serialize with usage
			chunk := map[string]interface{}{
				"id":      state.ChatID,
				"object":  "chat.completion.chunk",
				"created": state.Created,
				"model":   state.Model,
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": finishReason,
					},
				},
				"usage": usage,
			}
			b, _ := json.Marshal(chunk)
			result = "data: " + string(b) + "\n\n"
		}

		return result + "data: [DONE]\n\n"

	case "error", "response.failed":
		if state.FinishReasonSent {
			return ""
		}
		state.FinishReasonSent = true

		var evt struct {
			Error *struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			} `json:"error"`
			Response *struct {
				Error *struct {
					Message string `json:"message"`
					Code    string `json:"code"`
				} `json:"error"`
			} `json:"response"`
		}
		json.Unmarshal(data, &evt)

		errMsg := "Unknown error"
		if evt.Error != nil && evt.Error.Message != "" {
			errMsg = evt.Error.Message
		} else if evt.Response != nil && evt.Response.Error != nil && evt.Response.Error.Message != "" {
			errMsg = evt.Response.Error.Message
		}

		result := formatSSEChunk(state, map[string]interface{}{
			"content": fmt.Sprintf("[Error] %s", errMsg),
		}, strPtr("stop"))
		return result + "data: [DONE]\n\n"

	case "response.reasoning_summary_text.delta":
		// Skip reasoning events — they are for display only
		return ""

	default:
		// Ignore other events (response.created, response.in_progress, etc.)
		return ""
	}
}

func formatSSEChunk(state *CodexResponseState, delta map[string]interface{}, finishReason *string) string {
	fr := interface{}(nil)
	if finishReason != nil {
		fr = *finishReason
	}

	chunk := map[string]interface{}{
		"id":      state.ChatID,
		"object":  "chat.completion.chunk",
		"created": state.Created,
		"model":   state.Model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         delta,
				"finish_reason": fr,
			},
		},
	}
	b, _ := json.Marshal(chunk)
	return "data: " + string(b) + "\n\n"
}

func strPtr(s string) *string {
	return &s
}

// ParseCodexSSE parses the Codex Responses API SSE stream body and
// converts it to OpenAI Chat Completions SSE format.
func ParseCodexSSE(body []byte, model string) []byte {
	state := NewCodexResponseState(model)
	var result strings.Builder

	lines := strings.Split(string(body), "\n")
	currentEvent := ""

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			dataStr := strings.TrimPrefix(line, "data: ")
			if currentEvent != "" {
				translated := TranslateCodexEvent(currentEvent, []byte(dataStr), state)
				if translated != "" {
					result.WriteString(translated)
				}
				currentEvent = ""
			}
			continue
		}

		if line == "" {
			currentEvent = ""
		}
	}

	// If stream ended without response.completed, flush
	if !state.FinishReasonSent {
		finishReason := "stop"
		if state.ToolCallIndex > 0 {
			finishReason = "tool_calls"
		}
		result.WriteString(formatSSEChunk(state, map[string]interface{}{}, &finishReason))
		result.WriteString("data: [DONE]\n\n")
	}

	return []byte(result.String())
}
