package kiro

import (
	"encoding/json"
	"fmt"
	"time"
)

// OpenAI SSE chunk types for response streaming.

// SSEChunk is an OpenAI-compatible streaming chunk.
type SSEChunk struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
	Model   string      `json:"model"`
	Choices []SSEChoice `json:"choices"`
	Usage   *SSEUsage   `json:"usage,omitempty"`
}

// SSEChoice is a choice in a streaming chunk.
type SSEChoice struct {
	Index        int       `json:"index"`
	Delta        SSEDelta  `json:"delta"`
	FinishReason *string   `json:"finish_reason"`
}

// SSEDelta is the delta content in a streaming chunk.
type SSEDelta struct {
	Role      string         `json:"role,omitempty"`
	Content   string         `json:"content,omitempty"`
	ToolCalls []SSEToolCall  `json:"tool_calls,omitempty"`
}

// SSEToolCall is a tool call in streaming format.
type SSEToolCall struct {
	Index    int             `json:"index"`
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type,omitempty"`
	Function *SSEToolCallFn  `json:"function,omitempty"`
}

// SSEToolCallFn is the function part of a streaming tool call.
type SSEToolCallFn struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// SSEUsage is token usage info.
type SSEUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ResponseTransformer converts parsed EventStream frames to OpenAI SSE chunks.
type ResponseTransformer struct {
	model        string
	responseID   string
	created      int64
	chunkIndex   int
	hasToolCalls bool
	toolCallIdx  int
	seenToolIDs  map[string]int
	usage        *SSEUsage
	totalContent int
	ctxPct       float64
	hasCtxUsage  bool
	hasMetering  bool
	finishSent   bool
}

// NewResponseTransformer creates a new transformer for a response stream.
func NewResponseTransformer(model string) *ResponseTransformer {
	return &ResponseTransformer{
		model:       model,
		responseID:  fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli()),
		created:     time.Now().Unix(),
		seenToolIDs: make(map[string]int),
	}
}

// TransformFrame converts a single EventStream frame to zero or more SSE lines.
// Returns SSE-formatted bytes (e.g. "data: {...}\n\n") or nil if frame should be skipped.
func (t *ResponseTransformer) TransformFrame(frame *EventFrame) [][]byte {
	eventType := frame.GetEventType()
	var results [][]byte

	switch eventType {
	case "assistantResponseEvent":
		if chunk := t.handleAssistantResponse(frame); chunk != nil {
			results = append(results, chunk)
		}

	case "codeEvent":
		if chunk := t.handleCodeEvent(frame); chunk != nil {
			results = append(results, chunk)
		}

	case "toolUseEvent":
		chunks := t.handleToolUseEvent(frame)
		results = append(results, chunks...)

	case "messageStopEvent":
		if chunk := t.handleMessageStop(); chunk != nil {
			results = append(results, chunk)
		}

	case "contextUsageEvent":
		t.handleContextUsage(frame)

	case "meteringEvent":
		t.hasMetering = true

	case "metricsEvent":
		t.handleMetricsEvent(frame)
	}

	// Emit final chunk after both metering + context usage received
	if t.hasMetering && t.hasCtxUsage && !t.finishSent {
		if chunk := t.emitFinalChunk(); chunk != nil {
			results = append(results, chunk)
		}
	}

	return results
}

// Flush emits any remaining finish/done markers.
func (t *ResponseTransformer) Flush() [][]byte {
	var results [][]byte

	if !t.finishSent {
		t.finishSent = true
		finishReason := "stop"
		if t.hasToolCalls {
			finishReason = "tool_calls"
		}
		chunk := t.buildChunk(SSEDelta{}, &finishReason)
		if t.usage != nil {
			chunk.Usage = t.usage
		}
		results = append(results, t.marshalSSE(chunk))
	}

	results = append(results, []byte("data: [DONE]\n\n"))
	return results
}

func (t *ResponseTransformer) handleAssistantResponse(frame *EventFrame) []byte {
	var payload struct {
		Content string `json:"content"`
	}
	if err := frame.ParsePayloadAs(&payload); err != nil || payload.Content == "" {
		return nil
	}

	t.totalContent += len(payload.Content)

	delta := SSEDelta{Content: payload.Content}
	if t.chunkIndex == 0 {
		delta.Role = "assistant"
	}
	t.chunkIndex++

	return t.marshalSSE(t.buildChunk(delta, nil))
}

func (t *ResponseTransformer) handleCodeEvent(frame *EventFrame) []byte {
	var payload struct {
		Content string `json:"content"`
	}
	if err := frame.ParsePayloadAs(&payload); err != nil || payload.Content == "" {
		return nil
	}

	t.totalContent += len(payload.Content)
	delta := SSEDelta{Content: payload.Content}
	t.chunkIndex++

	return t.marshalSSE(t.buildChunk(delta, nil))
}

