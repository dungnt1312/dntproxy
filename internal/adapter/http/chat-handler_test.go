package http

import (
	"io"
	"strings"
	"testing"
)

func TestAggregateChatCompletionFromSSE(t *testing.T) {
	stream := io.NopCloser(strings.NewReader(strings.Join([]string{
		`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"test-model","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"test-model","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
		``,
	}, "\n")))

	completion, err := aggregateChatCompletion(stream, "fallback-model", "req-1")
	if err != nil {
		t.Fatalf("aggregateChatCompletion returned error: %v", err)
	}

	if completion.ID != "chatcmpl-test" {
		t.Fatalf("expected upstream id, got %q", completion.ID)
	}
	if completion.Object != "chat.completion" {
		t.Fatalf("expected chat.completion object, got %q", completion.Object)
	}
	if completion.Created != 123 {
		t.Fatalf("expected upstream created timestamp, got %d", completion.Created)
	}
	if completion.Model != "test-model" {
		t.Fatalf("expected upstream model, got %q", completion.Model)
	}
	if len(completion.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(completion.Choices))
	}
	choice := completion.Choices[0]
	if choice.Index != 0 {
		t.Fatalf("expected choice index 0, got %d", choice.Index)
	}
	if choice.Message.Role != "assistant" {
		t.Fatalf("expected assistant role, got %q", choice.Message.Role)
	}
	if choice.Message.Content != "Hello world" {
		t.Fatalf("expected aggregated content, got %q", choice.Message.Content)
	}
	if choice.FinishReason != "stop" {
		t.Fatalf("expected stop finish reason, got %q", choice.FinishReason)
	}
}

func TestAggregateChatCompletionWithToolCalls(t *testing.T) {
	stream := io.NopCloser(strings.NewReader(strings.Join([]string{
		`data: {"id":"chatcmpl-tc","object":"chat.completion.chunk","created":456,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-tc","object":"chat.completion.chunk","created":456,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"get_weather","arguments":"{\"lo"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-tc","object":"chat.completion.chunk","created":456,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"cation\":\"NYC\"}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-tc","object":"chat.completion.chunk","created":456,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
		``,
	}, "\n")))

	completion, err := aggregateChatCompletion(stream, "fallback-model", "req-2")
	if err != nil {
		t.Fatalf("aggregateChatCompletion returned error: %v", err)
	}

	if len(completion.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(completion.Choices))
	}
	choice := completion.Choices[0]
	if choice.FinishReason != "tool_calls" {
		t.Fatalf("expected tool_calls finish reason, got %q", choice.FinishReason)
	}
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(choice.Message.ToolCalls))
	}
	tc := choice.Message.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Fatalf("expected tool call id call_123, got %q", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Fatalf("expected function name get_weather, got %q", tc.Function.Name)
	}
	expectedArgs := `{"location":"NYC"}`
	if tc.Function.Arguments != expectedArgs {
		t.Fatalf("expected arguments %q, got %q", expectedArgs, tc.Function.Arguments)
	}
}

func TestAggregateChatCompletionWithUsage(t *testing.T) {
	stream := io.NopCloser(strings.NewReader(strings.Join([]string{
		`data: {"id":"chatcmpl-u","object":"chat.completion.chunk","created":789,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-u","object":"chat.completion.chunk","created":789,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":1,"total_tokens":11}}`,
		`data: [DONE]`,
		``,
	}, "\n")))

	completion, err := aggregateChatCompletion(stream, "fallback-model", "req-3")
	if err != nil {
		t.Fatalf("aggregateChatCompletion returned error: %v", err)
	}

	if completion.Usage == nil {
		t.Fatal("expected usage to be present")
	}
	if completion.Usage.PromptTokens != 10 {
		t.Fatalf("expected 10 prompt tokens, got %d", completion.Usage.PromptTokens)
	}
	if completion.Usage.CompletionTokens != 1 {
		t.Fatalf("expected 1 completion token, got %d", completion.Usage.CompletionTokens)
	}
	if completion.Usage.TotalTokens != 11 {
		t.Fatalf("expected 11 total tokens, got %d", completion.Usage.TotalTokens)
	}
}

func TestAggregateChatCompletionMultiChoice(t *testing.T) {
	stream := io.NopCloser(strings.NewReader(strings.Join([]string{
		`data: {"id":"chatcmpl-mc","object":"chat.completion.chunk","created":100,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"A"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-mc","object":"chat.completion.chunk","created":100,"model":"test-model","choices":[{"index":1,"delta":{"role":"assistant","content":"B"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-mc","object":"chat.completion.chunk","created":100,"model":"test-model","choices":[{"index":0,"delta":{"content":"1"},"finish_reason":"stop"}]}`,
		`data: {"id":"chatcmpl-mc","object":"chat.completion.chunk","created":100,"model":"test-model","choices":[{"index":1,"delta":{"content":"2"},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
		``,
	}, "\n")))

	completion, err := aggregateChatCompletion(stream, "fallback-model", "req-4")
	if err != nil {
		t.Fatalf("aggregateChatCompletion returned error: %v", err)
	}

	if len(completion.Choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(completion.Choices))
	}
	if completion.Choices[0].Message.Content != "A1" {
		t.Fatalf("expected choice 0 content 'A1', got %q", completion.Choices[0].Message.Content)
	}
	if completion.Choices[1].Message.Content != "B2" {
		t.Fatalf("expected choice 1 content 'B2', got %q", completion.Choices[1].Message.Content)
	}
}
