package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gin-gonic/gin"
)

func TestAPIListModelsOmitsInactiveConnections(t *testing.T) {
	store, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}

	cfg := domain.DefaultConfig()
	cfg.ProviderConnections = []domain.ProviderConnection{
		{ID: "active-openai", Provider: "openai", Name: "Active OpenAI", AuthType: "apikey", IsActive: true, Weight: 100, SupportedModels: []string{"gpt-4o"}},
		{ID: "inactive-openai", Provider: "openai", Name: "Inactive OpenAI", AuthType: "apikey", IsActive: false, Weight: 100, SupportedModels: []string{"gpt-4o", "gpt-inactive-only"}},
	}
	if err := store.Save(&cfg); err != nil {
		t.Fatalf("save db: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/models", apiListModels(store))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}

	var body []struct {
		ID          string `json:"id"`
		Connections []struct {
			ID       string `json:"id"`
			IsActive bool   `json:"isActive"`
		} `json:"connections"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	models := make(map[string][]string)
	for _, model := range body {
		for _, conn := range model.Connections {
			if !conn.IsActive {
				t.Fatalf("inactive connection included for %s: %#v", model.ID, conn)
			}
			models[model.ID] = append(models[model.ID], conn.ID)
		}
	}

	if got := models["openai/gpt-4o"]; len(got) != 1 || got[0] != "active-openai" {
		t.Fatalf("active model connections = %#v", got)
	}
	if _, ok := models["openai/gpt-inactive-only"]; ok {
		t.Fatalf("inactive-only model was listed: %#v", models)
	}
}
