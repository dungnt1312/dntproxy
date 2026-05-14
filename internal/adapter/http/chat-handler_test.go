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
