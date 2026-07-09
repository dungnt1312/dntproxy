package service

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestShouldStopCredentialRetry(t *testing.T) {
	if shouldStopCredentialRetry(1, 0) {
		t.Fatal("0 means unlimited")
	}
	if shouldStopCredentialRetry(1, 2) {
		t.Fatal("should continue")
	}
	if !shouldStopCredentialRetry(2, 2) {
		t.Fatal("should stop at max")
	}
	if shouldStopCredentialRetry(0, 1) {
		t.Fatal("zero attempts should not stop")
	}
	if !shouldStopCredentialRetry(5, 3) {
		t.Fatal("should stop when attempted exceeds max")
	}
}

func TestExecuteOnProviderRespectsMaxRetryCredentials(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-a", Name: "a", Provider: "kiro", Weight: 100, IsActive: true, Priority: 1},
			{ID: "conn-b", Name: "b", Provider: "kiro", Weight: 100, IsActive: true, Priority: 2},
			{ID: "conn-c", Name: "c", Provider: "kiro", Weight: 100, IsActive: true, Priority: 3},
		},
		Settings: domain.Settings{
			MaxRetryCredentials: 2,
			ConnectionStrategy:  ConnectionStrategyPriorityFallback,
		},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-a|model-a": {Status: 503, Err: errors.New("service unavailable")},
		"conn-b|model-a": {Status: 503, Err: errors.New("service unavailable")},
		"conn-c|model-a": {Status: 503, Err: errors.New("service unavailable")},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)

	result, err := svc.executeOnProvider(
		[]byte(`{"model":"kiro/model-a","messages":[]}`),
		"kiro/model-a",
		"req-budget-2",
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.OK {
		t.Fatal("expected failure when budget exhausted")
	}
	if !result.AllowFallback {
		t.Fatal("budget exhausted must set AllowFallback=true")
	}
	if result.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d (%s)", result.StatusCode, result.Error)
	}
	if !strings.Contains(result.Error, "credential retry budget exhausted") {
		t.Fatalf("expected budget exhausted message, got %q", result.Error)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("expected exactly 2 Execute calls, got %d: %+v", len(exec.calls), exec.calls)
	}
	for _, call := range exec.calls {
		if call.ConnectionID == "conn-c" {
			t.Fatalf("conn-c must not be selected when max=2, calls=%+v", exec.calls)
		}
	}
}

func TestExecuteOnProviderMaxRetryCredentialsOne(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-a", Name: "a", Provider: "kiro", Weight: 10000, IsActive: true},
			{ID: "conn-b", Name: "b", Provider: "kiro", Weight: 1, IsActive: true},
		},
		Settings: domain.Settings{MaxRetryCredentials: 1},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-a|model-a": {Status: 503, Err: errors.New("service unavailable")},
		"conn-b|model-a": {Status: 503, Err: errors.New("service unavailable")},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", exec)

	svc := NewChatService(store, registry)

	result, err := svc.executeOnProvider(
		[]byte(`{"model":"kiro/model-a","messages":[]}`),
		"kiro/model-a",
		"req-budget-1",
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.OK {
		t.Fatalf("expected budget failure, got %+v", result)
	}
	if !result.AllowFallback {
		t.Fatal("expected AllowFallback=true")
	}
	if len(exec.calls) != 1 {
		t.Fatalf("expected exactly 1 Execute with max=1, got %d: %+v", len(exec.calls), exec.calls)
	}
}
