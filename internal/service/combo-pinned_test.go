package service

import (
	"fmt"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

// TestHandleChat_PinnedConnection verifies that pinned connections are used correctly
func TestHandleChat_PinnedConnection(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{
				ID:       "conn-primary",
				Provider: "kiro",
				Name:     "Primary",
				IsActive: true,
				Weight:   100,
			},
			{
				ID:       "conn-backup",
				Provider: "kiro",
				Name:     "Backup",
				IsActive: true,
				Weight:   50,
			},
		},
		Combos: []domain.Combo{
			{
				Name: "pinned-combo",
				Models: []string{
					"kr/model-a@conn-primary",
					"kr/model-b@conn-backup",
				},
			},
		},
	})

	executor := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-primary|model-a": {Status: 503, Err: fmt.Errorf("server error")},
		"conn-backup|model-b":  {Status: 200, Body: "data: [DONE]\n\n"},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", executor)
	service := NewChatService(store, registry)

	result := service.HandleChat([]byte(`{"messages":[]}`), "pinned-combo", "req-1", nil)

	if result.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d: %s", result.StatusCode, result.Error)
	}

	// Verify execution order: should use pinned connections
	if len(executor.calls) != 2 {
		t.Errorf("Expected 2 executions, got %d", len(executor.calls))
	}
	if executor.calls[0].ConnectionID != "conn-primary" || executor.calls[0].Model != "model-a" {
		t.Errorf("Expected first execution on conn-primary:model-a, got %s:%s",
			executor.calls[0].ConnectionID, executor.calls[0].Model)
	}
	if executor.calls[1].ConnectionID != "conn-backup" || executor.calls[1].Model != "model-b" {
		t.Errorf("Expected second execution on conn-backup:model-b, got %s:%s",
			executor.calls[1].ConnectionID, executor.calls[1].Model)
	}
}

// TestHandleChat_PinnedConnectionUnavailable verifies that unavailable pinned connections fail immediately
func TestHandleChat_PinnedConnectionUnavailable(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{
				ID:               "conn-primary",
				Provider:         "kiro",
				Name:             "Primary",
				IsActive:         true,
				RateLimitedUntil: "2099-01-01T00:00:00Z", // Rate limited
			},
			{
				ID:       "conn-backup",
				Provider: "kiro",
				Name:     "Backup",
				IsActive: true,
			},
		},
		Combos: []domain.Combo{
			{
				Name: "pinned-unavailable",
				Models: []string{
					"kr/model-a@conn-primary", // Rate limited
					"kr/model-b",              // Auto-select (should use backup)
				},
			},
		},
	})

	executor := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-backup|model-b": {Status: 200, Body: "data: [DONE]\n\n"},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", executor)
	service := NewChatService(store, registry)

	result := service.HandleChat([]byte(`{"messages":[]}`), "pinned-unavailable", "req-1", nil)

	if result.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d: %s", result.StatusCode, result.Error)
	}

	// Should skip rate-limited pinned connection and use auto-select for second model
	if len(executor.calls) != 1 {
		t.Errorf("Expected 1 execution, got %d", len(executor.calls))
	}
	if executor.calls[0].ConnectionID != "conn-backup" || executor.calls[0].Model != "model-b" {
		t.Errorf("Expected execution on conn-backup:model-b, got %s:%s",
			executor.calls[0].ConnectionID, executor.calls[0].Model)
	}
}

// TestHandleChat_MixedPinnedAndAuto verifies mixed pinned and auto-select strategy
func TestHandleChat_MixedPinnedAndAuto(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{
				ID:       "conn-1",
				Provider: "kiro",
				Name:     "Kiro 1",
				IsActive: true,
				Weight:   100,
			},
			{
				ID:       "conn-2",
				Provider: "kiro",
				Name:     "Kiro 2",
				IsActive: true,
				Weight:   50,
			},
		},
		Combos: []domain.Combo{
			{
				Name: "mixed-combo",
				Models: []string{
					"kr/model-a@conn-1", // Pinned
					"kr/model-b",        // Auto-select
				},
			},
		},
	})

	executor := newFakeExecutor(map[string]fakeExecuteResponse{
		"conn-1|model-a": {Status: 503, Err: fmt.Errorf("server error")},
		"conn-1|model-b": {Status: 200, Body: "data: [DONE]\n\n"},
		"conn-2|model-b": {Status: 200, Body: "data: [DONE]\n\n"},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("kiro", executor)
	service := NewChatService(store, registry)

	result := service.HandleChat([]byte(`{"messages":[]}`), "mixed-combo", "req-1", nil)

	if result.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d: %s", result.StatusCode, result.Error)
	}

	// Should try pinned first, then auto-select
	if len(executor.calls) < 2 {
		t.Errorf("Expected at least 2 executions, got %d", len(executor.calls))
	}
	if executor.calls[0].ConnectionID != "conn-1" || executor.calls[0].Model != "model-a" {
		t.Errorf("Expected first execution on conn-1:model-a (pinned), got %s:%s",
			executor.calls[0].ConnectionID, executor.calls[0].Model)
	}
	// Second execution should be on any available connection (conn-1 or conn-2)
	secondCall := executor.calls[1]
	if secondCall.Model != "model-b" {
		t.Errorf("Expected second execution for model-b, got %s", secondCall.Model)
	}
	if secondCall.ConnectionID != "conn-1" && secondCall.ConnectionID != "conn-2" {
		t.Errorf("Expected second execution on conn-1 or conn-2, got %s", secondCall.ConnectionID)
	}
}

// TestParseModelString_EdgeCases tests edge cases in model string parsing
func TestParseModelString_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantConnID string
		wantErr    bool
	}{
		{
			name:       "multiple @ symbols - takes first",
			input:      "kr/model@conn-1@extra",
			wantConnID: "conn-1@extra", // Split only on first @, rest is part of connID
			wantErr:    false,
		},
		{
			name:       "empty connection ID",
			input:      "kr/model@",
			wantConnID: "",
			wantErr:    false,
		},
		{
			name:       "connection ID with special chars",
			input:      "kr/model@conn-abc-123_xyz",
			wantConnID: "conn-abc-123_xyz",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseModelString(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("Expected error, got nil")
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if !tt.wantErr && parsed.ConnectionID != tt.wantConnID {
				t.Errorf("Expected connectionID %q, got %q", tt.wantConnID, parsed.ConnectionID)
			}
		})
	}
}
