package http

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gin-gonic/gin"
)

func TestAPIClearConnectionErrorModelOnly(t *testing.T) {
	store, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}

	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	cfg := domain.DefaultConfig()
	cfg.ProviderConnections = []domain.ProviderConnection{{
		ID:               "conn-ws",
		Provider:         "openai-compatible",
		Name:             "ws",
		IsActive:         true,
		RateLimitedUntil: future,
		BackoffLevel:     3,
		LastError:        "returned 403: model_not_entitled claude-4-opus",
		LastErrorAt:      future,
		ModelLocks: map[string]string{
			"claude-4-opus": future,
			"kimi-k2-6":     future,
		},
	}}
	if err := store.Save(&cfg); err != nil {
		t.Fatalf("save db: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/connections/:id/clear-error", apiClearConnectionError(store))

	rec := performJSONRequest(router, http.MethodPost, "/connections/conn-ws/clear-error", map[string]any{"model": "claude-4-opus"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}

	conn, err := store.GetConnectionByID("conn-ws")
	if err != nil {
		t.Fatalf("get connection: %v", err)
	}
	if conn == nil {
		t.Fatal("connection missing")
	}
	if conn.RateLimitedUntil != future || conn.BackoffLevel != 3 {
		t.Fatalf("account cooldown should remain, got until=%q backoff=%d", conn.RateLimitedUntil, conn.BackoffLevel)
	}
	if _, ok := conn.ModelLocks["claude-4-opus"]; ok {
		t.Fatalf("model lock was not cleared: %#v", conn.ModelLocks)
	}
	if _, ok := conn.ModelLocks["kimi-k2-6"]; !ok {
		t.Fatalf("unrelated model lock should remain: %#v", conn.ModelLocks)
	}
	if conn.LastError != "" || conn.LastErrorAt != "" {
		t.Fatalf("last error should be cleared for matching model, got %q at %q", conn.LastError, conn.LastErrorAt)
	}
}

func TestAPIClearConnectionErrorNormalizesPublicModelID(t *testing.T) {
	store, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}

	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	cfg := domain.DefaultConfig()
	cfg.ProviderConnections = []domain.ProviderConnection{{
		ID:          "conn-ws",
		Provider:    "openai-compatible",
		Name:        "Windsurf",
		RoutePrefix: "windsurf",
		IsActive:    true,
		LastError:   "returned 403: model_not_entitled RL-4m",
		LastErrorAt: future,
		ModelLocks:  map[string]string{"RL-4m": future},
	}}
	if err := store.Save(&cfg); err != nil {
		t.Fatalf("save db: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/connections/:id/clear-error", apiClearConnectionError(store))

	rec := performJSONRequest(router, http.MethodPost, "/connections/conn-ws/clear-error", map[string]any{"model": "windsurf/RL-4m"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}

	conn, err := store.GetConnectionByID("conn-ws")
	if err != nil {
		t.Fatalf("get connection: %v", err)
	}
	if conn == nil {
		t.Fatal("connection missing")
	}
	if len(conn.ModelLocks) != 0 {
		t.Fatalf("expected public model ID to clear raw model lock, got %#v", conn.ModelLocks)
	}
}
