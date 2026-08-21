package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestImportBackupAcceptsObjectComboStrategies(t *testing.T) {
	r, db, admin := newSettingsTestRouter(t)
	r.POST("/api/backup/import", apiImportBackup(db))

	body := `{
		"version": "1.0",
		"providerConnections": [],
		"combos": [],
		"modelAliases": {},
		"apiKeys": [{
			"id": "k-admin", "name": "admin", "key": "sk-admin-test",
			"isActive": true, "dashboardAccess": true
		}],
		"settings": {"comboStrategies": {"o1": {"strategy": "fallback"}}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin.Key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	cfg, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Settings.ComboStrategies["o1"]; got != "fallback" {
		t.Fatalf("comboStrategies[o1] = %q, want fallback", got)
	}
}

func TestImportBackupVersionlessImportsConnectionsOnly(t *testing.T) {
	r, db, admin := newSettingsTestRouter(t)
	r.POST("/api/backup/import", apiImportBackup(db))

	if err := db.Update(func(cfg *domain.AppConfig) {
		cfg.Combos = []domain.Combo{{
			ID: "combo-1", Name: "keep-me", Models: []string{"openai/gpt-4"},
		}}
		cfg.ModelAliases = domain.AliasMap{"alias-a": "openai/gpt-4"}
	}); err != nil {
		t.Fatal(err)
	}

	body := `{
		"settings": {"comboStrategies": {"gpt-content": {"fallbackStrategy": "round-robin"}}},
		"providerConnections": [{
			"id": "codex-1", "provider": "codex", "authType": "oauth",
			"name": "Imported", "refreshToken": "test-refresh-token"
		}],
		"apiKeys": [{"id": "foreign", "name": "foreign", "key": "sk-foreign", "isActive": true}],
		"combos": [{"id": "foreign-combo", "name": "claude-haiku", "models": ["claude-haiku-4-5"]}],
		"modelAliases": {"foreign-alias": "claude-opus-4-6"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin.Key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	cfg, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ProviderConnections) != 1 || cfg.ProviderConnections[0].Provider != "openai" {
		t.Fatalf("connections = %#v, want codex mapped to openai", cfg.ProviderConnections)
	}
	if len(cfg.APIKeys) != 1 || cfg.APIKeys[0].Key != admin.Key {
		t.Fatalf("apiKeys were overridden: %#v", cfg.APIKeys)
	}
	if len(cfg.Combos) != 1 || cfg.Combos[0].Name != "keep-me" {
		t.Fatalf("combos were overridden: %#v", cfg.Combos)
	}
	if cfg.ModelAliases["alias-a"] != "openai/gpt-4" {
		t.Fatalf("aliases were overridden: %#v", cfg.ModelAliases)
	}
}
