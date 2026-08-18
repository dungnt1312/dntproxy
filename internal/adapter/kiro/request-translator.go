package kiro

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/google/uuid"
)

// KiroMessage represents a message in Kiro conversation history.
type KiroMessage struct {
	UserInputMessage         *KiroUserMessage      `json:"userInputMessage,omitempty"`
	AssistantResponseMessage *KiroAssistantMessage `json:"assistantResponseMessage,omitempty"`
}

// KiroUserMessage is a user message in Kiro format.
type KiroUserMessage struct {
	Content                 string                  `json:"content"`
	ModelID                 string                  `json:"modelId"`
	Origin                  string                  `json:"origin,omitempty"`
	UserInputMessageContext *KiroUserMessageContext `json:"userInputMessageContext,omitempty"`
}

// KiroUserMessageContext holds tools and tool results.
type KiroUserMessageContext struct {
	Tools       []KiroTool       `json:"tools,omitempty"`
	ToolResults []KiroToolResult `json:"toolResults,omitempty"`
}

// KiroTool represents a tool specification.
type KiroTool struct {
	ToolSpecification KiroToolSpec `json:"toolSpecification"`
}

// KiroToolSpec is the tool specification details.
type KiroToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema KiroInputSchema `json:"inputSchema"`
}

// KiroInputSchema wraps the JSON schema.
type KiroInputSchema struct {
	JSON json.RawMessage `json:"json"`
}

// KiroToolResult represents a tool execution result.
type KiroToolResult struct {
	ToolUseID string            `json:"toolUseId"`
	Status    string            `json:"status"`
	Content   []KiroTextContent `json:"content"`
}

// KiroTextContent is a text content block.
type KiroTextContent struct {
	Text string `json:"text"`
}

// KiroAssistantMessage is an assistant message in Kiro format.
type KiroAssistantMessage struct {
	Content  string        `json:"content"`
	ToolUses []KiroToolUse `json:"toolUses,omitempty"`
}

// KiroToolUse represents a tool use by the assistant.
type KiroToolUse struct {
	ToolUseID string      `json:"toolUseId"`
	Name      string      `json:"name"`
	Input     interface{} `json:"input"`
}

// KiroPayload is the full request payload for Kiro API.
type KiroPayload struct {
	ConversationState KiroConversationState `json:"conversationState"`
	ProfileArn        string                `json:"profileArn,omitempty"`
	InferenceConfig   *KiroInferenceConfig  `json:"inferenceConfig,omitempty"`
	ToolConfig        *KiroToolConfig       `json:"toolConfig,omitempty"`
}

// KiroToolConfig is the Bedrock-style tool configuration required by Kiro
// whenever toolUse/toolResult content blocks are present.
type KiroToolConfig struct {
	Tools []KiroTool `json:"tools"`
}

// KiroConversationState holds the conversation context.
type KiroConversationState struct {
	ChatTriggerType string        `json:"chatTriggerType"`
	ConversationID  string        `json:"conversationId"`
	CurrentMessage  KiroMessage   `json:"currentMessage"`
	History         []KiroMessage `json:"history"`
}

// KiroInferenceConfig holds inference parameters.
type KiroInferenceConfig struct {
	MaxTokens   int      `json:"maxTokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"topP,omitempty"`
}

// OpenAI types for parsing incoming requests.

// OpenAIRequest is the incoming OpenAI chat completion request.
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
	Stream      *bool           `json:"stream,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
}

// OpenAIMessage is a message in OpenAI format.
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content"` // string or []ContentBlock
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// ContentBlock is a content block in OpenAI multimodal format.
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ImageURL  interface{}     `json:"image_url,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"` // for tool_result
}

// OpenAIToolCall is a tool call in OpenAI format.
type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function OpenAIFunctionCall `json:"function"`
}

// OpenAIFunctionCall is a function call.
type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// OpenAITool is a tool definition in OpenAI format.
type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

