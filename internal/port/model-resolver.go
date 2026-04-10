package port

import "github.com/dungnt/dntproxy/internal/domain"

// ModelResolver handles model string parsing, alias resolution, and combo expansion.
type ModelResolver interface {
	// Resolve parses a model string and returns provider + model info.
	Resolve(modelStr string) (*domain.ModelInfo, error)

	// GetComboModels returns the model list if modelStr is a combo name, nil otherwise.
	GetComboModels(modelStr string) ([]string, error)
}
