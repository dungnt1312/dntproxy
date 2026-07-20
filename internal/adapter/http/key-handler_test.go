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

func newKeyTestRouter(t *testing.T) (*gin.Engine, *storage.JsonDB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	cfg := domain.DefaultConfig()
	cfg.ProviderConnections = []domain.ProviderConnection{
		{ID: "conn-openai", Name: "OpenAI Main", Provider: "openai", AuthType: "apikey", IsActive: true, Weight: 100},
		{ID: "conn-kiro", Name: "Kiro Main", Provider: "kiro", AuthType: "oauth", IsActive: true, Weight: 100},
	}
	// Mark bootstrap done so the storage layer does not auto-create an admin
	// key on this fresh test DB, which would skew key-count assertions.
	cfg.Settings.AdminKeyBootstrapped = true
	if err := db.Save(&cfg); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	r := gin.New()
	r.GET("/keys", apiListKeys(db))
	r.POST("/keys", apiCreateKey(db))
	r.PUT("/keys/:id", apiUpdateKey(db))
	r.DELETE("/keys/:id", apiDeleteKey(db))
	return r, db
}

func performJSONRequest(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var reader bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&reader).Encode(body)
	}
	req := httptest.NewRequest(method, path, &reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAPIKeyCreateStoresAllowlists(t *testing.T) {
	router, db := newKeyTestRouter(t)

	rec := performJSONRequest(router, http.MethodPost, "/keys", map[string]any{
		"name":                 "restricted",
		"allowedConnectionIds": []string{"conn-openai", "conn-openai", ""},
		"allowedModels":        []string{"oai/gpt-4o", "oai/gpt-4o", ""},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}

	cfg, err := db.Load()
	if err != nil {
		t.Fatalf("load db: %v", err)
	}
	if len(cfg.APIKeys) != 1 {
		t.Fatalf("api key count = %d", len(cfg.APIKeys))
	}
	key := cfg.APIKeys[0]
	if got := key.AllowedConnectionIDs; len(got) != 1 || got[0] != "conn-openai" {
		t.Fatalf("allowed connections = %#v", got)
	}
	if got := key.AllowedModels; len(got) != 1 || got[0] != "oai/gpt-4o" {
		t.Fatalf("allowed models = %#v", got)
	}
}

func TestAPIKeyUpdateStoresPermissionsAndActiveState(t *testing.T) {
	router, db := newKeyTestRouter(t)
	create := performJSONRequest(router, http.MethodPost, "/keys", map[string]any{"name": "restricted"})
	if create.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", create.Code, create.Body.String())
	}

	cfg, err := db.Load()
	if err != nil {
		t.Fatalf("load db: %v", err)
	}
	id := cfg.APIKeys[0].ID

	rec := performJSONRequest(router, http.MethodPut, "/keys/"+id, map[string]any{
		"name":                 "updated",
		"isActive":             false,
		"allowedConnectionIds": []string{"conn-kiro"},
		"allowedModels":        []string{"kr/claude-sonnet"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", rec.Code, rec.Body.String())
	}

	cfg, err = db.Load()
	if err != nil {
		t.Fatalf("load db: %v", err)
	}
	key := cfg.APIKeys[0]
	if key.Name != "updated" || key.IsActive {
		t.Fatalf("key metadata = %#v", key)
	}
	if got := key.AllowedConnectionIDs; len(got) != 1 || got[0] != "conn-kiro" {
		t.Fatalf("allowed connections = %#v", got)
	}
	if got := key.AllowedModels; len(got) != 1 || got[0] != "kr/claude-sonnet" {
		t.Fatalf("allowed models = %#v", got)
	}
}

func TestAPIKeyRejectsUnknownConnection(t *testing.T) {
	router, _ := newKeyTestRouter(t)

	rec := performJSONRequest(router, http.MethodPost, "/keys", map[string]any{
		"name":                 "bad",
		"allowedConnectionIds": []string{"missing-conn"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
}
