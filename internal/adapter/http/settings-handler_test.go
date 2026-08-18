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

func newSettingsTestRouter(t *testing.T) (*gin.Engine, *storage.JsonDB, domain.APIKey) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	admin := domain.APIKey{
		ID: "admin-1", Name: "Admin", Key: "sk-dnt-admin-test",
		IsActive: true, DashboardAccess: true,
	}
	cfg := domain.DefaultConfig()
	cfg.Settings.AdminKeyBootstrapped = true
	cfg.Settings.DashboardAccessMigrated = true
	cfg.Settings.RequireAPIKey = true
	cfg.APIKeys = []domain.APIKey{admin}
	if err := db.Save(&cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	r := gin.New()
	r.Use(dashboardKeyMiddleware(db))
	r.GET("/api/settings", apiGetSettings(db))
	r.PUT("/api/settings", apiUpdateSettings(db))
	return r, db, admin
}

func TestGetSettingsUnauthenticatedOmitsSecrets(t *testing.T) {
	r, _, _ := newSettingsTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["telegram"]; ok {
		t.Fatalf("unauthenticated settings leaked telegram: %s", w.Body.String())
	}
	if _, ok := body["requireApiKey"]; ok {
		t.Fatal("unauthenticated GET should not return requireApiKey")
	}
	if _, ok := body["port"]; ok {
		t.Fatal("unauthenticated GET should not return port")
	}
	if w.Body.String() != "" && bytes.Contains(w.Body.Bytes(), []byte("secret-bot-token")) {
		t.Fatal("bot token leaked to unauthenticated caller")
	}
}

func TestGetSettingsAuthenticatedOmitsPortAndRequireAPIKey(t *testing.T) {
	r, _, admin := newSettingsTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+admin.Key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["port"]; ok {
		t.Fatal("authenticated GET should not return port")
	}
	if _, ok := body["requireApiKey"]; ok {
		t.Fatal("authenticated GET should not return requireApiKey")
	}
	if _, ok := body["comboStrategy"]; !ok {
		t.Fatal("authenticated GET missing comboStrategy")
	}
}

func TestUpdateSettingsIgnoresPortAndRequireAPIKey(t *testing.T) {
	r, db, admin := newSettingsTestRouter(t)
	cfg, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Settings.Port = 20199
	cfg.Settings.RequireAPIKey = true
	if err := db.Save(cfg); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(
		`{"port":8080,"requireApiKey":false,"logBodies":true,"comboStrategy":"fallback"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin.Key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	cfg, err = db.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Settings.Port != 20199 {
		t.Fatalf("Port was written from settings API: %d", cfg.Settings.Port)
	}
	if !cfg.Settings.RequireAPIKey {
		t.Fatal("RequireAPIKey was written from settings API")
	}
	if !cfg.Settings.LogBodies {
		t.Fatal("LogBodies was not updated")
	}
}

func TestV1AlwaysRequiresAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.DefaultConfig()
	cfg.Settings.AdminKeyBootstrapped = true
	cfg.Settings.RequireAPIKey = false
	if err := db.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	r := NewRouter(db, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("open /v1 with requireApiKey=false: got %d want 401 body=%s", w.Code, w.Body.String())
	}
}

func TestOriginAllowedRejectsPrefixSpoof(t *testing.T) {
	if originAllowed("http://localhost.evil.com", nil) {
		t.Fatal("localhost prefix spoof allowed")
	}
	if originAllowed("http://127.0.0.1.nip.io", nil) {
		t.Fatal("127.0.0.1 prefix spoof allowed")
	}
	if !originAllowed("http://localhost:20199", nil) {
		t.Fatal("exact localhost:port should be allowed")
	}
	if originAllowed("https://evil.trycloudflare.com", nil) {
		t.Fatal("foreign tunnel origin allowed")
	}
}
