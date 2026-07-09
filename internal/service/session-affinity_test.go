package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

func TestSessionAffinityStoreTTL(t *testing.T) {
	s := NewSessionAffinityStore()
	s.Put("k", "c1", 40*time.Millisecond)
	if id, ok := s.Get("k"); !ok || id != "c1" {
		t.Fatalf("got %s %v", id, ok)
	}
	time.Sleep(50 * time.Millisecond)
	if _, ok := s.Get("k"); ok {
		t.Fatal("expected expire")
	}
}

func TestSessionAffinityStoreDelete(t *testing.T) {
	s := NewSessionAffinityStore()
	s.Put("k", "c1", time.Minute)
	s.Delete("k")
	if _, ok := s.Get("k"); ok {
		t.Fatal("expected deleted")
	}
}

func TestSessionAffinityStoreEmptyKeyNoop(t *testing.T) {
	s := NewSessionAffinityStore()
	s.Put("", "c1", time.Minute)
	if _, ok := s.Get(""); ok {
		t.Fatal("empty key must not store")
	}
	s.Delete("") // must not panic
}

func TestAffinityKeyPrecedence(t *testing.T) {
	// header present wins
	if got := AffinityKey("ak", "openai", "gpt", "sess"); got != "h:sess" {
		t.Fatalf("header key = %q, want h:sess", got)
	}
	// api key path when no header
	if got := AffinityKey("ak", "openai", "gpt", ""); got != "k:ak|openai|gpt" {
		t.Fatalf("api key key = %q, want k:ak|openai|gpt", got)
	}
	// empty when neither
	if got := AffinityKey("", "openai", "gpt", ""); got != "" {
		t.Fatalf("empty key = %q, want \"\"", got)
	}
	// header still wins over api key
	if got := AffinityKey("ak", "p", "m", "s1"); !strings.HasPrefix(got, "h:") {
		t.Fatalf("expected h: prefix, got %q", got)
	}
}

