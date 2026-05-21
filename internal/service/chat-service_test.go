package service

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
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

	result := svc.HandleChat([]byte(`{"model":"coder-combo","messages":[{"role":"user","content":"hi"}]}`), "coder-combo", "req-1", nil)
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

	result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-2", nil)
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

	result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-3", nil)
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

func TestHandleChat_ModelEntitlementLocksModelOnly(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-1", Name: "p1", Provider: "openai-compatible", Priority: 1, Weight: 100, IsActive: true},
			{ID: "conn-2", Name: "p2", Provider: "openai-compatible", Priority: 2, Weight: 100, IsActive: true},
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyPriorityFallback},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-1|claude-4-opus": {Status: 403, Err: errors.New(`returned 403: {"error":{"type":"model_not_entitled","message":"model claude-4-opus is not entitled"}}`)},
		"conn-2|claude-4-opus": {Status: 200, Body: "data: [DONE]\n\n"},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("openai-compatible", exec)

	svc := NewChatService(store, registry)

	result := svc.HandleChat([]byte(`{"model":"openai-compatible/claude-4-opus","messages":[]}`), "openai-compatible/claude-4-opus", "req-entitlement", nil)
	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected fallback success, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()

	conn1, _ := store.GetConnectionByID("conn-1")
	if conn1 == nil {
		t.Fatal("conn-1 missing")
	}
	if conn1.RateLimitedUntil != "" {
		t.Fatalf("model entitlement should not cooldown whole connection, got rateLimitedUntil=%q", conn1.RateLimitedUntil)
	}
	if !domain.IsModelLockActive(conn1.ModelLocks, "claude-4-opus") {
		t.Fatalf("expected claude-4-opus model lock, got %#v", conn1.ModelLocks)
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

	result := svc.HandleChat([]byte(`{"model":"kiro/model-b","messages":[]}`), "kiro/model-b", "req-4", nil)
	if result.StatusCode != 400 {
		t.Fatalf("expected 400 for unsupported model, got %d (%s)", result.StatusCode, result.Error)
	}
	if !strings.Contains(result.Error, "No connections available") && !strings.Contains(result.Error, "support model") {
		t.Fatalf("expected unsupported/unavailable model message, got %q", result.Error)
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

	result := svc.HandleChat([]byte(`{"model":"sonnet","messages":[{"role":"user","content":"hi"}]}`), "sonnet", "req-5", nil)
	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected alias resolution success, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()

	if len(exec.calls) != 1 || exec.calls[0].Model != "claude-sonnet" {
		t.Fatalf("expected execution with resolved model 'claude-sonnet', calls=%+v", exec.calls)
	}
}

func TestHandleChat_ModelPolicyFiltersComboMembers(t *testing.T) {
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
		"conn-1|model-b": {Status: 200, Body: "data: [DONE]\n\n"},
	})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)
	result := svc.HandleChat(
		[]byte(`{"model":"coder-combo","messages":[]}`),
		"coder-combo",
		"req-policy-1",
		&port.APIKeyPolicy{AllowedModels: []string{"kiro/model-b"}},
	)

	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected filtered combo success, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()
	if len(exec.calls) != 1 || exec.calls[0].Model != "model-b" {
		t.Fatalf("expected only allowed model-b execution, calls=%+v", exec.calls)
	}
}

func TestHandleChat_ModelPolicyRejectsComboWithNoAllowedMembers(t *testing.T) {
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
	})
	exec := newFakeExecutor(map[string]fakeExecuteResponse{})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)
	result := svc.HandleChat(
		[]byte(`{"model":"coder-combo","messages":[]}`),
		"coder-combo",
		"req-policy-2",
		&port.APIKeyPolicy{AllowedModels: []string{"kiro/model-c"}},
	)

	if result.StatusCode != 403 {
		t.Fatalf("expected 403, got %d (%s)", result.StatusCode, result.Error)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("executor must not be called for forbidden combo, calls=%+v", exec.calls)
	}
}

func TestHandleChat_ModelPolicyAllowsDirectAliasByAliasName(t *testing.T) {
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
	result := svc.HandleChat(
		[]byte(`{"model":"sonnet","messages":[]}`),
		"sonnet",
		"req-policy-3",
		&port.APIKeyPolicy{AllowedModels: []string{"sonnet"}},
	)

	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected alias policy success, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()
}

