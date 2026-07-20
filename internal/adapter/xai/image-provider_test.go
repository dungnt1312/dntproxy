package xai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

func TestImageProviderCapabilitiesAreModelSpecific(t *testing.T) {
	provider := NewImageProvider()
	if got := provider.Capabilities("grok-4.3"); got.Generate || got.Edit {
		t.Fatalf("chat model capabilities = %#v", got)
	}
	if got := provider.Capabilities("grok-imagine-image"); !got.Generate || !got.Edit {
		t.Fatalf("image model capabilities = %#v", got)
	}
}

func TestImageProviderEditForwardsOpenAICompatibleJSON(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"created":1,"data":[{"url":"https://example.test/image.png"}]}`))
	}))
	defer server.Close()
	originalClient := shared.StreamingHTTPClient
	shared.StreamingHTTPClient = server.Client()
	t.Cleanup(func() { shared.StreamingHTTPClient = originalClient })

	results, status, err := NewImageProvider().Edit(context.Background(), port.ImageRequest{
		Model: "grok-imagine-image",
		Body: []byte(`{
			"prompt":"edit",
			"images":[{"image_url":"https://example.test/reference.png"}],
			"size":"1792x1024",
			"response_format":"url"
		}`),
		Credentials: &domain.Credentials{APIKey: "key", BaseURL: server.URL},
	})
	if err != nil || status != http.StatusOK || len(results) != 1 {
		t.Fatalf("status=%d results=%#v err=%v", status, results, err)
	}
	if received["aspect_ratio"] != "16:9" {
		t.Fatalf("payload = %#v", received)
	}
}