func TestExecuteOnProvider_SessionAffinitySoftPin(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-a", Name: "a", Provider: "kiro", Weight: 1, IsActive: true, Priority: 2},
			{ID: "conn-b", Name: "b", Provider: "kiro", Weight: 10000, IsActive: true, Priority: 1},
		},
		Settings: domain.Settings{
			SessionAffinityEnabled:    true,
			SessionAffinityTTLSeconds: 60,
			ConnectionStrategy:        ConnectionStrategyPriorityFallback,
		},
	})

	// First request succeeds on priority-fallback (conn-b has lower priority number = preferred).
	// Second request with same session should soft-pin to whatever first used.
	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-a|model-a": {Status: 200, Body: "data: {\"id\":\"a\"}\n\ndata: [DONE]\n\n"},
		"conn-b|model-a": {Status: 200, Body: "data: {\"id\":\"b\"}\n\ndata: [DONE]\n\n"},
	})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)
	svc := NewChatService(store, registry)

	meta := port.RequestMetadata{SessionKey: "sess-1"}
	r1, err := svc.executeOnProvider([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-aff-1", nil, meta)
	if err != nil || r1 == nil || !r1.OK {
		t.Fatalf("first request failed: err=%v result=%+v", err, r1)
	}
	if r1.Stream != nil {
		_ = r1.Stream.Close()
	}
	if len(exec.calls) != 1 {
		t.Fatalf("expected 1 call after first request, got %d", len(exec.calls))
	}
	firstConn := exec.calls[0].ConnectionID

	r2, err := svc.executeOnProvider([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-aff-2", nil, meta)
	if err != nil || r2 == nil || !r2.OK {
		t.Fatalf("second request failed: err=%v result=%+v", err, r2)
	}
	if r2.Stream != nil {
		_ = r2.Stream.Close()
	}
	if len(exec.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(exec.calls))
	}
	if exec.calls[1].ConnectionID != firstConn {
		t.Fatalf("expected sticky connection %s, got %s", firstConn, exec.calls[1].ConnectionID)
	}
}

func TestExecuteOnProvider_SessionAffinityRetryableFailDeletes(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-a", Name: "a", Provider: "kiro", Weight: 100, IsActive: true, Priority: 1},
			{ID: "conn-b", Name: "b", Provider: "kiro", Weight: 100, IsActive: true, Priority: 2},
		},
		Settings: domain.Settings{
			SessionAffinityEnabled:    true,
			SessionAffinityTTLSeconds: 60,
			ConnectionStrategy:        ConnectionStrategyPriorityFallback,
		},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-a|model-a": {Status: 503, Err: errors.New("service unavailable")},
		"conn-b|model-a": {Status: 200, Body: "data: {\"id\":\"b\"}\n\ndata: [DONE]\n\n"},
	})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)
	svc := NewChatService(store, registry)

	// Seed sticky to conn-a
	key := AffinityKey("", "kiro", "model-a", "sess-fail")
	svc.sessionAffinity.Put(key, "conn-a", time.Minute)

	meta := port.RequestMetadata{SessionKey: "sess-fail"}
	r, err := svc.executeOnProvider([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-aff-fail", nil, meta)
	if err != nil || r == nil || !r.OK {
		t.Fatalf("expected fallback success: err=%v result=%+v", err, r)
	}
	if r.Stream != nil {
		_ = r.Stream.Close()
	}
	if len(exec.calls) < 2 {
		t.Fatalf("expected sticky fail then fallback, calls=%+v", exec.calls)
	}
	if exec.calls[0].ConnectionID != "conn-a" {
		t.Fatalf("first call should be sticky conn-a, got %s", exec.calls[0].ConnectionID)
	}
	if exec.calls[1].ConnectionID != "conn-b" {
		t.Fatalf("second call should be conn-b, got %s", exec.calls[1].ConnectionID)
	}
	if _, ok := svc.sessionAffinity.Get(key); ok {
		// After success on conn-b, affinity should be rewritten to conn-b (Put on success).
		// Delete happened on sticky fail; Put on success with conn-b.
	}
	if id, ok := svc.sessionAffinity.Get(key); !ok || id != "conn-b" {
		t.Fatalf("affinity after success should be conn-b, got %s ok=%v", id, ok)
	}
}

func TestExecuteOnProvider_HardPinWinsOverAffinity(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-a", Name: "a", Provider: "kiro", Weight: 100, IsActive: true},
			{ID: "conn-b", Name: "b", Provider: "kiro", Weight: 100, IsActive: true},
		},
		Settings: domain.Settings{
			SessionAffinityEnabled:    true,
			SessionAffinityTTLSeconds: 60,
		},
	})
	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-a|model-a": {Status: 200, Body: "data: {\"id\":\"a\"}\n\ndata: [DONE]\n\n"},
		"conn-b|model-a": {Status: 200, Body: "data: {\"id\":\"b\"}\n\ndata: [DONE]\n\n"},
	})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)
	svc := NewChatService(store, registry)

	// Seed sticky to conn-a, but hard-pin to conn-b
	key := AffinityKey("", "kiro", "model-a", "sess-hard")
	svc.sessionAffinity.Put(key, "conn-a", time.Minute)

	meta := port.RequestMetadata{SessionKey: "sess-hard"}
	r, err := svc.executeOnProvider([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a@conn-b", "req-hard", nil, meta)
	if err != nil || r == nil || !r.OK {
		t.Fatalf("hard pin failed: err=%v result=%+v", err, r)
	}
	if r.Stream != nil {
		_ = r.Stream.Close()
	}
	if len(exec.calls) != 1 || exec.calls[0].ConnectionID != "conn-b" {
		t.Fatalf("expected hard pin conn-b only, calls=%+v", exec.calls)
	}
	// Affinity must not be rewritten by hard pin
	if id, ok := svc.sessionAffinity.Get(key); !ok || id != "conn-a" {
		t.Fatalf("hard pin must not rewrite affinity, got %s ok=%v", id, ok)
	}
}

func TestExecuteOnProvider_StickyNotInAllowlistSkipped(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-a", Name: "a", Provider: "kiro", Weight: 100, IsActive: true, Priority: 1},
			{ID: "conn-b", Name: "b", Provider: "kiro", Weight: 100, IsActive: true, Priority: 2},
		},
		Settings: domain.Settings{
			SessionAffinityEnabled:    true,
			SessionAffinityTTLSeconds: 60,
			ConnectionStrategy:        ConnectionStrategyPriorityFallback,
		},
	})
	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-a|model-a": {Status: 200, Body: "data: {\"id\":\"a\"}\n\ndata: [DONE]\n\n"},
		"conn-b|model-a": {Status: 200, Body: "data: {\"id\":\"b\"}\n\ndata: [DONE]\n\n"},
	})
	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)
	svc := NewChatService(store, registry)

	key := AffinityKey("ak1", "kiro", "model-a", "")
	svc.sessionAffinity.Put(key, "conn-a", time.Minute)

	// Allow only conn-b; sticky conn-a must be skipped
	meta := port.RequestMetadata{APIKeyID: "ak1"}
	r, err := svc.executeOnProvider([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-allow", []string{"conn-b"}, meta)
	if err != nil || r == nil || !r.OK {
		t.Fatalf("request failed: err=%v result=%+v", err, r)
	}
	if r.Stream != nil {
		_ = r.Stream.Close()
	}
	if len(exec.calls) != 1 || exec.calls[0].ConnectionID != "conn-b" {
		t.Fatalf("expected only conn-b, calls=%+v", exec.calls)
	}
}
