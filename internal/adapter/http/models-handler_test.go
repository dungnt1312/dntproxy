package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/gin-gonic/gin"
)

func TestModelsHandlerAppliesAPIKeyPolicy(t *testing.T) {
	store, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}

	cfg := domain.DefaultConfig()
	cfg.Settings.RequireAPIKey = true
	cfg.ProviderConnections = []domain.ProviderConnection{
		{ID: "conn-glm", Provider: "glm", Name: "GLM", AuthType: "apikey", IsActive: true, Weight: 100, SupportedModels: []string{"glm-lite", "glm-pro"}},
		{ID: "conn-openai", Provider: "openai", Name: "OpenAI", AuthType: "apikey", IsActive: true, Weight: 100, SupportedModels: []string{"gpt-4o"}},
	}
	cfg.Combos = []domain.Combo{
		{Name: "glm-combo", Models: []string{"glm/glm-lite", "openai/gpt-4o"}},
		{Name: "openai-combo", Models: []string{"openai/gpt-4o"}},
	}
	cfg.ModelAliases = domain.AliasMap{
		"lite": "glm/glm-lite",
		"gpt":  "openai/gpt-4o",
	}
	cfg.APIKeys = []domain.APIKey{
		{
			ID:                   "key-1",
			Name:                 "GLM Lite",
			Key:                  "sk-test",
			IsActive:             true,
			AllowedConnectionIDs: []string{"conn-glm"},
			AllowedModels:        []string{"glm/glm-lite"},
		},
	}
	if err := store.Save(&cfg); err != nil {
		t.Fatalf("save db: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/v1")
	group.Use(apiKeyMiddleware(store))
	modelAccess := service.NewModelAccessService(store)
	group.GET("/models", modelsHandler(modelAccess))

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-test")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Data []modelObject `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	ids := make(map[string]string, len(body.Data))
	for _, model := range body.Data {
		ids[model.ID] = model.OwnedBy
	}

	for _, want := range []string{"glm/glm-lite", "glm-combo", "lite"} {
		if _, ok := ids[want]; !ok {
			t.Fatalf("missing allowed model %q in %#v", want, ids)
		}
	}
	for _, disallowed := range []string{"glm/glm-pro", "openai/gpt-4o", "openai-combo", "gpt"} {
		if _, ok := ids[disallowed]; ok {
			t.Fatalf("disallowed model %q was listed in %#v", disallowed, ids)
		}
	}
}
