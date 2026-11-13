package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	providerAdapter "github.com/dungnt/dntproxy/internal/adapter/provider"
	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// failingImageProvider always fails the way a broke upstream account does.
type failingImageProvider struct {
	calls int
}

func (*failingImageProvider) Capabilities(string) domain.ImageCapabilities {
	return domain.ImageCapabilities{
		Generate:        true,
		Edit:            false,
		ResponseFormats: []string{"b64_json"},
	}
}

func (p *failingImageProvider) Generate(context.Context, port.ImageRequest) ([]domain.ImageResult, int, error) {
	p.calls++
	return nil, http.StatusBadRequest, errors.New(`returned 400: {"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`)
}

func (*failingImageProvider) Edit(context.Context, port.ImageRequest) ([]domain.ImageResult, int, error) {
	return nil, http.StatusBadRequest, errors.New("edit not supported")
}

func newImageTestStore(t *testing.T) *storage.JsonDB {
	t.Helper()
	store, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.DefaultConfig()
	cfg.ProviderConnections = []domain.ProviderConnection{{
		ID:               "image-conn",
		Provider:         "minimax",
		Name:             "MiniMax",
		AuthType:         "apikey",
		APIKey:           "provider-key",
		IsActive:         true,
		Weight:           100,
		SupportedModels:  []string{"image-01"},
		RateLimitedUntil: time.Now().Add(20 * time.Second).UTC().Format(time.RFC3339),
		LastError:        "http2: response body closed",
		LastErrorAt:      time.Now().UTC().Format(time.RFC3339),
	}}
	if err := store.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	return store
}

// Image routes resolve credentials directly instead of going through
// ChatService, which normally records failures. Without recording, an account
// that always fails kept being picked for every following image request.
func TestImageGeneration_RecordsFailureOnConnection(t *testing.T) {
	store := newImageTestStore(t)
	// Start from a healthy connection so the route can select it.
	if err := store.Update(func(cfg *domain.AppConfig) {
		cfg.ProviderConnections[0].RateLimitedUntil = ""
		cfg.ProviderConnections[0].LastError = ""
	}); err != nil {
		t.Fatal(err)
	}

	provider := &failingImageProvider{}
	images := providerAdapter.NewImageRegistry()
	images.RegisterImageProvider("minimax", provider)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/images/generations", imageGenerationsHandler(store, images))

	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations",
		strings.NewReader(`{"model":"minimax/image-01","prompt":"draw","response_format":"b64_json"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code == http.StatusOK {
		t.Fatalf("expected a failure, got 200 body=%s", response.Body.String())
	}
	if provider.calls != 1 {
		t.Fatalf("expected 1 provider call, got %d", provider.calls)
	}

	conn, _ := store.GetConnectionByID("image-conn")
	if conn == nil {
		t.Fatal("connection missing")
	}
	if conn.LastError == "" {
		t.Fatalf("failure must be recorded on the connection, conn=%+v", conn)
	}
	if conn.RateLimitedUntil == "" || !domain.IsAccountUnavailable(conn.RateLimitedUntil) {
		t.Fatalf("a spent balance must sideline the account, got %q", conn.RateLimitedUntil)
	}

	// The following image request must not pick the cooled connection again.
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, selErr := selectImageCredentials(ctx, store, "minimax", "image-01", "minimax/image-01", "", "")
	if selErr == nil {
		t.Fatal("expected selection to skip the cooling connection")
	}
}

// A success must clear a stale transient cooldown on the image path too.
func TestImageGeneration_ClearsFailureOnSuccess(t *testing.T) {
	store := newImageTestStore(t)

	clearImageFailure(store, &domain.Credentials{ConnectionID: "image-conn"}, "image-01")

	conn, _ := store.GetConnectionByID("image-conn")
	if conn.RateLimitedUntil != "" || conn.LastError != "" {
		t.Fatalf("successful image call must clear a transient cooldown, got %+v", conn)
	}
}

// A cooling pool on an image route must answer 429 with the recovery hint
// (not an open-ended 503) so the caller knows when to retry.
func TestImageGeneration_CoolingPoolReturnsRetryAfter(t *testing.T) {
	store := newImageTestStore(t) // connection starts cooling for 20s
	images := providerAdapter.NewImageRegistry()
	images.RegisterImageProvider("minimax", &failingImageProvider{})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/images/generations", imageGenerationsHandler(store, images))

	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations",
		strings.NewReader(`{"model":"minimax/image-01","prompt":"draw","response_format":"b64_json"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for a cooling pool, got %d body=%s", response.Code, response.Body.String())
	}
	retry := response.Header().Get("Retry-After")
	if retry == "" {
		t.Fatal("expected a Retry-After header")
	}
	secs, err := strconv.Atoi(retry)
	if err != nil || secs <= 0 || secs > 30 {
		t.Fatalf("expected a short positive Retry-After, got %q", retry)
	}
}
