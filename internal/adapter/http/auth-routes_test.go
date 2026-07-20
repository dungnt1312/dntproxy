package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gin-gonic/gin"
)

func newAuthHardeningRouter(t *testing.T) (*gin.Engine, *storage.JsonDB, domain.APIKey) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}

	dashKey := domain.APIKey{
		ID:              "dash-1",
		Name:            "Dashboard",
		Key:             "sk-dnt-dashboard-test",
		IsActive:        true,
		DashboardAccess: true,
	}
	proxyKey := domain.APIKey{
		ID:              "proxy-1",
		Name:            "Proxy Only",
		Key:             "sk-dnt-proxy-only",
		IsActive:        true,
		DashboardAccess: false,
	}

	cfg := domain.DefaultConfig()
	cfg.Settings.AdminKeyBootstrapped = true
	cfg.Settings.DashboardAccessMigrated = true
	cfg.APIKeys = []domain.APIKey{dashKey, proxyKey}
	cfg.ProviderConnections = []domain.ProviderConnection{
		{
			ID:       "conn-openai",
			Name:     "OpenAI",
			Provider: "openai",
			AuthType: "apikey",
			IsActive: true,
			Weight:   100,
			TenantID: "",
		},
		{
			ID:       "conn-acme",
			Name:     "Acme Kiro",
			Provider: "kiro",
			AuthType: "oauth",
			IsActive: true,
			Weight:   100,
			TenantID: "acme",
		},
	}
	if err := db.Save(&cfg); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	r := gin.New()
	// Mirror production: dashboard middleware + auth routes under /api
	api := r.Group("/api")
	api.Use(dashboardKeyMiddleware(db))
	{
		api.POST("/auth/validate-key", apiValidateKey(db))
		api.GET("/auth/session", apiSession(db))
		RegisterAuthRoutes(api, db)
	}
	return r, db, dashKey
}

func TestAuthRoutesRequireDashboardKey(t *testing.T) {
	r, _, _ := newAuthHardeningRouter(t)

	paths := []string{
		"/api/auth/kiro/start-builderid",
		"/api/auth/kiro/start-idc",
		"/api/auth/kiro/poll",
		"/api/auth/kiro/start-social",
		"/api/auth/kiro/exchange-social",
		"/api/auth/openai/start",
		"/api/auth/openai/exchange",
		"/api/auth/qwen/start",
		"/api/auth/qwen/poll",
		"/api/auth/xai/start",
		"/api/auth/xai/exchange",
		"/api/auth/xai/import-file",
		"/api/connections/conn-openai/fetch-models",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s without key: got %d want 401 body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestAuthRoutesRejectProxyOnlyKey(t *testing.T) {
	r, _, _ := newAuthHardeningRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/openai/start", nil)
	req.Header.Set("Authorization", "Bearer sk-dnt-proxy-only")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("proxy-only key: got %d want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestAuthRoutesAcceptDashboardKey(t *testing.T) {
	r, _, dash := newAuthHardeningRouter(t)

	// openai/start is local-only PKCE setup — should not 401/403 with dashboard key
	req := httptest.NewRequest(http.MethodPost, "/api/auth/openai/start", nil)
	req.Header.Set("Authorization", "Bearer "+dash.Key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Fatalf("dashboard key rejected: status=%d body=%s", w.Code, w.Body.String())
	}
	if w.Code != http.StatusOK {
		// PKCE start should succeed without network
		t.Fatalf("openai start: got %d body=%s", w.Code, w.Body.String())
	}
}

func TestFetchModelsEnforcesTenantOwnership(t *testing.T) {
	r, db, _ := newAuthHardeningRouter(t)

	// Tenant-scoped dashboard key
	cfg, err := db.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cfg.APIKeys = append(cfg.APIKeys, domain.APIKey{
		ID:              "tenant-key",
		Name:            "Acme Dash",
		Key:             "sk-dnt-acme-dash",
		IsActive:        true,
		DashboardAccess: true,
		TenantID:        "acme",
	})
	if err := db.Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Tenant acme must not see admin-owned conn-openai
	req := httptest.NewRequest(http.MethodPost, "/api/connections/conn-openai/fetch-models", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer sk-dnt-acme-dash")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant fetch-models: got %d want 404 body=%s", w.Code, w.Body.String())
	}

	// Own connection should not 404 for ownership (may return provider defaults)
	req2 := httptest.NewRequest(http.MethodPost, "/api/connections/conn-acme/fetch-models", bytes.NewBufferString(`{}`))
	req2.Header.Set("Authorization", "Bearer sk-dnt-acme-dash")
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code == http.StatusNotFound || w2.Code == http.StatusUnauthorized || w2.Code == http.StatusForbidden {
		t.Fatalf("own connection fetch-models failed: %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestAPISessionRejectsNonDashboardAndInactive(t *testing.T) {
	r, db, dash := newAuthHardeningRouter(t)

	// Proxy-only key → not authenticated for dashboard session
	req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	req.Header.Set("Authorization", "Bearer sk-dnt-proxy-only")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["authenticated"] != false {
		t.Fatalf("proxy-only session: %#v", body)
	}

	// Valid dashboard key → authenticated
	req2 := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	req2.Header.Set("Authorization", "Bearer "+dash.Key)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var body2 map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &body2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body2["authenticated"] != true {
		t.Fatalf("dashboard session: %#v", body2)
	}

	// Deactivate dashboard key
	cfg, err := db.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for i := range cfg.APIKeys {
		if cfg.APIKeys[i].Key == dash.Key {
			cfg.APIKeys[i].IsActive = false
		}
	}
	if err := db.Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	req3.Header.Set("Authorization", "Bearer "+dash.Key)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	var body3 map[string]any
	if err := json.Unmarshal(w3.Body.Bytes(), &body3); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body3["authenticated"] != false {
		t.Fatalf("inactive key session: %#v", body3)
	}
}
