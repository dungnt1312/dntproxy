package port

import "github.com/dungnt/dntproxy/internal/domain"

// ModelFetcher fetches available models from a provider's API.
type ModelFetcher interface {
	// FetchModels returns the list of model IDs available for the connection.
	// Returns nil if the provider doesn't support model fetching.
	FetchModels(conn *domain.ProviderConnection) ([]string, error)
}

// ModelListResult holds fetched model info.
type ModelListResult struct {
	Models []domain.ModelInfo
}

// ModelInfo holds basic model metadata.
type ModelInfo struct {
	ID       string
	Name     string
	Provider string
}

// StandardModelFetcher is the default implementation using OpenAI-compatible /v1/models endpoint.
type StandardModelFetcher struct{}

// NewStandardModelFetcher creates a standard model fetcher.
func NewStandardModelFetcher() *StandardModelFetcher {
	return &StandardModelFetcher{}
}

// FetchModels calls the provider's /v1/models endpoint.
func (f *StandardModelFetcher) FetchModels(conn *domain.ProviderConnection) ([]string, error) {
	// TODO: implement standard OpenAI-compatible /v1/models fetch
	return nil, nil
}
