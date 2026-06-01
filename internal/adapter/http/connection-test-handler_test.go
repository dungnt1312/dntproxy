package http

import (
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestConnectionTestURLKeepsXAIVersionPrefix(t *testing.T) {
	conn := &domain.ProviderConnection{
		Provider: "xai",
		BaseURL:  "https://api.x.ai/v1",
	}
	cfg := domain.GetProviderConfig("xai")

	got := providerTestURL(conn, cfg)
	want := "https://api.x.ai/v1/responses"
	if got != want {
		t.Fatalf("providerTestURL() = %q, want %q", got, want)
	}
}

func TestConnectionTestURLStripsVersionForChatCompletionsProviders(t *testing.T) {
	conn := &domain.ProviderConnection{
		Provider: "openai-compatible",
		BaseURL:  "https://example.com/v1",
	}
	cfg := domain.GetProviderConfig("openai-compatible")

	got := providerTestURL(conn, cfg)
	want := "https://example.com/v1/chat/completions"
	if got != want {
		t.Fatalf("providerTestURL() = %q, want %q", got, want)
	}
}
