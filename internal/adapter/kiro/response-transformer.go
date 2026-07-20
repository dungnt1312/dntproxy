package kiro

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
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
	Index        int      `json:"index"`
	Delta        SSEDelta `json:"delta"`
	FinishReason *string  `json:"finish_reason"`
}

// SSEDelta is the delta content in a streaming chunk.
type SSEDelta struct {
	Role      string        `json:"role,omitempty"`
	Content   string        `json:"content,omitempty"`
	ToolCalls []SSEToolCall `json:"tool_calls,omitempty"`
}

// SSEToolCall is a tool call in streaming format.
type SSEToolCall struct {
	Index    int            `json:"index"`
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Function *SSEToolCallFn `json:"function,omitempty"`
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

// UsageReport is the provider usage data captured from stream events.
type UsageReport struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	Source       string
}

// PayloadReport is a bounded response preview captured from provider events.
type PayloadReport struct {
	ResponsePreview string
	Truncated       bool
	Source          string
}

// ResponseTransformer converts parsed EventStream frames to OpenAI SSE chunks.
type ResponseTransformer struct {
	model         string
	contextWindow int // used for input token estimation when no metrics available
	responseID    string
	created       int64
	chunkIndex    int
	hasToolCalls  bool
	toolCallIdx   int
	seenToolIDs   map[string]int
	usage         *SSEUsage
	totalContent  int
	ctxPct        float64
	hasCtxUsage   bool
	hasMetering   bool
	stopReceived  bool
	finishSent    bool
	usageSource   string
	usageSent     bool
	onUsage       func(UsageReport)
	preview       strings.Builder
	previewBytes  int
	previewCut    bool
	payloadSent   bool
	onPayload     func(PayloadReport)
}

// NewResponseTransformer creates a new transformer for a response stream.
func NewResponseTransformer(model string, contextWindow int) *ResponseTransformer {
	if contextWindow <= 0 {
		contextWindow = 200000 // fallback
	}
	return &ResponseTransformer{
		model:         model,
		contextWindow: contextWindow,
		responseID:    fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli()),
		created:       time.Now().Unix(),
		seenToolIDs:   make(map[string]int),
	}
}

// SetUsageCallback registers a hook for usage persistence.
func (t *ResponseTransformer) SetUsageCallback(callback func(UsageReport)) {
	t.onUsage = callback
}

// SetPayloadCallback registers a hook for response payload preview persistence.
func (t *ResponseTransformer) SetPayloadCallback(callback func(PayloadReport)) {
	t.onPayload = callback
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
		t.stopReceived = true

	case "contextUsageEvent":
		t.handleContextUsage(frame)

	case "meteringEvent":
		t.handleMetering(frame)

	case "metricsEvent":
		t.handleMetricsEvent(frame)

	default:
		if eventType != "" {
			log.Printf("[KIRO] unknown event=%s payload=%s", eventType, string(frame.Payload))
		}
	}

	// Emit final chunk when we have usage info or stop is received.
	hasHints := t.hasMetering && t.hasCtxUsage
	if !t.finishSent && (hasHints || t.stopReceived) {
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
		if t.usage == nil && t.totalContent > 0 {
			t.estimateUsage()
		}
		chunk := t.buildChunk(SSEDelta{}, &finishReason)
		if t.usage != nil {
			chunk.Usage = t.usage
			t.notifyUsage()
		}
		results = append(results, t.marshalSSE(chunk))
	}

	t.notifyPayload()
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

	t.capturePayloadText(payload.Content)
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

	t.capturePayloadText(payload.Content)
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

		t.capturePayloadText(fmt.Sprintf("\n[tool:%s] %s", toolName, argsStr))

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

