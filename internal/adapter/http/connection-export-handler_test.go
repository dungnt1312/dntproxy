package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gin-gonic/gin"
)

func newImportConnectionsRouter(t *testing.T) (*gin.Engine, *storage.JsonDB, domain.APIKey) {
	t.Helper()
	r, db, admin := newSettingsTestRouter(t)
	r.POST("/api/connections/import-multiple", apiImportConnectionsFromFile(db))
	return r, db, admin
}

func TestImportMultipleVersionlessNineRouter(t *testing.T) {
	r, db, admin := newImportConnectionsRouter(t)

	body := `{
		"providerConnections": [
			{"id": "codex-1", "provider": "codex", "authType": "oauth", "refreshToken": "test-refresh-token", "name": "codex"},
			{"id": "glm-1", "provider": "glm", "authType": "apikey", "apiKey": "test-glm-key", "name": "glm"},
			{"id": "cline-1", "provider": "cline", "authType": "oauth", "refreshToken": "test-refresh-token", "name": "cline"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/connections/import-multiple", strings.NewReader(body))
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
	byID := map[string]domain.ProviderConnection{}
	for _, conn := range cfg.ProviderConnections {
		byID[conn.ID] = conn
	}
	if byID["codex-1"].Provider != "openai" {
		t.Fatalf("codex-1 = %#v, want openai", byID["codex-1"])
	}
	if byID["glm-1"].Provider != "glm" || byID["glm-1"].APIKey != "test-glm-key" {
		t.Fatalf("glm-1 = %#v", byID["glm-1"])
	}
	if _, ok := byID["cline-1"]; ok {
		t.Fatal("unsupported cline connection should have been skipped")
	}
	// Keys must be untouched by the connections-only import.
	if len(cfg.APIKeys) != 1 || cfg.APIKeys[0].Key != admin.Key {
		t.Fatalf("apiKeys = %#v", cfg.APIKeys)
	}
}

func TestImportMultipleVersionedBackupStillWorks(t *testing.T) {
	r, db, admin := newImportConnectionsRouter(t)

	body := `{
		"version": "1.0",
		"providerConnections": [
			{"id": "native-1", "provider": "openai", "authType": "apikey", "apiKey": "test-key", "name": "native"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/connections/import-multiple", strings.NewReader(body))
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
		t.Fatalf("connections = %#v", cfg.ProviderConnections)
	}
}