func (t *ResponseTransformer) handleToolUseEvent(frame *EventFrame) [][]byte {
	t.hasToolCalls = true
	var results [][]byte

	// Try parsing as single tool use or array
	var singleTool struct {
		ToolUseID string      `json:"toolUseId"`
		Name      string      `json:"name"`
		Input     interface{} `json:"input"`
	}

	if err := frame.ParsePayloadAs(&singleTool); err == nil && singleTool.Name != "" {
		results = append(results, t.emitToolUse(singleTool.ToolUseID, singleTool.Name, singleTool.Input)...)
		return results
	}

	// Try as array
	var toolArray []struct {
		ToolUseID string      `json:"toolUseId"`
		Name      string      `json:"name"`
		Input     interface{} `json:"input"`
	}
	if err := frame.ParsePayloadAs(&toolArray); err == nil {
		for _, tu := range toolArray {
			results = append(results, t.emitToolUse(tu.ToolUseID, tu.Name, tu.Input)...)
		}
	}

	return results
}

func (t *ResponseTransformer) emitToolUse(toolCallID, toolName string, toolInput interface{}) [][]byte {
	var results [][]byte

	if toolCallID == "" {
		toolCallID = fmt.Sprintf("call_%d", time.Now().UnixMilli())
	}

	// Check if this is a new tool call
	toolIndex, seen := t.seenToolIDs[toolCallID]
	if !seen {
		toolIndex = t.toolCallIdx
		t.seenToolIDs[toolCallID] = toolIndex
		t.toolCallIdx++

		// Emit start chunk with name
		delta := SSEDelta{}
		if t.chunkIndex == 0 {
			delta.Role = "assistant"
		}
		delta.ToolCalls = []SSEToolCall{{
			Index: toolIndex,
			ID:    toolCallID,
			Type:  "function",
			Function: &SSEToolCallFn{
				Name:      toolName,
				Arguments: "",
			},
		}}
		t.chunkIndex++
		results = append(results, t.marshalSSE(t.buildChunk(delta, nil)))
	}

	// Emit arguments chunk
	if toolInput != nil {
		var argsStr string
		switch v := toolInput.(type) {
		case string:
			argsStr = v
		default:
			b, _ := json.Marshal(v)
			argsStr = string(b)
		}

		delta := SSEDelta{
			ToolCalls: []SSEToolCall{{
				Index: toolIndex,
				Function: &SSEToolCallFn{
					Arguments: argsStr,
				},
			}},
		}
		t.chunkIndex++
		results = append(results, t.marshalSSE(t.buildChunk(delta, nil)))
	}

	return results
}

func (t *ResponseTransformer) handleMessageStop() []byte {
	finishReason := "stop"
	if t.hasToolCalls {
		finishReason = "tool_calls"
	}
	t.finishSent = true

	chunk := t.buildChunk(SSEDelta{}, &finishReason)
	return t.marshalSSE(chunk)
}

func (t *ResponseTransformer) handleContextUsage(frame *EventFrame) {
	var payload struct {
		ContextUsagePercentage float64 `json:"contextUsagePercentage"`
	}
	if err := frame.ParsePayloadAs(&payload); err == nil && payload.ContextUsagePercentage > 0 {
		t.ctxPct = payload.ContextUsagePercentage
		t.hasCtxUsage = true
	}
}

func (t *ResponseTransformer) handleMetricsEvent(frame *EventFrame) {
	// Try nested metricsEvent or flat
	var payload struct {
		MetricsEvent *struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		} `json:"metricsEvent"`
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
	}
	if err := frame.ParsePayloadAs(&payload); err != nil {
		return
	}

	input := payload.InputTokens
	output := payload.OutputTokens
	if payload.MetricsEvent != nil {
		input = payload.MetricsEvent.InputTokens
		output = payload.MetricsEvent.OutputTokens
	}

	if input > 0 || output > 0 {
		t.usage = &SSEUsage{
			PromptTokens:     input,
			CompletionTokens: output,
			TotalTokens:      input + output,
		}
	}
}

func (t *ResponseTransformer) emitFinalChunk() []byte {
	t.finishSent = true

	// Estimate tokens if not available from metrics
	if t.usage == nil {
		outputTokens := 0
		if t.totalContent > 0 {
			outputTokens = max(1, t.totalContent/4)
		}
		inputTokens := 0
		if t.ctxPct > 0 {
			inputTokens = int(t.ctxPct * 200000 / 100)
		}
		t.usage = &SSEUsage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		}
	}

	finishReason := "stop"
	if t.hasToolCalls {
		finishReason = "tool_calls"
	}

	chunk := t.buildChunk(SSEDelta{}, &finishReason)
	chunk.Usage = t.usage
	return t.marshalSSE(chunk)
}

func (t *ResponseTransformer) buildChunk(delta SSEDelta, finishReason *string) *SSEChunk {
	return &SSEChunk{
		ID:      t.responseID,
		Object:  "chat.completion.chunk",
		Created: t.created,
		Model:   t.model,
		Choices: []SSEChoice{{
			Index:        0,
			Delta:        delta,
			FinishReason: finishReason,
		}},
	}
}

func (t *ResponseTransformer) marshalSSE(chunk *SSEChunk) []byte {
	data, _ := json.Marshal(chunk)
	return []byte(fmt.Sprintf("data: %s\n\n", data))
}
