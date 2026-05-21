package service

import (
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// testStore implements port.CredentialStore for testing.
type testStore struct {
	cfg *domain.AppConfig
}

func (s *testStore) Load() (*domain.AppConfig, error)            { return s.cfg, nil }
func (s *testStore) Save(_ *domain.AppConfig) error              { return nil }
func (s *testStore) Update(fn func(cfg *domain.AppConfig)) error { fn(s.cfg); return nil }
func (s *testStore) GetActiveConnections(_ string) ([]domain.ProviderConnection, error) {
	return nil, nil
}
func (s *testStore) GetConnectionByID(id string) (*domain.ProviderConnection, error) {
	for i := range s.cfg.ProviderConnections {
		if s.cfg.ProviderConnections[i].ID == id {
			return &s.cfg.ProviderConnections[i], nil
		}
	}
	return nil, nil
}
func (s *testStore) UpdateConnection(_ *domain.ProviderConnection) error { return nil }
func (s *testStore) GetCombos() ([]domain.Combo, error)                  { return s.cfg.Combos, nil }
func (s *testStore) GetComboByName(name string) (*domain.Combo, error) {
	for i := range s.cfg.Combos {
		if s.cfg.Combos[i].Name == name {
			return &s.cfg.Combos[i], nil
		}
	}
	return nil, nil
}
func (s *testStore) GetModelAliases() (domain.AliasMap, error)        { return s.cfg.ModelAliases, nil }
func (s *testStore) GetAPIKeys() ([]domain.APIKey, error)             { return nil, nil }
func (s *testStore) ValidateAPIKey(_ string) bool                     { return true }
func (s *testStore) GetAPIKeyByValue(_ string) (*domain.APIKey, bool) { return nil, false }
func (s *testStore) GetSettings() (*domain.Settings, error)           { return &s.cfg.Settings, nil }
func (s *testStore) GetModelRegistry() (*domain.ModelRegistry, error) {
	return s.cfg.ModelRegistry, nil
}
func (s *testStore) GetConnectionIDsForCombo(name string) ([]string, error) {
	for _, c := range s.cfg.Combos {
		if c.Name == name {
			return c.ConnectionIDs, nil
		}
	}
	return nil, nil
}

func newTestConfig() *domain.AppConfig {
	return &domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "conn-kiro-1", Provider: "kiro", IsActive: true, SupportedModels: []string{"claude-sonnet-4.5", "claude-opus-4.6"}},
			{ID: "conn-kiro-2", Provider: "kiro", IsActive: true, SupportedModels: []string{"claude-sonnet-4.5"}},
			{ID: "conn-glm-1", Provider: "glm", IsActive: true, SupportedModels: []string{"glm-5.1"}},
			{ID: "conn-oai-1", Provider: "openai", IsActive: true, SupportedModels: []string{"gpt-4", "gpt-4o"}},
			{ID: "conn-inactive", Provider: "kiro", IsActive: false, SupportedModels: []string{"claude-sonnet-4.5"}},
		},
		Combos: []domain.Combo{
			{Name: "my-combo", Models: []string{"kr/claude-sonnet-4.5", "glm/glm-5.1"}},
			{Name: "kiro-only", Models: []string{"kr/claude-sonnet-4.5", "kr/claude-opus-4.6"}},
			{Name: "pinned-combo", Models: []string{"kr/claude-sonnet-4.5@conn-kiro-1", "glm/glm-5.1"}},
		},
		ModelAliases: domain.AliasMap{
			"sonnet": "kr/claude-sonnet-4.5",
			"glm":    "glm/glm-5.1",
		},
		Settings: domain.Settings{ComboStrategy: "fallback"},
	}
}

func TestBuildPool_Unrestricted(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	pool, err := svc.BuildPool(nil)
	if err != nil {
		t.Fatalf("BuildPool: %v", err)
	}

	// Should see all active connection models
	modelIDs := modelRefIDs(pool.Models)
	assertContains(t, modelIDs, "kiro/claude-sonnet-4.5")
	assertContains(t, modelIDs, "kiro/claude-opus-4.6")
	assertContains(t, modelIDs, "glm/glm-5.1")
	assertContains(t, modelIDs, "openai/gpt-4")
	assertContains(t, modelIDs, "openai/gpt-4o")

	// Should see all combos
	comboNames := comboRefNames(pool.Combos)
	assertContains(t, comboNames, "my-combo")
	assertContains(t, comboNames, "kiro-only")

	// Should see both aliases
	aliasNames := aliasRefNames(pool.Aliases)
	assertContains(t, aliasNames, "sonnet")
	assertContains(t, aliasNames, "glm")
}

