package custom

import (
	"fmt"

	"github.com/dungnt/dntproxy/internal/domain"
)

// NoOpModelFetcher is a stub model fetcher for providers without model list API support.
type NoOpModelFetcher struct{}

// NewNoOpModelFetcher creates a no-op model fetcher.
func NewNoOpModelFetcher() *NoOpModelFetcher {
	return &NoOpModelFetcher{}
}

// FetchModels returns the default models from provider config.
func (n *NoOpModelFetcher) FetchModels(conn *domain.ProviderConnection) ([]string, error) {
	cfg := domain.GetProviderConfig(conn.Provider)
	if len(cfg.DefaultModels) == 0 {
		return nil, fmt.Errorf("model fetch not supported for %s", conn.Provider)
	}
	return cfg.DefaultModels, nil
}
