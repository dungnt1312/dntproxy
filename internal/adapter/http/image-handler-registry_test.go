package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	providerAdapter "github.com/dungnt/dntproxy/internal/adapter/provider"
	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

type recordingImageProvider struct {
	generations int
	edits       int
}

func (*recordingImageProvider) Capabilities(string) domain.ImageCapabilities {
	return domain.ImageCapabilities{
		Generate:        true,
		Edit:            true,
		MaxReferences:   1,
		ResponseFormats: []string{"b64_json"},
	}
}

func (p *recordingImageProvider) Generate(context.Context, port.ImageRequest) ([]domain.ImageResult, int, error) {
	p.generations++
	return []domain.ImageResult{{B64JSON: "aW1hZ2U="}}, http.StatusOK, nil
}

func (p *recordingImageProvider) Edit(context.Context, port.ImageRequest) ([]domain.ImageResult, int, error) {
	p.edits++
	return []domain.ImageResult{{B64JSON: "ZWRpdA=="}}, http.StatusOK, nil
}

func TestImageHandlersDispatchThroughImageRegistry(t *testing.T) {
	store, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.DefaultConfig()
	cfg.ProviderConnections = []domain.ProviderConnection{{
		ID:              "conn-minimax",
		Provider:        "minimax",
		Name:            "MiniMax",
		AuthType:        "apikey",
		APIKey:          "provider-key",
		IsActive:        true,
		Weight:          100,
		SupportedModels: []string{"image-01"},
	}}
	if err := store.Save(&cfg); err != nil {
		t.Fatal(err)
	}

	recorder := &recordingImageProvider{}
	images := providerAdapter.NewImageRegistry()
	images.RegisterImageProvider("minimax", recorder)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/images/generations", imageGenerationsHandler(store, images))
	router.POST("/v1/images/edits", imageEditsHandler(store, images))

	for _, request := range []struct {
		path string
		body string
	}{
		{"/v1/images/generations", `{"model":"minimax/image-01","prompt":"draw","response_format":"b64_json"}`},
		{"/v1/images/edits", `{"model":"minimax/image-01","prompt":"edit","image":"data:image/png;base64,aQ==","response_format":"b64_json"}`},
	} {
		httpRequest := httptest.NewRequest(http.MethodPost, request.path, strings.NewReader(request.body))
		httpRequest.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httpRequest)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", request.path, response.Code, response.Body.String())
		}
		var result domain.ImageGenerationsResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || len(result.Data) != 1 {
			t.Fatalf("%s response=%s err=%v", request.path, response.Body.String(), err)
		}
	}
	if recorder.generations != 1 || recorder.edits != 1 {
		t.Fatalf("calls generation=%d edit=%d", recorder.generations, recorder.edits)
	}
}