func TestBuildPool_ConnectionRestricted(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	policy := &port.APIKeyPolicy{AllowedConnectionIDs: []string{"conn-glm-1"}}
	pool, err := svc.BuildPool(policy)
	if err != nil {
		t.Fatalf("BuildPool: %v", err)
	}

	modelIDs := modelRefIDs(pool.Models)
	if len(modelIDs) != 1 || modelIDs[0] != "glm/glm-5.1" {
		t.Errorf("expected only glm/glm-5.1, got %v", modelIDs)
	}

	// "sonnet" alias targets kiro model which has no allowed connection
	aliasNames := aliasRefNames(pool.Aliases)
	assertNotContains(t, aliasNames, "sonnet")
	// "glm" alias targets glm/glm-5.1 which is in pool
	assertContains(t, aliasNames, "glm")

	// kiro-only combo should be invisible
	comboNames := comboRefNames(pool.Combos)
	assertNotContains(t, comboNames, "kiro-only")
	// my-combo has glm member → visible
	assertContains(t, comboNames, "my-combo")
}

func TestBuildPool_ModelRestricted(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	policy := &port.APIKeyPolicy{AllowedModels: []string{"kiro/claude-sonnet-4.5"}}
	pool, err := svc.BuildPool(policy)
	if err != nil {
		t.Fatalf("BuildPool: %v", err)
	}

	modelIDs := modelRefIDs(pool.Models)
	if len(modelIDs) != 1 || modelIDs[0] != "kiro/claude-sonnet-4.5" {
		t.Errorf("expected only kiro/claude-sonnet-4.5, got %v", modelIDs)
	}

	// "sonnet" alias allowed (targets allowed model)
	aliasNames := aliasRefNames(pool.Aliases)
	assertContains(t, aliasNames, "sonnet")
	// "glm" alias not allowed
	assertNotContains(t, aliasNames, "glm")
}

func TestBuildPool_CombinedRestriction(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	// Allow glm model but only kiro connections → no effective models
	policy := &port.APIKeyPolicy{
		AllowedConnectionIDs: []string{"conn-kiro-1"},
		AllowedModels:        []string{"glm/glm-5.1"},
	}
	pool, err := svc.BuildPool(policy)
	if err != nil {
		t.Fatalf("BuildPool: %v", err)
	}

	if len(pool.Models) != 0 {
		t.Errorf("expected empty model pool, got %v", modelRefIDs(pool.Models))
	}
}

func TestBuildPool_InactiveConnectionExcluded(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	pool, err := svc.BuildPool(nil)
	if err != nil {
		t.Fatalf("BuildPool: %v", err)
	}

	// conn-inactive should not contribute; verify kiro/claude-sonnet-4.5 has only 2 connections
	for _, m := range pool.Models {
		if m.QualifiedID == "kiro/claude-sonnet-4.5" {
			if len(m.ConnectionIDs) != 2 {
				t.Errorf("expected 2 connections for sonnet, got %d: %v", len(m.ConnectionIDs), m.ConnectionIDs)
			}
			return
		}
	}
	t.Error("kiro/claude-sonnet-4.5 not found in pool")
}

func TestBuildPool_ComboPartialAllowed(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	// Only allow kiro models → my-combo loses glm member but keeps kiro member
	policy := &port.APIKeyPolicy{AllowedModels: []string{"kiro/claude-sonnet-4.5"}}
	pool, err := svc.BuildPool(policy)
	if err != nil {
		t.Fatalf("BuildPool: %v", err)
	}

	for _, c := range pool.Combos {
		if c.Name == "my-combo" {
			if len(c.EffectiveModels) != 1 || c.EffectiveModels[0] != "kiro/claude-sonnet-4.5" {
				t.Errorf("expected my-combo with 1 effective model, got %v", c.EffectiveModels)
			}
			return
		}
	}
	t.Error("my-combo not found in pool")
}

func TestBuildPool_ComboNoAllowedMembers(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	// Only allow openai models → kiro-only combo hidden
	policy := &port.APIKeyPolicy{AllowedModels: []string{"openai/gpt-4"}}
	pool, err := svc.BuildPool(policy)
	if err != nil {
		t.Fatalf("BuildPool: %v", err)
	}

	comboNames := comboRefNames(pool.Combos)
	assertNotContains(t, comboNames, "kiro-only")
}

