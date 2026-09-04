package http

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/provider"
	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gin-gonic/gin"
)

func newMCPTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	cfg := domain.DefaultConfig()
	cfg.Settings.AdminKeyBootstrapped = true
	cfg.Settings.DashboardAccessMigrated = true
	cfg.APIKeys = []domain.APIKey{{
		ID: "k1", Name: "proxy", Key: "sk-dnt-mcp-test", IsActive: true,
	}}
	if err := db.Update(func(c *domain.AppConfig) {
		c.APIKeys = cfg.APIKeys
		c.Settings = cfg.Settings
	}); err != nil {
		t.Fatal(err)
	}

	return NewRouter(db, provider.NewRegistry(), provider.NewImageRegistry(), nil)
}

func TestMCPRequiresAPIKey(t *testing.T) {
	router := newMCPTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 without key", w.Code)
	}
}

func TestMCPRejectsInvalidKey(t *testing.T) {
	router := newMCPTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Authorization", "Bearer sk-wrong")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 with invalid key", w.Code)
	}
}

func TestMCPValidKeyPassesAuth(t *testing.T) {
	router := newMCPTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Authorization", "Bearer sk-dnt-mcp-test")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Fatalf("status = %d, valid key must pass auth (body: %s)", w.Code, w.Body.String())
	}
}