// OpenAIFunction is a function definition.
type OpenAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// BuildKiroPayload converts an OpenAI request to Kiro format.
func BuildKiroPayload(req *OpenAIRequest, model string, creds *domain.Credentials) (*KiroPayload, error) {
	tools := convertTools(req.Tools)
	history, currentMsg, effectiveTools := convertMessages(req.Messages, tools, model)

	// Inject timestamp into current message content
	finalContent := ""
	if currentMsg != nil && currentMsg.UserInputMessage != nil {
		finalContent = currentMsg.UserInputMessage.Content
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	finalContent = fmt.Sprintf("[Context: Current time is %s]\n\n%s", timestamp, finalContent)

	// Build current message
	current := KiroMessage{
		UserInputMessage: &KiroUserMessage{
			Content: finalContent,
			ModelID: model,
			Origin:  "AI_EDITOR",
		},
	}

	// Copy context (tools + tool results) from converted current message
	if currentMsg != nil && currentMsg.UserInputMessage != nil && currentMsg.UserInputMessage.UserInputMessageContext != nil {
		current.UserInputMessage.UserInputMessageContext = currentMsg.UserInputMessage.UserInputMessageContext
	}

	payload := &KiroPayload{
		ConversationState: KiroConversationState{
			ChatTriggerType: "MANUAL",
			ConversationID:  uuid.New().String(),
			CurrentMessage:  current,
			History:         history,
		},
	}

	// Bedrock/Kiro requires toolConfig whenever tools are used (toolUse and
	// toolResult content blocks). Without it the API rejects with 400
	// TOOL_CONFIG_MISSING and the stream never starts.
	if len(effectiveTools) > 0 {
		payload.ToolConfig = &KiroToolConfig{Tools: effectiveTools}
	}

	// Set profileArn
	if creds != nil {
		profileArn := creds.GetProfileArn()
		if profileArn != "" {
			payload.ProfileArn = profileArn
		}
	}

	// Inference config
	payload.InferenceConfig = &KiroInferenceConfig{
		MaxTokens: 32000,
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		payload.InferenceConfig.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		payload.InferenceConfig.Temperature = req.Temperature
	}
	if req.TopP != nil {
		payload.InferenceConfig.TopP = req.TopP
	}

	return payload, nil
}

func convertTools(tools []OpenAITool) []KiroTool {
	if len(tools) == 0 {
		return nil
	}

	var result []KiroTool
	for _, t := range tools {
		name := t.Function.Name
		desc := t.Function.Description
		if strings.TrimSpace(desc) == "" {
			desc = fmt.Sprintf("Tool: %s", name)
		}

		schema := t.Function.Parameters
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
		} else {
			// Ensure required field exists
			var schemaMap map[string]interface{}
			if err := json.Unmarshal(schema, &schemaMap); err == nil {
				if _, ok := schemaMap["required"]; !ok {
					schemaMap["required"] = []interface{}{}
					schema, _ = json.Marshal(schemaMap)
				}
			}
		}

		result = append(result, KiroTool{
			ToolSpecification: KiroToolSpec{
				Name:        name,
				Description: desc,
				InputSchema: KiroInputSchema{JSON: schema},
			},
		})
	}
	return result
}

func convertMessages(messages []OpenAIMessage, tools []KiroTool, model string) ([]KiroMessage, *KiroMessage, []KiroTool) {
	var history []KiroMessage
	var currentMessage *KiroMessage

	var pendingUserContent []string
	var pendingAssistantContent []string
	var pendingToolResults []KiroToolResult
	var currentRole string

	flushPending := func() {
		if currentRole == "user" {
			content := strings.Join(pendingUserContent, "\n\n")
			if strings.TrimSpace(content) == "" {
				content = "continue"
			}

			userMsg := KiroMessage{
				UserInputMessage: &KiroUserMessage{
					Content: content,
					ModelID: "",
				},
			}

			if len(pendingToolResults) > 0 {
				if userMsg.UserInputMessage.UserInputMessageContext == nil {
					userMsg.UserInputMessage.UserInputMessageContext = &KiroUserMessageContext{}
				}
				userMsg.UserInputMessage.UserInputMessageContext.ToolResults = pendingToolResults
			}

			// Add tools to first user message
			if len(tools) > 0 && len(history) == 0 {
				if userMsg.UserInputMessage.UserInputMessageContext == nil {
					userMsg.UserInputMessage.UserInputMessageContext = &KiroUserMessageContext{}
				}
				userMsg.UserInputMessage.UserInputMessageContext.Tools = tools
			}

			history = append(history, userMsg)
			currentMessage = &userMsg
			pendingUserContent = nil
			pendingToolResults = nil

		} else if currentRole == "assistant" {
			content := strings.Join(pendingAssistantContent, "\n\n")
			if strings.TrimSpace(content) == "" {
				content = "..."
			}

			assistantMsg := KiroMessage{
				AssistantResponseMessage: &KiroAssistantMessage{
					Content: content,
				},
			}
			history = append(history, assistantMsg)
			pendingAssistantContent = nil
		}
	}

	for _, msg := range messages {
		role := msg.Role
		// Normalize: system/tool → user
		if role == "system" || role == "tool" {
			role = "user"
		}

		// If role changes, flush pending
		if role != currentRole && currentRole != "" {
			flushPending()
		}
		currentRole = role

		if role == "user" {
			content, toolResults := extractUserContent(msg)

			if msg.Role == "tool" {
				// Tool role message
				toolContent := extractStringContent(msg.Content)
				pendingToolResults = append(pendingToolResults, KiroToolResult{
					ToolUseID: msg.ToolCallID,
					Status:    "success",
					Content:   []KiroTextContent{{Text: toolContent}},
				})
			} else {
				if content != "" {
					pendingUserContent = append(pendingUserContent, content)
				}
				pendingToolResults = append(pendingToolResults, toolResults...)
			}

		} else if role == "assistant" {
			textContent, toolUses := extractAssistantContent(msg)

			if textContent != "" {
				pendingAssistantContent = append(pendingAssistantContent, textContent)
			}

			if len(toolUses) > 0 {
				flushPending()

				// Attach tool uses to last assistant message
				if len(history) > 0 {
					last := &history[len(history)-1]
					if last.AssistantResponseMessage != nil {
						last.AssistantResponseMessage.ToolUses = toolUses
					}
				}
				currentRole = ""
			}
		}
	}

	// Flush remaining
	if currentRole != "" {
		flushPending()
	}

	// Pop last userInputMessage as currentMessage
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].UserInputMessage != nil {
			msg := history[i]
			currentMessage = &msg
			history = append(history[:i], history[i+1:]...)
			break
		}
	}

	// Grab tools from first history item before cleanup
	var firstHistoryTools []KiroTool
	if len(history) > 0 && history[0].UserInputMessage != nil &&
		history[0].UserInputMessage.UserInputMessageContext != nil {
		firstHistoryTools = history[0].UserInputMessage.UserInputMessageContext.Tools
	}

	// Clean up history: remove tools from history items, set modelId
	for i := range history {
		if history[i].UserInputMessage != nil {
			if history[i].UserInputMessage.UserInputMessageContext != nil {
				history[i].UserInputMessage.UserInputMessageContext.Tools = nil
				if len(history[i].UserInputMessage.UserInputMessageContext.ToolResults) == 0 {
					history[i].UserInputMessage.UserInputMessageContext = nil
				}
			}
			if history[i].UserInputMessage.ModelID == "" {
				history[i].UserInputMessage.ModelID = model
			}
		}
	}

	// Inject tools into currentMessage
	effectiveTools := firstHistoryTools
	if len(effectiveTools) == 0 {
		effectiveTools = tools
	}
	if len(effectiveTools) > 0 && currentMessage != nil && currentMessage.UserInputMessage != nil {
		if currentMessage.UserInputMessage.UserInputMessageContext == nil {
			currentMessage.UserInputMessage.UserInputMessageContext = &KiroUserMessageContext{}
		}
		if len(currentMessage.UserInputMessage.UserInputMessageContext.Tools) == 0 {
			currentMessage.UserInputMessage.UserInputMessageContext.Tools = effectiveTools
		}
	}

	return history, currentMessage, effectiveTools
}

func extractStringContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func extractUserContent(msg OpenAIMessage) (string, []KiroToolResult) {
	if len(msg.Content) == 0 {
		return "", nil
	}

	// Try as string first
	var s string
	if err := json.Unmarshal(msg.Content, &s); err == nil {
		return s, nil
	}

	// Try as array of content blocks
	var blocks []ContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return string(msg.Content), nil
	}

	var textParts []string
	var toolResults []KiroToolResult

	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				textParts = append(textParts, b.Text)
			}
		case "image_url":
			imgURL := extractImageURL(b.ImageURL)
			if imgURL != "" {
				textParts = append(textParts, fmt.Sprintf("[Image: %s]", imgURL))
			}
		case "tool_result":
			text := ""
			if len(b.Content) > 0 {
				// Content can be string or array
				var s string
				if err := json.Unmarshal(b.Content, &s); err == nil {
					text = s
				} else {
					var innerBlocks []ContentBlock
					if err := json.Unmarshal(b.Content, &innerBlocks); err == nil {
						for _, ib := range innerBlocks {
							if ib.Text != "" {
								text += ib.Text + "\n"
							}
						}
					}
				}
			}
			toolResults = append(toolResults, KiroToolResult{
				ToolUseID: b.ToolUseID,
				Status:    "success",
				Content:   []KiroTextContent{{Text: text}},
			})
		}
	}

	return strings.Join(textParts, "\n"), toolResults
}

func extractImageURL(imageURL interface{}) string {
	if imageURL == nil {
		return ""
	}
	if urlMap, ok := imageURL.(map[string]interface{}); ok {
		if url, ok := urlMap["url"].(string); ok {
			return url
		}
	}
	if url, ok := imageURL.(string); ok {
		return url
	}
	return ""
}

func extractAssistantContent(msg OpenAIMessage) (string, []KiroToolUse) {
	var textContent string
	var toolUses []KiroToolUse

	// Check for tool_calls field (OpenAI format)
	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			var input interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				input = map[string]interface{}{}
			}
			toolUses = append(toolUses, KiroToolUse{
				ToolUseID: tc.ID,
				Name:      tc.Function.Name,
				Input:     input,
			})
		}
	}

	// Extract text content
	if len(msg.Content) == 0 {
		return "", toolUses
	}

	var s string
	if err := json.Unmarshal(msg.Content, &s); err == nil {
		textContent = strings.TrimSpace(s)
		return textContent, toolUses
	}

	// Array of content blocks (Claude format)
	var blocks []ContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err == nil {
		var textParts []string
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if b.Text != "" {
					textParts = append(textParts, b.Text)
				}
			case "tool_use":
				if b.Name == "" {
					continue
				}
				var input interface{}
				if len(b.Input) > 0 {
					_ = json.Unmarshal(b.Input, &input)
				}
				toolUses = append(toolUses, KiroToolUse{
					ToolUseID: b.ID,
					Name:      b.Name,
					Input:     input,
				})
			}
		}
		textContent = strings.TrimSpace(strings.Join(textParts, "\n"))
	}

	return textContent, toolUses
}
