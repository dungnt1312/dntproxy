package service

import (
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// tenantTestCredentialStore extends testCredentialStore with LoadForTenant
// so the tenant-aware code paths actually filter.
type tenantTestCredentialStore struct {
	*testCredentialStore
}

func (s *tenantTestCredentialStore) LoadForTenant(tenantID string) (*domain.AppConfig, error) {
	cfg, err := s.testCredentialStore.Load()
	if err != nil {
		return nil, err
	}
	return domain.FilterConfigByTenant(cfg, tenantID), nil
}

// Ensure it satisfies the tenant extension interface.
var _ port.CredentialStoreTenantExt = (*tenantTestCredentialStore)(nil)

// TestChatServiceRespectsTenantIsolation verifies that when a tenant is
// specified, the ChatService can only route to connections owned by that tenant.
func TestChatServiceRespectsTenantIsolation(t *testing.T) {
	cfg := &domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "acme-conn", Provider: "kiro", AuthType: "oauth", IsActive: true, Weight: 100, TenantID: "acme"},
			{ID: "globex-conn", Provider: "kiro", AuthType: "oauth", IsActive: true, Weight: 100, TenantID: "globex"},
		},
		Settings: domain.Settings{ComboStrategy: "fallback", ConnectionStrategy: "weighted-random"},
	}
	base := newTestCredentialStore(cfg)
	store := &tenantTestCredentialStore{testCredentialStore: base}

	executor := newFakeExecutor(map[string]fakeExecuteResponse{
		"acme-conn|model-a":   {Status: 200},
		"globex-conn|model-a": {Status: 200},
	})

	providers := newTestProviderRegistry()
	providers.RegisterExecutor("kiro", executor)

	svc := NewChatServiceWithDeps(
		NewModelResolver(store),
		NewAccountSelector(store),
		NewComboHandler(),
		providers,
		store,
	)

	// Request as acme — should only ever use acme-conn
	result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-acme", nil,
		port.RequestMetadata{TenantID: "acme"})
	if result.StatusCode != 200 {
		t.Fatalf("acme request failed: status=%d err=%s", result.StatusCode, result.Error)
	}
	for _, call := range executor.calls {
		if call.ConnectionID != "acme-conn" {
			t.Errorf("acme request routed to %s, want acme-conn only", call.ConnectionID)
		}
	}

	executor.calls = nil

	// Request as globex — should only ever use globex-conn
	result = svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-globex", nil,
		port.RequestMetadata{TenantID: "globex"})
	if result.StatusCode != 200 {
		t.Fatalf("globex request failed: status=%d err=%s", result.StatusCode, result.Error)
	}
	for _, call := range executor.calls {
		if call.ConnectionID != "globex-conn" {
			t.Errorf("globex request routed to %s, want globex-conn only", call.ConnectionID)
		}
	}
}

// TestChatServiceTenantWithoutConnectionsReturns404 confirms a tenant with no
// matching connections gets an explicit error rather than falling back to
// another tenant's resources.
func TestChatServiceTenantWithoutConnectionsReturns404(t *testing.T) {
	cfg := &domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "acme-conn", Provider: "kiro", AuthType: "oauth", IsActive: true, Weight: 100, TenantID: "acme"},
		},
		Settings: domain.Settings{ComboStrategy: "fallback"},
	}
	base := newTestCredentialStore(cfg)
	store := &tenantTestCredentialStore{testCredentialStore: base}

	executor := newFakeExecutor(map[string]fakeExecuteResponse{})
	providers := newTestProviderRegistry()
	providers.RegisterExecutor("kiro", executor)

	svc := NewChatServiceWithDeps(
		NewModelResolver(store),
		NewAccountSelector(store),
		NewComboHandler(),
		providers,
		store,
	)

	// globex has no connections — should fail, NOT use acme's connection.
	result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-orphan", nil,
		port.RequestMetadata{TenantID: "globex"})

	if result.StatusCode == 200 {
		t.Fatal("orphan tenant request succeeded — isolation broken")
	}
	if len(executor.calls) != 0 {
		t.Errorf("orphan tenant triggered %d executor calls, want 0", len(executor.calls))
	}
}

// TestChatServiceLegacyModeSeesAllConnections confirms the legacy (empty tenant)
// path retains the original single-tenant behavior of using all connections.
func TestChatServiceLegacyModeSeesAllConnections(t *testing.T) {
	cfg := &domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "c1", Provider: "kiro", AuthType: "oauth", IsActive: true, Weight: 100, TenantID: "acme"},
			{ID: "c2", Provider: "kiro", AuthType: "oauth", IsActive: true, Weight: 100, TenantID: "globex"},
		},
		Settings: domain.Settings{ComboStrategy: "fallback"},
	}
	base := newTestCredentialStore(cfg)
	store := &tenantTestCredentialStore{testCredentialStore: base}

	executor := newFakeExecutor(map[string]fakeExecuteResponse{
		"c1|model-a": {Status: 200},
		"c2|model-a": {Status: 200},
	})

	providers := newTestProviderRegistry()
	providers.RegisterExecutor("kiro", executor)

	svc := NewChatServiceWithDeps(
		NewModelResolver(store),
		NewAccountSelector(store),
		NewComboHandler(),
		providers,
		store,
	)

	// Legacy request (no tenant) — should succeed and use the pool.
	result := svc.HandleChat([]byte(`{"model":"kiro/model-a","messages":[]}`), "kiro/model-a", "req-legacy", nil)
	if result.StatusCode != 200 {
		t.Fatalf("legacy request failed: status=%d err=%s", result.StatusCode, result.Error)
	}
}

// TestModelAccessServiceTenantComboIsolation verifies combos are isolated.
func TestModelAccessServiceTenantComboIsolation(t *testing.T) {
	cfg := &domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "c1", Provider: "kiro", AuthType: "oauth", IsActive: true, Weight: 100, TenantID: "acme"},
		},
		Combos: []domain.Combo{
			{Name: "acme-combo", Models: []string{"kiro/model-a"}, TenantID: "acme"},
			{Name: "globex-combo", Models: []string{"kiro/model-a"}, TenantID: "globex"},
		},
		Settings: domain.Settings{ComboStrategy: "fallback"},
	}
	base := newTestCredentialStore(cfg)
	store := &tenantTestCredentialStore{testCredentialStore: base}

	mas := NewModelAccessService(store)

	// acme can resolve its own combo
	plan, err := mas.ResolveRouteForTenant("acme-combo", nil, "acme")
	if err != nil {
		t.Fatalf("ResolveRouteForTenant(acme-combo, acme) error = %v", err)
	}
	if !plan.IsCombo || plan.ComboName != "acme-combo" {
		t.Errorf("acme expected acme-combo, got isCombo=%v name=%q", plan.IsCombo, plan.ComboName)
	}

	// acme CANNOT resolve globex's combo
	plan, err = mas.ResolveRouteForTenant("globex-combo", nil, "acme")
	if err == nil && plan != nil && plan.IsCombo {
		t.Errorf("acme resolved globex-combo — isolation broken (plan=%+v)", plan)
	}

	// Legacy mode can resolve both
	plan, err = mas.ResolveRouteForTenant("globex-combo", nil, "")
	if err != nil {
		t.Fatalf("legacy globex-combo error = %v", err)
	}
	if !plan.IsCombo {
		t.Errorf("legacy expected to resolve globex-combo")
	}
}
