package xai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTranslateChatToResponses(t *testing.T) {
	body := []byte(`{
		"model":"grok/grok-4.3",
		"stream":true,
		"messages":[
			{"role":"system","content":"be concise"},
			{"role":"user","content":"hello"}
		],
		"temperature":0.2,
		"max_tokens":123
	}`)

	got, err := TranslateChatToResponses("grok-4.3", body)
	if err != nil {
		t.Fatalf("TranslateChatToResponses() error = %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("unmarshal translated: %v", err)
	}
	if payload["model"] != "grok-4.3" {
		t.Fatalf("model = %v", payload["model"])
	}
	if payload["stream"] != true {
		t.Fatalf("stream = %v", payload["stream"])
	}
	if payload["instructions"] != "be concise" {
		t.Fatalf("instructions = %v", payload["instructions"])
	}
	if payload["max_output_tokens"].(float64) != 123 {
		t.Fatalf("max_output_tokens = %v", payload["max_output_tokens"])
	}
}

func TestTranslateChatToResponsesRejectsUnsupportedTool(t *testing.T) {
	body := []byte(`{"model":"grok/grok-4.3","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"web_search"}]}`)

	_, err := TranslateChatToResponses("grok-4.3", body)
	if err == nil || !strings.Contains(err.Error(), "unsupported tool type") {
		t.Fatalf("error = %v, want unsupported tool type", err)
	}
}

func TestTranslateResponsesEventDelta(t *testing.T) {
	state := NewResponseState("grok-4.3")
	got := TranslateResponsesEvent([]byte(`{"type":"response.output_text.delta","delta":"hi"}`), state)
	if !strings.Contains(got, `"content":"hi"`) {
		t.Fatalf("translated delta = %s", got)
	}
}

func TestTranslateResponsesEventCompleted(t *testing.T) {
	state := NewResponseState("grok-4.3")
	got := TranslateResponsesEvent([]byte(`{"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`), state)
	if !strings.Contains(got, `"finish_reason":"stop"`) {
		t.Fatalf("translated completed = %s", got)
	}
	if state.Usage.PromptTokens != 10 || state.Usage.CompletionTokens != 2 {
		t.Fatalf("usage = %+v", state.Usage)
	}
}
