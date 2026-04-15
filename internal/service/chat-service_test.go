package service

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestHandleChat_ComboFallback_NoDuplicateExecution(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{{
			ID:       "conn-1",
			Name:     "primary",
			Provider: "kiro",
			Weight:   100,
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

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)

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
			{ID: "conn-1", Name: "p1", Provider: "kiro", Weight: 100, IsActive: true},
			{ID: "conn-2", Name: "p2", Provider: "kiro", Weight: 100, IsActive: true},
		},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-1|model-a": {Status: 400, Err: errors.New("invalid request body")},
		"conn-2|model-a": {Status: 400, Err: errors.New("invalid request body")},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)

	result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-2")
	if result.StatusCode != 400 {
		t.Fatalf("expected 400, got %d (%s)", result.StatusCode, result.Error)
	}
	// Only one connection should be tried (400 = client error, no fallback)
	if len(exec.calls) != 1 {
		t.Fatalf("expected single execution, calls=%+v", exec.calls)
	}

	// Verify the used connection is not cooled down
	usedConnID := exec.calls[0].ConnectionID
	conn, _ := store.GetConnectionByID(usedConnID)
	if conn == nil {
		t.Fatalf("connection %s missing", usedConnID)
	}
	if conn.RateLimitedUntil != "" || conn.BackoffLevel != 0 {
		t.Fatalf("connection should not be cooled down, got rateLimitedUntil=%q backoff=%d", conn.RateLimitedUntil, conn.BackoffLevel)
	}
}

func TestHandleChat_RateLimit_FallbackToNextAccount(t *testing.T) {
	// Give conn-1 very high weight so it's almost always selected first
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-1", Name: "p1", Provider: "kiro", Weight: 10000, IsActive: true},
			{ID: "conn-2", Name: "p2", Provider: "kiro", Weight: 1, IsActive: true},
		},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-1|model-a": {Status: 429, Err: errors.New("rate limit exceeded")},
		"conn-2|model-a": {Status: 200, Body: "data: [DONE]\n\n"},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)

	result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-3")
	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected 200 with stream, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()

	// Should have tried 2 connections (first failed 429, second succeeded)
	if len(exec.calls) != 2 {
		t.Fatalf("expected 2 calls for fallback, got %d: %+v", len(exec.calls), exec.calls)
	}

	// conn-1 should be cooled down
	conn1, _ := store.GetConnectionByID("conn-1")
	if conn1 == nil || conn1.RateLimitedUntil == "" {
		t.Fatalf("expected conn-1 cooldown to be set, conn=%+v", conn1)
	}
}

func TestHandleChat_UnsupportedModel_Returns400WithoutExecution(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{{
			ID:              "conn-1",
			Name:            "p1",
			Provider:        "kiro",
			Weight:          100,
			IsActive:        true,
			SupportedModels: []string{"model-a"},
		}},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)

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

func TestHandleChat_AliasResolution(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{{
			ID:       "conn-1",
			Name:     "primary",
			Provider: "kiro",
			Weight:   100,
			IsActive: true,
		}},
		ModelAliases: domain.AliasMap{
			"sonnet": "kiro/claude-sonnet",
		},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-1|claude-sonnet": {Status: 200, Body: "data: [DONE]\n\n"},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)

	result := svc.HandleChat([]byte(`{"model":"sonnet","messages":[{"role":"user","content":"hi"}]}`), "sonnet", "req-5")
	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected alias resolution success, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()

	if len(exec.calls) != 1 || exec.calls[0].Model != "claude-sonnet" {
		t.Fatalf("expected execution with resolved model 'claude-sonnet', calls=%+v", exec.calls)
	}
}

func TestWeightedRandomSelect_DistributionByWeight(t *testing.T) {
	connections := []domain.ProviderConnection{
		{ID: "heavy", Weight: 900},
		{ID: "light", Weight: 100},
	}

	heavyCount := 0
	iterations := 10000
	for i := 0; i < iterations; i++ {
		selected := weightedRandomSelect(connections)
		if selected.ID == "heavy" {
			heavyCount++
		}
	}

	// Expected: ~90% heavy. Allow 85%-95% range.
	heavyPct := float64(heavyCount) / float64(iterations) * 100
	if heavyPct < 85 || heavyPct > 95 {
		t.Fatalf("expected ~90%% heavy, got %.1f%% (%d/%d)", heavyPct, heavyCount, iterations)
	}
}

func TestWeightedRandomSelect_EqualWeights(t *testing.T) {
	connections := []domain.ProviderConnection{
		{ID: "a", Weight: 100},
		{ID: "b", Weight: 100},
		{ID: "c", Weight: 100},
	}

	counts := map[string]int{}
	iterations := 10000
	for i := 0; i < iterations; i++ {
		selected := weightedRandomSelect(connections)
		counts[selected.ID]++
	}

	// Each should be roughly 33%. Allow 28%-38%.
	for id, count := range counts {
		pct := float64(count) / float64(iterations) * 100
		if pct < 28 || pct > 38 {
			t.Fatalf("connection %s has unexpected distribution: %.1f%% (%d/%d)", id, pct, count, iterations)
		}
	}
}
