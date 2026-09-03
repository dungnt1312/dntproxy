package shared

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// maxAggregateScanToken bounds a single SSE line while aggregating.
const maxAggregateScanToken = 10 * 1024 * 1024

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *ChatCompletionUsage   `json:"usage,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason,omitempty"`
}

type ChatCompletionMessage struct {
	Role      string              `json:"role"`
	Content   string              `json:"content"`
	ToolCalls []ChatToolCallEntry `json:"tool_calls,omitempty"`
}

type ChatToolCallEntry struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function ChatToolCallFn `json:"function"`
}

type ChatToolCallFn struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamingChatChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id,omitempty"`
				Type     string `json:"type,omitempty"`
				Function struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *ChatCompletionUsage `json:"usage,omitempty"`
}

// AggregateChatCompletion reads an OpenAI-compatible SSE stream and assembles
// a single non-streaming chat completion response.
func AggregateChatCompletion(stream io.Reader, requestedModel, requestID string) (ChatCompletionResponse, error) {
	response := ChatCompletionResponse{
		ID:      "chatcmpl-" + requestID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []ChatCompletionChoice{{Index: 0, Message: ChatCompletionMessage{Role: "assistant"}}},
	}

	// Use a map of index → pointer into a separate slice to avoid dangling pointers on append.
	choiceList := []*ChatCompletionChoice{&response.Choices[0]}
	choiceMap := map[int]*ChatCompletionChoice{0: choiceList[0]}

	scanner := bufio.NewScanner(stream)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxAggregateScanToken)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var chunk streamingChatChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return ChatCompletionResponse{}, err
		}
		if chunk.ID != "" {
			response.ID = chunk.ID
		}
		if chunk.Created != 0 {
			response.Created = chunk.Created
		}
		if chunk.Model != "" {
			response.Model = chunk.Model
		}
		if chunk.Usage != nil {
			response.Usage = chunk.Usage
		}

		for _, chunkChoice := range chunk.Choices {
			choice := choiceMap[chunkChoice.Index]
			if choice == nil {
				newChoice := &ChatCompletionChoice{
					Index:   chunkChoice.Index,
					Message: ChatCompletionMessage{Role: "assistant"},
				}
				choiceList = append(choiceList, newChoice)
				choiceMap[chunkChoice.Index] = newChoice
			}
			choice = choiceMap[chunkChoice.Index]

			if chunkChoice.Delta.Role != "" {
				choice.Message.Role = chunkChoice.Delta.Role
			}
			if chunkChoice.Delta.Content != "" {
				choice.Message.Content += chunkChoice.Delta.Content
			}

			for _, tc := range chunkChoice.Delta.ToolCalls {
				idx := tc.Index
				for len(choice.Message.ToolCalls) <= idx {
					choice.Message.ToolCalls = append(choice.Message.ToolCalls, ChatToolCallEntry{Type: "function"})
				}
				dest := &choice.Message.ToolCalls[idx]
				if tc.ID != "" {
					dest.ID = tc.ID
				}
				if tc.Type != "" {
					dest.Type = tc.Type
				}
				if tc.Function.Name != "" {
					dest.Function.Name = tc.Function.Name
				}
				dest.Function.Arguments += tc.Function.Arguments
			}

			if chunkChoice.FinishReason != nil {
				choice.FinishReason = *chunkChoice.FinishReason
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ChatCompletionResponse{}, err
	}

	// Rebuild response.Choices from choiceList (avoids dangling pointer issue)
	response.Choices = make([]ChatCompletionChoice, len(choiceList))
	for i, cp := range choiceList {
		response.Choices[i] = *cp
	}

	return response, nil
}
