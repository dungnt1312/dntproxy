package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestImageProviderConnectionProbeIsNonGenerative(t *testing.T) {
	var method, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	result := testImageProviderAPI(
		&domain.ProviderConnection{APIKey: "test-key", BaseURL: server.URL + "/api/v3"},
		domain.ProviderConfig{Format: domain.FormatImageAPI},
	)
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if method != http.MethodGet || path != "/api/v3/models" {
		t.Fatalf("probe = %s %s, want GET /api/v3/models", method, path)
	}
}
