package service

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestHandleChat_ComboSinglePass_NoDuplicateExecution(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{{
			ID:       "conn-1",
			Name:     "primary",
			Provider: "kiro",
			Priority: 1,
			IsActive: true,
		}},
		Combos: []domain.Combo{{
			Name:   "coder-combo",
			Models: []string{"kiro/model-a", "kiro/model-b"},
		}},
		Settings: domain.Settings{ComboStrategy: "fallback"},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-1|model-a": {Status: 500, Err: errors.New("server error")},
		"conn-1|model-b": {Status: 200, Body: "data: {\"id\":\"ok\"}\n\ndata: [DONE]\n\n"},
	})

	svc := NewChatService(store)
	svc.kiroExecutor = exec

	result := svc.HandleChat([]byte(`{"model":"coder-combo","messages":[{"role":"user","content":"hi"}]}`), "coder-combo", "req-1")
	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected combo success stream, got status=%d err=%q", result.StatusCode, result.Error)
	}
	defer result.Stream.Close()

	payload, readErr := io.ReadAll(result.Stream)
	if readErr != nil {
		t.Fatalf("read stream: %v", readErr)
	}
	if !strings.Contains(string(payload), "[DONE]") {
		t.Fatalf("unexpected stream payload: %s", string(payload))
	}

	if len(exec.calls) != 2 {
		t.Fatalf("expected 2 executions, got %d", len(exec.calls))
	}
	if exec.callCountForModel("model-a") != 1 || exec.callCountForModel("model-b") != 1 {
		t.Fatalf("expected each combo model to execute once, calls=%+v", exec.calls)
	}
}

func TestHandleChat_Client400_DoesNotFallbackOrCooldown(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-1", Name: "p1", Provider: "kiro", Priority: 1, IsActive: true},
			{ID: "conn-2", Name: "p2", Provider: "kiro", Priority: 2, IsActive: true},
		},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-1|model-a": {Status: 400, Err: errors.New("invalid request body")},
		"conn-2|model-a": {Status: 200, Body: "data: [DONE]\n\n"},
	})

	svc := NewChatService(store)
	svc.kiroExecutor = exec

	result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-2")
	if result.StatusCode != 400 {
		t.Fatalf("expected 400, got %d (%s)", result.StatusCode, result.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].ConnectionID != "conn-1" {
		t.Fatalf("expected single execution on first account, calls=%+v", exec.calls)
	}

	conn1, _ := store.GetConnectionByID("conn-1")
	if conn1 == nil {
		t.Fatal("conn-1 missing")
	}
	if conn1.RateLimitedUntil != "" || conn1.BackoffLevel != 0 {
		t.Fatalf("conn-1 should not be cooled down, got rateLimitedUntil=%q backoff=%d", conn1.RateLimitedUntil, conn1.BackoffLevel)
	}
}

func TestHandleChat_RateLimit_FallbackToNextAccount(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-1", Name: "p1", Provider: "kiro", Priority: 1, IsActive: true},
			{ID: "conn-2", Name: "p2", Provider: "kiro", Priority: 2, IsActive: true},
		},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-1|model-a": {Status: 429, Err: errors.New("rate limit exceeded")},
		"conn-2|model-a": {Status: 200, Body: "data: [DONE]\n\n"},
	})

	svc := NewChatService(store)
	svc.kiroExecutor = exec

	result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-3")
	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected 200 with stream, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()

	if len(exec.calls) != 2 || exec.calls[0].ConnectionID != "conn-1" || exec.calls[1].ConnectionID != "conn-2" {
		t.Fatalf("expected fallback conn-1 -> conn-2, calls=%+v", exec.calls)
	}

	conn1, _ := store.GetConnectionByID("conn-1")
	if conn1 == nil || conn1.RateLimitedUntil == "" {
		t.Fatalf("expected conn-1 cooldown to be set, conn=%+v", conn1)
	}
	if conn1.BackoffLevel < 1 {
		t.Fatalf("expected conn-1 backoff level increment, got %d", conn1.BackoffLevel)
	}
}

func TestHandleChat_UnsupportedModel_Returns400WithoutExecution(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{{
			ID:              "conn-1",
			Name:            "p1",
			Provider:        "kiro",
			Priority:        1,
			IsActive:        true,
			SupportedModels: []string{"model-a"},
		}},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{})

	svc := NewChatService(store)
	svc.kiroExecutor = exec

	result := svc.HandleChat([]byte(`{"model":"kiro/model-b","messages":[]}`), "kiro/model-b", "req-4")
	if result.StatusCode != 400 {
		t.Fatalf("expected 400 for unsupported model, got %d (%s)", result.StatusCode, result.Error)
	}
	if !strings.Contains(result.Error, "support model") {
		t.Fatalf("expected unsupported-model message, got %q", result.Error)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("executor must not be called for unsupported model, calls=%+v", exec.calls)
	}
}