func TestResolveRoute_DirectModel(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	plan, err := svc.ResolveRoute("kr/claude-sonnet-4.5", nil)
	if err != nil {
		t.Fatalf("ResolveRoute: %v", err)
	}

	if plan.IsCombo {
		t.Error("expected not combo")
	}
	if len(plan.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(plan.Attempts))
	}
	a := plan.Attempts[0]
	if a.Provider != "kiro" || a.Model != "claude-sonnet-4.5" {
		t.Errorf("unexpected attempt: %+v", a)
	}
}

func TestResolveRoute_Combo(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	plan, err := svc.ResolveRoute("my-combo", nil)
	if err != nil {
		t.Fatalf("ResolveRoute: %v", err)
	}

	if !plan.IsCombo || plan.ComboName != "my-combo" {
		t.Error("expected combo")
	}
	if len(plan.Attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(plan.Attempts))
	}
}

func TestResolveRoute_ModelDenied(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	policy := &port.APIKeyPolicy{AllowedModels: []string{"openai/gpt-4"}}
	plan, err := svc.ResolveRoute("kr/claude-sonnet-4.5", policy)
	if err != nil {
		t.Fatalf("ResolveRoute: %v", err)
	}

	if len(plan.Attempts) != 0 {
		t.Errorf("expected 0 attempts for denied model, got %d", len(plan.Attempts))
	}
}

func TestResolveRoute_PinnedConnectionDenied(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	policy := &port.APIKeyPolicy{AllowedConnectionIDs: []string{"conn-kiro-2"}}
	plan, err := svc.ResolveRoute("kr/claude-sonnet-4.5@conn-kiro-1", policy)
	if err != nil {
		t.Fatalf("ResolveRoute: %v", err)
	}

	if len(plan.Attempts) != 0 {
		t.Errorf("expected 0 attempts for denied pinned connection, got %d", len(plan.Attempts))
	}
}

func TestResolveRoute_Alias(t *testing.T) {
	store := &testStore{cfg: newTestConfig()}
	svc := NewModelAccessService(store)

	plan, err := svc.ResolveRoute("sonnet", nil)
	if err != nil {
		t.Fatalf("ResolveRoute: %v", err)
	}

	if plan.IsCombo {
		t.Error("expected not combo for alias")
	}
	if len(plan.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(plan.Attempts))
	}
	if plan.Attempts[0].Provider != "kiro" {
		t.Errorf("expected kiro provider, got %q", plan.Attempts[0].Provider)
	}
}

func TestBuildPool_OpenAICompatibleUsesRoutePrefix(t *testing.T) {
	cfg := newTestConfig()
	cfg.ProviderConnections = append(cfg.ProviderConnections, domain.ProviderConnection{
		ID:              "conn-custom-1",
		Provider:        "openai-compatible",
		Name:            "Windsurf",
		RoutePrefix:     "windsurf",
		IsActive:        true,
		SupportedModels: []string{"RL-4m"},
	})
	store := &testStore{cfg: cfg}
	svc := NewModelAccessService(store)

	pool, err := svc.BuildPool(nil)
	if err != nil {
		t.Fatalf("BuildPool: %v", err)
	}

	assertContains(t, modelRefIDs(pool.Models), "windsurf/RL-4m")
	assertNotContains(t, modelRefIDs(pool.Models), "openai-compatible/RL-4m")
}

func TestResolveRoute_OpenAICompatibleRoutePrefixPinsConnection(t *testing.T) {
	cfg := newTestConfig()
	cfg.ProviderConnections = append(cfg.ProviderConnections,
		domain.ProviderConnection{ID: "conn-windsurf", Provider: "openai-compatible", Name: "Windsurf", RoutePrefix: "windsurf", IsActive: true, SupportedModels: []string{"RL-4m"}},
		domain.ProviderConnection{ID: "conn-other", Provider: "openai-compatible", Name: "Other", RoutePrefix: "other", IsActive: true, SupportedModels: []string{"RL-4m"}},
	)
	store := &testStore{cfg: cfg}
	svc := NewModelAccessService(store)

	plan, err := svc.ResolveRoute("windsurf/RL-4m", nil)
	if err != nil {
		t.Fatalf("ResolveRoute: %v", err)
	}
	if len(plan.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(plan.Attempts))
	}
	attempt := plan.Attempts[0]
	if attempt.QualifiedModel != "openai-compatible/RL-4m@conn-windsurf" {
		t.Fatalf("unexpected qualified model: %q", attempt.QualifiedModel)
	}
	if len(attempt.AllowedConnectionIDs) != 1 || attempt.AllowedConnectionIDs[0] != "conn-windsurf" {
		t.Fatalf("expected pinned windsurf connection, got %v", attempt.AllowedConnectionIDs)
	}
}

