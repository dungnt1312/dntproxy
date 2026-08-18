package kiro

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestBuildKiroRequestHeaders(t *testing.T) {
	creds := &domain.Credentials{AccessToken: "tok-123"}
	req, err := buildKiroRequest(context.Background(), []byte(`{}`), creds, 1)
	if err != nil {
		t.Fatal(err)
	}

	if req.URL.String() != kiroBaseURL {
		t.Fatalf("URL = %s, want %s", req.URL, kiroBaseURL)
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
	retryReq, err := buildKiroRequest(context.Background(), []byte(`{}`), creds, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := retryReq.Header.Get("Amz-Sdk-Request"); got != "attempt=2; max=2" {
		t.Fatalf("Amz-Sdk-Request = %q, want %q", got, "attempt=2; max=2")
	}

	// Invocation ID must be fresh per attempt.
	first, err := buildKiroRequest(context.Background(), []byte(`{}`), creds, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.Header.Get("Amz-Sdk-Invocation-Id") == retryReq.Header.Get("Amz-Sdk-Invocation-Id") {
		t.Fatal("invocation ID reused across attempts")
	}
}

func TestBuildKiroRequestNoTokenNoAuthHeader(t *testing.T) {
	req, err := buildKiroRequest(context.Background(), []byte(`{}`), &domain.Credentials{}, 1)
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
	payload, err := BuildKiroPayload(&req, "claude-sonnet-4.6", &domain.Credentials{})
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
	payload, err := BuildKiroPayload(&req, "claude-sonnet-4.6", &domain.Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	if payload.ToolConfig != nil {
		t.Fatal("ToolConfig must be nil when no tools are present")
	}
}
