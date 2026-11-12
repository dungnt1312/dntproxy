package kiro

import "testing"

// Current Kiro streams end with metadataEvent{stopReason} and no
// messageStopEvent. That must count as a clean end, not a truncation.
func TestMetadataEventStopReasonIsTerminal(t *testing.T) {
	tr := NewResponseTransformer("claude-sonnet-5", 1000000)
	frames := []EventFrame{
		frameOf("assistantResponseEvent", map[string]string{"content": "pong"}),
		frameOf("metadataEvent", map[string]string{"stopReason": "END_TURN"}),
		frameOf("meteringEvent", map[string]interface{}{"unit": "credit", "usage": 0.005}),
	}
	for i := range frames {
		tr.TransformFrame(&frames[i])
	}
	tr.Flush()
	if tr.WasTruncated() {
		t.Fatal("a stream ending in metadataEvent{stopReason} was classified as truncated")
	}
}

func TestMetadataEventWithoutStopReasonIsNotTerminal(t *testing.T) {
	tr := NewResponseTransformer("claude-sonnet-5", 1000000)
	frames := []EventFrame{
		frameOf("assistantResponseEvent", map[string]string{"content": "partial"}),
		frameOf("metadataEvent", map[string]string{"conversationId": "c1"}),
	}
	for i := range frames {
		tr.TransformFrame(&frames[i])
	}
	tr.Flush()
	if !tr.WasTruncated() {
		t.Fatal("metadataEvent without stopReason must not mark the stream complete")
	}
}
