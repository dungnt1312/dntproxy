package commandcode

import (
	"encoding/json"
	"testing"
)

func TestBuildRequestExtractsSystemAndMapsModel(t *testing.T) {
	body := []byte(`{
		"model":"cmc/deepseek-v4-pro",
		"temperature":0.2,
		"max_tokens":128,
		"messages":[
			{"role":"system","content":"be brief"},
			{"role":"user","content":"hello"}
		]
	}`)
	got, err := buildRequest("deepseek-v4-pro", body)
	if err != nil {
		t.Fatalf("buildRequest: %v", err)
	}
	if got.Params.Model != "deepseek/deepseek-v4-pro" {
		t.Fatalf("model = %q", got.Params.Model)
	}
	if got.Params.System != "be brief" {
		t.Fatalf("system = %q", got.Params.System)
	}
	if !got.Params.Stream {
		t.Fatal("stream must be forced true")
	}
	if got.Params.MaxTokens != 128 || got.Params.Temperature != 0.2 {
		t.Fatalf("params = %+v", got.Params)
	}
	if len(got.Params.Messages) != 1 || got.Params.Messages[0].Role != "user" {
		t.Fatalf("messages = %+v", got.Params.Messages)
	}
}

func TestBuildRequestClampsMaxTokens(t *testing.T) {
	body := []byte(`{
		"model":"cmc/deepseek-v4-flash",
		"max_tokens":1000000,
		"messages":[{"role":"user","content":"hi"}]
	}`)
	got, err := buildRequest("deepseek/deepseek-v4-flash", body)
	if err != nil {
		t.Fatal(err)
	}
	if got.Params.MaxTokens != 200000 {
		t.Fatalf("max_tokens = %d, want 200000", got.Params.MaxTokens)
	}
}

func TestConvertToolsFlattensOpenAIShape(t *testing.T) {
	tools := []any{map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "lookup",
			"description": "find things",
			"parameters":  map[string]any{"type": "object"},
		},
	}}
	got := convertTools(tools)
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	raw, _ := json.Marshal(got[0])
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["name"] != "lookup" {
		t.Fatalf("name = %v", m["name"])
	}
	if _, ok := m["input_schema"]; !ok {
		t.Fatalf("missing input_schema: %s", raw)
	}
}

func TestConvertToolResultMessage(t *testing.T) {
	msgs := convertMessages([]chatMessage{
		{Role: "assistant", ToolCalls: []toolCall{{
			ID: "call_1", Type: "function", Function: functionCall{Name: "lookup", Arguments: `{"q":"x"}`},
		}}},
		{Role: "tool", ToolCallID: "call_1", Content: "ok"},
	})
	if len(msgs) != 2 {
		t.Fatalf("len = %d", len(msgs))
	}
	if msgs[1].Role != "tool" || msgs[1].Content[0].Type != "tool-result" {
		t.Fatalf("tool result = %+v", msgs[1])
	}
	if msgs[1].Content[0].Output == nil || msgs[1].Content[0].Output.Value != "ok" {
		t.Fatalf("output = %+v", msgs[1].Content[0].Output)
	}
}
