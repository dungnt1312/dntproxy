package service

import (
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// ModelResolver handles model string parsing, alias resolution, and combo expansion.
type ModelResolver struct {
	store port.CredentialStore
}

// NewModelResolver creates a new ModelResolver.
func NewModelResolver(store port.CredentialStore) *ModelResolver {
	return &ModelResolver{store: store}
}

// Resolve parses a model string and returns provider + model.
// Supports: "alias/model", "provider/model", combo names, model aliases.
func (r *ModelResolver) Resolve(modelStr string) (*domain.ModelInfo, error) {
	// Check if it's "provider/model" or "alias/model" format
	if strings.Contains(modelStr, "/") {
		idx := strings.Index(modelStr, "/")
		providerOrAlias := modelStr[:idx]
		model := modelStr[idx+1:]

		provider := resolveProviderAlias(providerOrAlias)

		// Try to get model metadata from registry
		cfg, err := r.store.Load()
		var modelDef *domain.ModelDefinition
		if err == nil && cfg != nil && cfg.ModelRegistry != nil {
			modelDef = cfg.ModelRegistry.GetModel(provider + "/" + model)
		}

		return &domain.ModelInfo{
			Provider:      provider,
			Model:         model,
			ProviderAlias: providerOrAlias,
			Definition:    modelDef,
		}, nil
	}

	// Try model alias
	aliases, err := r.store.GetModelAliases()
	if err == nil && aliases != nil {
		if resolved, ok := aliases[modelStr]; ok && strings.Contains(resolved, "/") {
			idx := strings.Index(resolved, "/")
			providerOrAlias := resolved[:idx]
			model := resolved[idx+1:]
			provider := resolveProviderAlias(providerOrAlias)

			// Try to get model metadata from registry
			cfg, err := r.store.Load()
			var modelDef *domain.ModelDefinition
			if err == nil && cfg != nil && cfg.ModelRegistry != nil {
				modelDef = cfg.ModelRegistry.GetModel(provider + "/" + model)
			}

			return &domain.ModelInfo{
				Provider:      provider,
				Model:         model,
				ProviderAlias: providerOrAlias,
				Definition:    modelDef,
			}, nil
		}
	}

	// Not a provider/model format and not an alias — could be a combo name
	// Return with empty provider to signal caller to check combos
	return &domain.ModelInfo{
		Provider: "",
		Model:    modelStr,
	}, nil
}

// GetComboModels returns the model list if modelStr is a combo name, nil otherwise.
func (r *ModelResolver) GetComboModels(modelStr string) ([]string, error) {
	// Don't check if it's in provider/model format
	if strings.Contains(modelStr, "/") {
		return nil, nil
	}

	combo, err := r.store.GetComboByName(modelStr)
	if err != nil {
		return nil, err
	}
	if combo != nil && len(combo.Models) > 0 {
		return combo.Models, nil
	}
	return nil, nil
}

func resolveProviderAlias(aliasOrID string) string {
	if id, ok := domain.ProviderAliasToID[aliasOrID]; ok {
		return id
	}
	return aliasOrID
}
