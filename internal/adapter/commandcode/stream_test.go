package commandcode

import (
	"strings"
	"testing"
	"time"
)

func TestTranslateTextAndFinish(t *testing.T) {
	state := newStreamState("deepseek/deepseek-v4-pro", time.Now().Unix())
	delta, done := translateNDJSON(`{"type":"text-delta","text":"hi"}`, state)
	if done || !strings.Contains(delta, `"content":"hi"`) || !strings.Contains(delta, `"role":"assistant"`) {
		t.Fatalf("delta = %q done=%v", delta, done)
	}
	finish, done := translateNDJSON(`{"type":"finish","finishReason":"stop","totalUsage":{"inputTokens":3,"outputTokens":1}}`, state)
	if !done || !strings.Contains(finish, `"finish_reason":"stop"`) || !strings.Contains(finish, "data: [DONE]") {
		t.Fatalf("finish = %q done=%v", finish, done)
	}
	if state.promptTokens != 3 || state.completionToks != 1 {
		t.Fatalf("usage = %d/%d", state.promptTokens, state.completionToks)
	}
}

func TestTranslateToolCallEvents(t *testing.T) {
	state := newStreamState("m", time.Now().Unix())
	start, _ := translateNDJSON(`{"type":"tool-input-start","id":"call_1","toolName":"lookup"}`, state)
	if !strings.Contains(start, `"name":"lookup"`) {
		t.Fatalf("start = %s", start)
	}
	args, _ := translateNDJSON(`{"type":"tool-input-delta","id":"call_1","delta":"{\"q\":"}`, state)
	if !strings.Contains(args, `"arguments"`) {
		t.Fatalf("args = %s", args)
	}
	dup, _ := translateNDJSON(`{"type":"tool-call","toolCallId":"call_1","toolName":"lookup","input":{"q":"x"}}`, state)
	if dup != "" {
		t.Fatalf("already streamed tool-call should be skipped, got %s", dup)
	}
}

func TestTranslateError(t *testing.T) {
	state := newStreamState("m", time.Now().Unix())
	got, done := translateNDJSON(`{"type":"error","error":{"message":"quota"}}`, state)
	if !done || !strings.Contains(got, `"quota"`) {
		t.Fatalf("error = %q done=%v", got, done)
	}
}

func TestNormalizeFinishReason(t *testing.T) {
	if got := normalizeFinishReason("tool-calls"); got != "tool_calls" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeFinishReason("max_tokens"); got != "length" {
		t.Fatalf("got %q", got)
	}
}