func TestResolveRoute_OpenAICompatibleRoutePrefixIgnoresMismatchedPin(t *testing.T) {
	cfg := newTestConfig()
	cfg.ProviderConnections = append(cfg.ProviderConnections,
		domain.ProviderConnection{ID: "conn-windsurf", Provider: "openai-compatible", Name: "Windsurf", RoutePrefix: "windsurf", IsActive: true, SupportedModels: []string{"RL-4m"}},
		domain.ProviderConnection{ID: "conn-other", Provider: "openai-compatible", Name: "Other", RoutePrefix: "other", IsActive: true, SupportedModels: []string{"RL-4m"}},
	)
	store := &testStore{cfg: cfg}
	svc := NewModelAccessService(store)

	plan, err := svc.ResolveRoute("windsurf/RL-4m@conn-other", nil)
	if err != nil {
		t.Fatalf("ResolveRoute: %v", err)
	}
	if len(plan.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(plan.Attempts))
	}
	attempt := plan.Attempts[0]
	if attempt.QualifiedModel != "openai-compatible/RL-4m@conn-windsurf" {
		t.Fatalf("route prefix must pin to owning connection, got %q", attempt.QualifiedModel)
	}
	if len(attempt.AllowedConnectionIDs) != 1 || attempt.AllowedConnectionIDs[0] != "conn-windsurf" {
		t.Fatalf("expected windsurf connection, got %v", attempt.AllowedConnectionIDs)
	}
}

func TestBuildPool_OpenAICompatibleAliasRespectsConnectionPolicy(t *testing.T) {
	cfg := newTestConfig()
	cfg.ProviderConnections = append(cfg.ProviderConnections,
		domain.ProviderConnection{ID: "conn-windsurf", Provider: "openai-compatible", Name: "Windsurf", RoutePrefix: "windsurf", IsActive: true, SupportedModels: []string{"RL-4m"}},
		domain.ProviderConnection{ID: "conn-other", Provider: "openai-compatible", Name: "Other", RoutePrefix: "other", IsActive: true, SupportedModels: []string{"RL-4m"}},
	)
	cfg.ModelAliases["legacy-custom"] = "openai-compatible/RL-4m"
	store := &testStore{cfg: cfg}
	svc := NewModelAccessService(store)

	pool, err := svc.BuildPool(&port.APIKeyPolicy{AllowedConnectionIDs: []string{"conn-other"}})
	if err != nil {
		t.Fatalf("BuildPool: %v", err)
	}

	var found *AliasRef
	for i := range pool.Aliases {
		if pool.Aliases[i].Name == "legacy-custom" {
			found = &pool.Aliases[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected legacy-custom alias to be visible")
	}
	if found.Target != "other/RL-4m" {
		t.Fatalf("expected alias to resolve to policy-allowed route prefix, got %q", found.Target)
	}
}

// --- helpers ---

func modelRefIDs(refs []ModelRef) []string {
	ids := make([]string, len(refs))
	for i, r := range refs {
		ids[i] = r.QualifiedID
	}
	return ids
}

func comboRefNames(refs []ComboRef) []string {
	names := make([]string, len(refs))
	for i, r := range refs {
		names[i] = r.Name
	}
	return names
}

func aliasRefNames(refs []AliasRef) []string {
	names := make([]string, len(refs))
	for i, r := range refs {
		names[i] = r.Name
	}
	return names
}

func assertContains(t *testing.T, slice []string, val string) {
	t.Helper()
	for _, s := range slice {
		if s == val {
			return
		}
	}
	t.Errorf("expected %v to contain %q", slice, val)
}

func assertNotContains(t *testing.T, slice []string, val string) {
	t.Helper()
	for _, s := range slice {
		if s == val {
			t.Errorf("expected %v to NOT contain %q", slice, val)
			return
		}
	}
}
