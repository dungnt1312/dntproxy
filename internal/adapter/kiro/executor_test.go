package kiro

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

// codeWhispererEndpoint is the legacy surface, the only one that carries the
// X-Amz-Target RPC header.
var codeWhispererEndpoint = kiroCodeWhispererHost + generateAssistantResponsePath

func TestBuildKiroRequestHeaders(t *testing.T) {
	creds := &domain.Credentials{AccessToken: "tok-123"}
	req, err := buildKiroRequest(context.Background(), codeWhispererEndpoint, []byte(`{}`), creds, 1)
	if err != nil {
		t.Fatal(err)
	}

	if req.URL.String() != codeWhispererEndpoint {
		t.Fatalf("URL = %s, want %s", req.URL, codeWhispererEndpoint)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer tok-123" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer tok-123")
	}
	if got := req.Header.Get("X-Amz-Target"); got != "AmazonCodeWhispererStreamingService.GenerateAssistantResponse" {
		t.Fatalf("X-Amz-Target = %q", got)
	}
	if got := req.Header.Get("Amz-Sdk-Request"); got != "attempt=1; max=2" {
		t.Fatalf("Amz-Sdk-Request = %q, want %q", got, "attempt=1; max=2")
	}
	if got := req.Header.Get("Amz-Sdk-Invocation-Id"); got == "" {
		t.Fatal("Amz-Sdk-Invocation-Id is empty")
	}
}

func TestBuildKiroRequestAttemptIncrements(t *testing.T) {
	creds := &domain.Credentials{}
	retryReq, err := buildKiroRequest(context.Background(), kiroCodeWhispererHost+generateAssistantResponsePath, []byte(`{}`), creds, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := retryReq.Header.Get("Amz-Sdk-Request"); got != "attempt=2; max=2" {
		t.Fatalf("Amz-Sdk-Request = %q, want %q", got, "attempt=2; max=2")
	}

	// Invocation ID must be fresh per attempt.
	first, err := buildKiroRequest(context.Background(), kiroCodeWhispererHost+generateAssistantResponsePath, []byte(`{}`), creds, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.Header.Get("Amz-Sdk-Invocation-Id") == retryReq.Header.Get("Amz-Sdk-Invocation-Id") {
		t.Fatal("invocation ID reused across attempts")
	}
}

func TestBuildKiroRequestNoTokenNoAuthHeader(t *testing.T) {
	req, err := buildKiroRequest(context.Background(), kiroCodeWhispererHost+generateAssistantResponsePath, []byte(`{}`), &domain.Credentials{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if auth := req.Header.Get("Authorization"); auth != "" {
		t.Fatalf("Authorization = %q, want empty", auth)
	}
	if strings.Contains(req.Header.Get("User-Agent"), "\n") {
		t.Fatal("User-Agent must not contain header injection")
	}
}

func TestBuildKiroPayloadIncludesToolConfig(t *testing.T) {
	body := `{
		"model": "claude-sonnet-4.6",
		"messages": [{"role":"user","content":"hi"}],
		"tools": [{
			"type": "function",
			"function": {
				"name": "read_file",
				"description": "Read a file",
				"parameters": {"type":"object","properties":{"path":{"type":"string"}}}
			}
		}]
	}`
	var req OpenAIRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	payload, err := BuildKiroPayload(context.Background(), &req, "claude-sonnet-4.6", &domain.Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	if payload.ToolConfig == nil {
		t.Fatal("ToolConfig must be set when tools are present")
	}
	if len(payload.ToolConfig.Tools) != 1 {
		t.Fatalf("ToolConfig.Tools = %d, want 1", len(payload.ToolConfig.Tools))
	}
	tool := payload.ToolConfig.Tools[0]
	if tool.ToolSpecification.Name != "read_file" {
		t.Fatalf("tool name = %q, want read_file", tool.ToolSpecification.Name)
	}
	if tool.ToolSpecification.InputSchema.JSON == nil {
		t.Fatal("inputSchema.json must be present")
	}

	// Serialize and confirm top-level toolConfig with Bedrock-style toolSpec.
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["toolConfig"]; !ok {
		t.Fatalf("payload missing top-level toolConfig: %s", raw)
	}
}

func TestBuildKiroPayloadNoToolConfigWithoutTools(t *testing.T) {
	body := `{"model":"claude-sonnet-4.6","messages":[{"role":"user","content":"hi"}]}`
	var req OpenAIRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	payload, err := BuildKiroPayload(context.Background(), &req, "claude-sonnet-4.6", &domain.Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	if payload.ToolConfig != nil {
		t.Fatal("ToolConfig must be nil when no tools are present")
	}
}

func TestBuildKiroPayloadAttachesInlineDataURLs(t *testing.T) {
	dataURL := "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQ"
	body := `{"model":"claude-sonnet-4.6","messages":[{"role":"user","content":[` +
		`{"type":"text","text":"What is in this image?"},` +
		`{"type":"image_url","image_url":{"url":"` + dataURL + `"}}]}]}`
	var req OpenAIRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	payload, err := BuildKiroPayload(context.Background(), &req, "claude-sonnet-4.6", &domain.Credentials{})
	if err != nil {
		t.Fatal(err)
	}

	msg := payload.ConversationState.CurrentMessage.UserInputMessage
	if msg == nil {
		t.Fatal("missing current user message")
	}
	if len(msg.Images) != 1 {
		t.Fatalf("images = %d, want 1 (payload: %s)", len(msg.Images), mustJSON(t, payload))
	}
	if msg.Images[0].Format != "jpeg" {
		t.Fatalf("format = %q, want jpeg", msg.Images[0].Format)
	}
	if msg.Images[0].Source.Bytes != "/9j/4AAQSkZJRgABAQ" {
		t.Fatalf("bytes mismatch: %q", msg.Images[0].Source.Bytes)
	}
	// The base64 blob must not leak into the text content.
	if strings.Contains(msg.Content, "/9j/4AAQ") {
		t.Fatalf("base64 leaked into content: %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "What is in this image?") {
		t.Fatalf("prompt text missing: %q", msg.Content)
	}
}

func TestBuildKiroPayloadRejectsUnsafeRemoteImageURL(t *testing.T) {
	body := `{"model":"claude-sonnet-4.6","messages":[{"role":"user","content":[` +
		`{"type":"text","text":"describe"},` +
		`{"type":"image_url","image_url":{"url":"http://127.0.0.1/cat.png"}}]}]}`
	var req OpenAIRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	_, err := BuildKiroPayload(context.Background(), &req, "claude-sonnet-4.6", &domain.Credentials{})
	if err == nil || !strings.Contains(err.Error(), "load image input") {
		t.Fatalf("error = %v, want safe remote-image rejection", err)
	}
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
