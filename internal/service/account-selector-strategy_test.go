package service

import (
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

func fillFirstTestConn(id string, priority int) domain.ProviderConnection {
	return domain.ProviderConnection{
		ID:              id,
		Provider:        "openai",
		Name:            id,
		IsActive:        true,
		AuthType:        "api-key",
		Priority:        priority,
		Weight:          100,
		SupportedModels: []string{"gpt"},
	}
}

func TestFillFirstSelectsLowestPriority(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			fillFirstTestConn("a", 2),
			fillFirstTestConn("b", 1),
			fillFirstTestConn("c", 3),
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})
	sel := NewAccountSelector(store)

	for i := 0; i < 20; i++ {
		creds, err := sel.SelectCredentials("openai", nil, "gpt", nil)
		if err != nil {
			t.Fatalf("iter %d: SelectCredentials: %v", i, err)
		}
		if creds.ConnectionID != "b" {
			t.Fatalf("iter %d: want connection b (lowest priority), got %q", i, creds.ConnectionID)
		}
	}
}

func TestFillFirstSkipsExcludedAndCooldown(t *testing.T) {
	cooldownUntil := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			fillFirstTestConn("a", 2),
			fillFirstTestConn("b", 1),
			fillFirstTestConn("c", 3),
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})
	// Put b on cooldown so it is unavailable.
	store.cfg.ProviderConnections[1].RateLimitedUntil = cooldownUntil

	sel := NewAccountSelector(store)

	// Exclude is empty; b is cooling → expect a (priority 2).
	creds, err := sel.SelectCredentials("openai", nil, "gpt", nil)
	if err != nil {
		t.Fatalf("SelectCredentials (cooldown): %v", err)
	}
	if creds.ConnectionID != "a" {
		t.Fatalf("want a after b cooldown, got %q", creds.ConnectionID)
	}

	// Clear cooldown; exclude b explicitly → still expect a.
	store.cfg.ProviderConnections[1].RateLimitedUntil = ""
	creds, err = sel.SelectCredentials("openai", map[string]bool{"b": true}, "gpt", nil)
	if err != nil {
		t.Fatalf("SelectCredentials (exclude): %v", err)
	}
	if creds.ConnectionID != "a" {
		t.Fatalf("want a after excluding b, got %q", creds.ConnectionID)
	}
}
