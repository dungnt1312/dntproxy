package kiro

import (
	"encoding/json"
	"strings"
	"testing"
)

// frameOf builds an EventFrame with the given event type and JSON payload.
func frameOf(eventType string, payload interface{}) EventFrame {
	var raw json.RawMessage
	if payload != nil {
		b, _ := json.Marshal(payload)
		raw = json.RawMessage(b)
	}
	return EventFrame{
		Headers: map[string]string{":event-type": eventType},
		Payload: raw,
	}
}

// finishReasonFromSSE extracts the finish_reason from the first SSE chunk that
// carries one, across the concatenated output bytes.
func finishReasonFromSSE(t *testing.T, chunks [][]byte) string {
	t.Helper()
	for _, c := range chunks {
		for _, line := range strings.Split(string(c), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var chunk SSEChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}
			for _, ch := range chunk.Choices {
				if ch.FinishReason != nil {
					return *ch.FinishReason
				}
			}
		}
	}
	return ""
}

// A stream that delivered content but hit EOF with no messageStopEvent and no
// tool call is truncated: it must finish as "length", not "stop".
func TestFlush_TruncatedContentFinishesAsLength(t *testing.T) {
	tr := NewResponseTransformer("kr/claude", 200000)
	tr.TransformFrame(ptr(frameOf("assistantResponseEvent", map[string]string{"content": "partial answer"})))

	got := finishReasonFromSSE(t, tr.Flush())
	if got != "length" {
		t.Fatalf("finish_reason = %q, want %q (truncated stream)", got, "length")
	}
}

// A stream that received messageStopEvent finished cleanly: finish as "stop".
func TestFlush_CleanStopFinishesAsStop(t *testing.T) {
	tr := NewResponseTransformer("kr/claude", 200000)
	tr.TransformFrame(ptr(frameOf("assistantResponseEvent", map[string]string{"content": "full answer"})))
	tr.TransformFrame(ptr(frameOf("messageStopEvent", nil)))

	// messageStopEvent alone (no metrics/ctx) does not emit the final chunk in
	// TransformFrame, so Flush produces it. Either way the reason must be "stop".
	got := finishReasonFromSSE(t, tr.Flush())
	if got != "" && got != "stop" {
		t.Fatalf("finish_reason = %q, want %q or empty (already finished)", got, "stop")
	}
	if tr.isTruncated() {
		t.Fatal("isTruncated() = true after messageStopEvent, want false")
	}
}

// A stream that delivered a tool call but no stop event is complete, not
// truncated: finish as "tool_calls".
func TestFlush_ToolCallFinishesAsToolCalls(t *testing.T) {
	tr := NewResponseTransformer("kr/claude", 200000)
	tr.TransformFrame(ptr(frameOf("toolUseEvent", map[string]interface{}{
		"toolUseId": "call_1",
		"name":      "get_weather",
		"input":     map[string]string{"city": "hanoi"},
	})))

	got := finishReasonFromSSE(t, tr.Flush())
	if got != "tool_calls" {
		t.Fatalf("finish_reason = %q, want %q", got, "tool_calls")
	}
	if tr.isTruncated() {
		t.Fatal("isTruncated() = true with a tool call, want false")
	}
}

// A stream with no content at all is not classified as truncated (empty
// response is handled elsewhere), so it does not get the "length" reason.
func TestFlush_EmptyStreamNotTruncated(t *testing.T) {
	tr := NewResponseTransformer("kr/claude", 200000)
	if tr.isTruncated() {
		t.Fatal("isTruncated() = true for empty stream, want false")
	}
	got := finishReasonFromSSE(t, tr.Flush())
	if got != "stop" {
		t.Fatalf("finish_reason = %q, want %q for empty stream", got, "stop")
	}
}

func ptr(f EventFrame) *EventFrame { return &f }
