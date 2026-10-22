package kiro

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

// captureUsage runs frames through a transformer and returns the usage report
// the request logger would receive.
func captureUsage(t *testing.T, frames ...EventFrame) (UsageReport, *ResponseTransformer) {
	t.Helper()
	var got UsageReport
	tr := NewResponseTransformer("claude-opus-5.5", 1000000)
	tr.SetUsageCallback(func(u UsageReport) { got = u })
	for i := range frames {
		tr.TransformFrame(&frames[i])
	}
	tr.Flush()
	return got, tr
}

// Kiro's tail for every turn: context usage + credit metering, no token counts.
func kiroTail() []EventFrame {
	return []EventFrame{
		frameOf("contextUsageEvent", map[string]float64{"contextUsagePercentage": 12.5}),
		frameOf("meteringEvent", map[string]interface{}{"unit": "credit", "unitPlural": "credits", "usage": 0.02}),
	}
}

// A tool-only turn (no visible text) used to be recorded as 0 output tokens.
func TestUsage_ToolOnlyTurnCountsArguments(t *testing.T) {
	args := strings.Repeat(`{"command":"ls -la"}`, 10) // 200 bytes
	frames := append([]EventFrame{
		frameOf("toolUseEvent", map[string]interface{}{"toolUseId": "t1", "name": "Bash", "input": args}),
	}, kiroTail()...)

	got, _ := captureUsage(t, frames...)
	want := (len("Bash") + len(args)) / 4
	if got.OutputTokens != want {
		t.Fatalf("output tokens = %d, want %d (tool name + arguments)", got.OutputTokens, want)
	}
	if got.Source != "estimated" {
		t.Fatalf("source = %q", got.Source)
	}
}

// Kiro streams tool input as fragments of the same toolUseId: every fragment
// counts, the name counts once.
func TestUsage_StreamedToolFragmentsSumAndNameCountsOnce(t *testing.T) {
	parts := []string{`{"comm`, `and":"git `, `status"}`}
	var frames []EventFrame
	for _, p := range parts {
		frames = append(frames, frameOf("toolUseEvent", map[string]interface{}{"toolUseId": "t1", "name": "Bash", "input": p}))
	}
	frames = append(frames, kiroTail()...)

	got, _ := captureUsage(t, frames...)
	want := (len("Bash") + len(strings.Join(parts, ""))) / 4
	if got.OutputTokens != want {
		t.Fatalf("output tokens = %d, want %d", got.OutputTokens, want)
	}
}

func TestUsage_ReasoningCountsButSignatureDoesNot(t *testing.T) {
	thinking := []string{"I", "'ll use find", " combined with wc to get this done in bash.\n\n"}
	var frames []EventFrame
	for _, s := range thinking {
		frames = append(frames, frameOf("reasoningContentEvent", map[string]string{"text": s}))
	}
	frames = append(frames, frameOf("reasoningContentEvent", map[string]string{"signature": strings.Repeat("x", 400)}))
	frames = append(frames, frameOf("assistantResponseEvent", map[string]string{"content": "Done."}))
	frames = append(frames, kiroTail()...)

	got, tr := captureUsage(t, frames...)
	want := (len(strings.Join(thinking, "")) + len("Done.")) / 4
	if got.OutputTokens != want {
		t.Fatalf("output tokens = %d, want %d (reasoning text + content, signature excluded)", got.OutputTokens, want)
	}
	// Reasoning is billed but not visible text: it must not change truncation logic.
	if tr.TotalContentBytes() != len("Done.") {
		t.Fatalf("visible content bytes = %d, want %d", tr.TotalContentBytes(), len("Done."))
	}
}

func TestUsage_ReasoningIsNotForwardedOrLogged(t *testing.T) {
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	tr := NewResponseTransformer("claude-opus-5.5", 1000000)
	f := frameOf("reasoningContentEvent", map[string]string{"text": "private chain of thought"})
	if out := tr.TransformFrame(&f); len(out) != 0 {
		t.Fatalf("reasoning fragment produced client output: %q", out)
	}
	if strings.Contains(logBuf.String(), "reasoningContentEvent") || strings.Contains(logBuf.String(), "private chain") {
		t.Fatalf("reasoning was logged: %q", logBuf.String())
	}

	// Other unknown events are still logged so new event types stay visible.
	g := frameOf("someFutureEvent", map[string]string{"x": "y"})
	tr.TransformFrame(&g)
	if !strings.Contains(logBuf.String(), "unknown event=someFutureEvent") {
		t.Fatalf("unknown events must still be logged, got %q", logBuf.String())
	}
}

// Visible-text turns keep the old estimate exactly.
func TestUsage_TextOnlyTurnUnchanged(t *testing.T) {
	text := strings.Repeat("abcd", 50) // 200 bytes
	frames := append([]EventFrame{
		frameOf("assistantResponseEvent", map[string]string{"content": text}),
	}, kiroTail()...)
	got, _ := captureUsage(t, frames...)
	if got.OutputTokens != 50 {
		t.Fatalf("output tokens = %d, want 50", got.OutputTokens)
	}
}

// Real provider metrics, when present, still win over the estimate.
func TestUsage_ProviderMetricsStillWin(t *testing.T) {
	frames := []EventFrame{
		frameOf("toolUseEvent", map[string]interface{}{"toolUseId": "t1", "name": "Bash", "input": strings.Repeat("x", 4000)}),
		frameOf("metricsEvent", map[string]int{"inputTokens": 100, "outputTokens": 7}),
	}
	got, _ := captureUsage(t, frames...)
	if got.OutputTokens != 7 || got.Source != "provider_metrics" {
		t.Fatalf("usage = %+v, want provider metrics", got)
	}
}

// A reasoning-only or tool-only stream is not a truncation (no visible text).
func TestUsage_HiddenOutputDoesNotTriggerTruncation(t *testing.T) {
	_, tr := captureUsage(t, frameOf("reasoningContentEvent", map[string]string{"text": "thinking"}))
	if tr.WasTruncated() {
		t.Fatal("reasoning-only stream classified as truncated")
	}
}