func TestHandleChat_ConnectionPolicySkipsUnavailableComboMember(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-kiro", Name: "kiro", Provider: "kiro", Weight: 100, IsActive: true},
			{ID: "conn-openai", Name: "openai", Provider: "openai", Weight: 100, IsActive: true},
		},
		Combos: []domain.Combo{{
			Name:   "mixed-combo",
			Models: []string{"kiro/model-a", "openai/model-b"},
		}},
	})
	kiroExec := newFakeExecutor(map[string]fakeExecuteResponse{})
	openaiExec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-openai|model-b": {Status: 200, Body: "data: [DONE]\n\n"},
	})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", kiroExec)
	registry.RegisterExecutor("openai", openaiExec)

	svc := NewChatService(store, registry)
	result := svc.HandleChat(
		[]byte(`{"model":"mixed-combo","messages":[]}`),
		"mixed-combo",
		"req-policy-4",
		&port.APIKeyPolicy{AllowedConnectionIDs: []string{"conn-openai"}},
	)

	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected fallback to allowed OpenAI connection, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()
	if len(kiroExec.calls) != 0 {
		t.Fatalf("kiro executor should not run without allowed connection, calls=%+v", kiroExec.calls)
	}
	if len(openaiExec.calls) != 1 || openaiExec.calls[0].ConnectionID != "conn-openai" {
		t.Fatalf("expected openai execution on conn-openai, calls=%+v", openaiExec.calls)
	}
}

func TestHandleChat_DisallowedPinnedConnectionFallsThrough(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-primary", Name: "primary", Provider: "kiro", Weight: 100, IsActive: true},
			{ID: "conn-backup", Name: "backup", Provider: "kiro", Weight: 100, IsActive: true},
		},
		Combos: []domain.Combo{{
			Name:   "pinned-combo",
			Models: []string{"kiro/model-a@conn-primary", "kiro/model-b"},
		}},
	})
	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-backup|model-b": {Status: 200, Body: "data: [DONE]\n\n"},
	})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)
	result := svc.HandleChat(
		[]byte(`{"model":"pinned-combo","messages":[]}`),
		"pinned-combo",
		"req-policy-5",
		&port.APIKeyPolicy{AllowedConnectionIDs: []string{"conn-backup"}},
	)

	// Disallowed pinned member is filtered out at route planning; combo falls through to model-b.
	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected fallback to model-b on conn-backup, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()
	if len(exec.calls) != 1 || exec.calls[0].Model != "model-b" {
		t.Fatalf("expected only model-b execution, calls=%+v", exec.calls)
	}
}

func TestHandleChat_ConnectionStrategyPriorityFallback(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-backup", Name: "backup", Provider: "kiro", Priority: 20, Weight: 100, IsActive: true},
			{ID: "conn-primary", Name: "primary", Provider: "kiro", Priority: 1, Weight: 1, IsActive: true},
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyPriorityFallback},
	})
	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-primary|model-a": {Status: 200, Body: "data: [DONE]\n\n"},
	})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)
	result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-priority", nil)

	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected priority fallback success, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()
	if len(exec.calls) != 1 || exec.calls[0].ConnectionID != "conn-primary" {
		t.Fatalf("expected primary priority connection, calls=%+v", exec.calls)
	}
}

func TestHandleChat_ConnectionStrategyRoundRobin(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-a", Name: "a", Provider: "kiro", Priority: 0, Weight: 100, IsActive: true},
			{ID: "conn-b", Name: "b", Provider: "kiro", Priority: 0, Weight: 100, IsActive: true},
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyRoundRobin},
	})
	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-a|model-a": {Status: 200, Body: "data: [DONE]\n\n"},
		"conn-b|model-a": {Status: 200, Body: "data: [DONE]\n\n"},
	})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)
	for i := 0; i < 2; i++ {
		result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-rr", nil)
		if result.StatusCode != 200 || result.Stream == nil {
			t.Fatalf("request %d expected round-robin success, got status=%d err=%q", i+1, result.StatusCode, result.Error)
		}
		result.Stream.Close()
	}

	if len(exec.calls) != 2 {
		t.Fatalf("expected two calls, got %+v", exec.calls)
	}
	if exec.calls[0].ConnectionID != "conn-a" || exec.calls[1].ConnectionID != "conn-b" {
		t.Fatalf("expected conn-a then conn-b, calls=%+v", exec.calls)
	}
}

func TestHandleChat_OpenAICompatibleRoutePrefixUsesPinnedCredentials(t *testing.T) {
	cfg := domain.DefaultConfig()
	cfg.ProviderConnections = []domain.ProviderConnection{
		{ID: "conn-windsurf", Provider: "openai-compatible", Name: "Windsurf", RoutePrefix: "windsurf", IsActive: true, APIKey: "ws-key", Weight: 100, SupportedModels: []string{"RL-4m"}},
		{ID: "conn-other", Provider: "openai-compatible", Name: "Other", RoutePrefix: "other", IsActive: true, APIKey: "other-key", Weight: 100, SupportedModels: []string{"RL-4m"}},
	}
	store := newTestCredentialStore(&cfg)
	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-windsurf|RL-4m": {Status: 200, Body: "data: [DONE]\n\n"},
	})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("openai-compatible", exec)

	svc := NewChatService(store, registry)
	result := svc.HandleChat([]byte(`{"model":"windsurf/RL-4m","messages":[]}`), "windsurf/RL-4m", "req-custom", nil)
	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected success, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()

	if len(exec.calls) != 1 || exec.calls[0].ConnectionID != "conn-windsurf" {
		t.Fatalf("expected windsurf connection, calls=%+v", exec.calls)
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
