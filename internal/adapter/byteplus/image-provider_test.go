package byteplus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

func TestImageProviderGenerateAndEdit(t *testing.T) {
	requests := make(chan ImageRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ImageRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		requests <- request
		_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/result.png"}]}`))
	}))
	defer server.Close()

	provider := NewImageProviderWithClient(&ImageClient{HTTPClient: server.Client()})
	credentials := &domain.Credentials{BaseURL: server.URL, APIKey: "secret"}
	generation := port.ImageRequest{
		Model:       "byteplus/seedream-4-5-250428",
		Body:        []byte(`{"prompt":"Create a fox","response_format":"url"}`),
		Credentials: credentials,
	}
	if _, status, err := provider.Generate(context.Background(), generation); err != nil || status != http.StatusOK {
		t.Fatalf("Generate status=%d err=%v", status, err)
	}
	if generated := <-requests; generated.Model != "seedream-4-5-250428" || generated.Image != nil {
		t.Fatalf("generation request = %#v", generated)
	}

	edit := port.ImageRequest{
		Model:       "seedream-4-5-250428",
		Body:        []byte(`{"prompt":"Make it blue","image":"https://example.com/input.png"}`),
		Credentials: credentials,
	}
	if _, status, err := provider.Edit(context.Background(), edit); err != nil || status != http.StatusOK {
		t.Fatalf("Edit status=%d err=%v", status, err)
	}
	if edited := <-requests; edited.Image != "https://example.com/input.png" {
		t.Fatalf("edit request = %#v", edited)
	}
}

func TestImageProviderRejectsUnsupportedEditModel(t *testing.T) {
	provider := NewImageProvider()
	_, status, err := provider.Edit(context.Background(), port.ImageRequest{
		Model: "seedream-3-0-t2i",
		Body:  []byte(`{"prompt":"edit","image":"https://example.com/input.png"}`),
	})
	if err == nil || status != http.StatusBadRequest {
		t.Fatalf("status=%d err=%v", status, err)
	}
}