func (t *ResponseTransformer) handleMetering(frame *EventFrame) {
	t.hasMetering = true

	// Try camelCase: {"inputTokens": ..., "outputTokens": ...}
	var camel struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
	}
	if err := frame.ParsePayloadAs(&camel); err == nil {
		if camel.InputTokens > 0 || camel.OutputTokens > 0 {
			t.setMetricsUsage(camel.InputTokens, camel.OutputTokens)
			return
		}
	}

	// Try snake_case: {"input_tokens": ..., "output_tokens": ...}
	var snake struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	}
	if err := frame.ParsePayloadAs(&snake); err == nil {
		if snake.InputTokens > 0 || snake.OutputTokens > 0 {
			t.setMetricsUsage(snake.InputTokens, snake.OutputTokens)
			return
		}
	}

	// Log payload for debugging
	log.Printf("[KIRO] meteringEvent payload: %s", string(frame.Payload))
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
	// Try nested format: {"metricsEvent": {"inputTokens": ..., "outputTokens": ...}}
	var nested struct {
		MetricsEvent *struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		} `json:"metricsEvent"`
	}
	if err := frame.ParsePayloadAs(&nested); err == nil && nested.MetricsEvent != nil {
		input := nested.MetricsEvent.InputTokens
		output := nested.MetricsEvent.OutputTokens
		if input > 0 || output > 0 {
			t.setMetricsUsage(input, output)
			return
		}
	}

	// Try flat camelCase: {"inputTokens": ..., "outputTokens": ...}
	var flatCamel struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
	}
	if err := frame.ParsePayloadAs(&flatCamel); err == nil {
		if flatCamel.InputTokens > 0 || flatCamel.OutputTokens > 0 {
			t.setMetricsUsage(flatCamel.InputTokens, flatCamel.OutputTokens)
			return
		}
	}

	// Try flat snake_case: {"input_tokens": ..., "output_tokens": ...}
	var flatSnake struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	}
	if err := frame.ParsePayloadAs(&flatSnake); err == nil {
		if flatSnake.InputTokens > 0 || flatSnake.OutputTokens > 0 {
			t.setMetricsUsage(flatSnake.InputTokens, flatSnake.OutputTokens)
			return
		}
	}

	// Log unmapped metricsEvent payload for future fix
	log.Printf("[KIRO] metricsEvent unparsed: %s", string(frame.Payload))
}

func (t *ResponseTransformer) setMetricsUsage(input, output int) {
	t.usage = &SSEUsage{
		PromptTokens:     input,
		CompletionTokens: output,
		TotalTokens:      input + output,
	}
	t.usageSource = "provider_metrics"
	t.notifyUsage()
}

func (t *ResponseTransformer) emitFinalChunk() []byte {
	t.finishSent = true

	// Estimate tokens if not available from metrics
	if t.usage == nil {
		t.estimateUsage()
	}

	finishReason := "stop"
	if t.hasToolCalls {
		finishReason = "tool_calls"
	}

	chunk := t.buildChunk(SSEDelta{}, &finishReason)
	chunk.Usage = t.usage
	t.notifyUsage()
	return t.marshalSSE(chunk)
}

func (t *ResponseTransformer) estimateUsage() {
	outputTokens := 0
	if t.totalContent > 0 {
		outputTokens = max(1, t.totalContent/4)
	}
	inputTokens := 0
	if t.ctxPct > 0 {
		inputTokens = int(t.ctxPct * float64(t.contextWindow) / 100)
	}
	t.usage = &SSEUsage{
		PromptTokens:     inputTokens,
		CompletionTokens: outputTokens,
		TotalTokens:      inputTokens + outputTokens,
	}
	t.usageSource = "estimated"
}

func (t *ResponseTransformer) notifyUsage() {
	if t.usageSent || t.onUsage == nil || t.usage == nil || t.usage.TotalTokens <= 0 {
		return
	}
	source := t.usageSource
	if source == "" {
		source = "provider_metrics"
	}
	t.usageSent = true
	t.onUsage(UsageReport{
		InputTokens:  t.usage.PromptTokens,
		OutputTokens: t.usage.CompletionTokens,
		TotalTokens:  t.usage.TotalTokens,
		Source:       source,
	})
}

func (t *ResponseTransformer) capturePayloadText(text string) {
	const maxPreviewBytes = 4000
	if text == "" || t.previewBytes >= maxPreviewBytes {
		if text != "" {
			t.previewCut = true
		}
		return
	}
	for _, r := range text {
		runeBytes := len(string(r))
		if t.previewBytes+runeBytes > maxPreviewBytes {
			t.previewCut = true
			return
		}
		t.preview.WriteRune(r)
		t.previewBytes += runeBytes
	}
}

func (t *ResponseTransformer) notifyPayload() {
	if t.payloadSent || t.onPayload == nil {
		return
	}
	preview := strings.TrimSpace(t.preview.String())
	if preview == "" {
		return
	}
	t.payloadSent = true
	t.onPayload(PayloadReport{
		ResponsePreview: preview,
		Truncated:       t.previewCut,
		Source:          "kiro_eventstream",
	})
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
