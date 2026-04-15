package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// ModelResolver handles model string parsing, alias resolution, and combo expansion.
type ModelResolver struct {
	store      port.CredentialStore
	modelCache *ModelCache
}

// NewModelResolver creates a new ModelResolver.
func NewModelResolver(store port.CredentialStore) *ModelResolver {
	return &ModelResolver{
		store:      store,
		modelCache: NewModelCache(10 * time.Minute),
	}
}

// RoutingResult is the unified result of resolving any model string.
type RoutingResult struct {
	// Models is a list of fully-qualified "provider/model" strings to try.
	// For a direct model or alias, this has 1 element.
	// For a combo, this has N elements.
	Models []string
	// IsCombo is true if the input resolved to a combo.
	IsCombo bool
	// ComboName is the combo name (empty if not a combo).
	ComboName string
	// AllowedConnectionIDs restricts which connections can be used (from combo config).
	AllowedConnectionIDs []string
}

// ResolveRouting resolves any model string (combo, alias, or "provider/model") into
// a unified RoutingResult with a list of fully-qualified "provider/model" strings.
func (r *ModelResolver) ResolveRouting(modelStr string) (*RoutingResult, error) {
	// 1. Check if it's a combo (only if no "/" in the string)
	if !strings.Contains(modelStr, "/") {
		combo, err := r.store.GetComboByName(modelStr)
		if err != nil {
			return nil, fmt.Errorf("lookup combo: %w", err)
		}
		if combo != nil && len(combo.Models) > 0 {
			// Normalize all models in the combo
			normalized := make([]string, 0, len(combo.Models))
			for _, m := range combo.Models {
				norm, err := r.normalizeModelStr(m)
				if err != nil {
					return nil, fmt.Errorf("normalize combo model %q: %w", m, err)
				}
				normalized = append(normalized, norm)
			}
			return &RoutingResult{
				Models:               normalized,
				IsCombo:              true,
				ComboName:            combo.Name,
				AllowedConnectionIDs: combo.ConnectionIDs,
			}, nil
		}

		// 2. Check if it's a model alias
		cfg, err := r.store.Load()
		if err == nil && cfg != nil {
			if resolved, ok := cfg.ModelAliases[modelStr]; ok && strings.Contains(resolved, "/") {
				norm, err := r.normalizeModelStr(resolved)
				if err != nil {
					return nil, fmt.Errorf("normalize alias %q: %w", resolved, err)
				}
				return &RoutingResult{
					Models: []string{norm},
				}, nil
			}
		}

		return nil, fmt.Errorf("unknown model or combo: %q", modelStr)
	}

	// 3. Direct "provider/model" format
	norm, err := r.normalizeModelStr(modelStr)
	if err != nil {
		return nil, err
	}
	return &RoutingResult{
		Models: []string{norm},
	}, nil
}

// normalizeModelStr converts "alias/model" to "provider/model" using ProviderAliasToID.
func (r *ModelResolver) normalizeModelStr(modelStr string) (string, error) {
	if !strings.Contains(modelStr, "/") {
		return "", fmt.Errorf("invalid model format: %q (expected provider/model)", modelStr)
	}
	idx := strings.Index(modelStr, "/")
	providerOrAlias := modelStr[:idx]
	model := modelStr[idx+1:]
	provider := resolveProviderAlias(providerOrAlias)
	return provider + "/" + model, nil
}

// Resolve parses a model string and returns provider + model.
// Supports: "alias/model", "provider/model", combo names, model aliases.
func (r *ModelResolver) Resolve(modelStr string) (*domain.ModelInfo, error) {
	if strings.Contains(modelStr, "/") {
		idx := strings.Index(modelStr, "/")
		providerOrAlias := modelStr[:idx]
		model := modelStr[idx+1:]

		provider := resolveProviderAlias(providerOrAlias)

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

	cfg, err := r.store.Load()
	if err == nil && cfg != nil {
		if resolved, ok := cfg.ModelAliases[modelStr]; ok && strings.Contains(resolved, "/") {
			idx := strings.Index(resolved, "/")
			providerOrAlias := resolved[:idx]
			model := resolved[idx+1:]
			provider := resolveProviderAlias(providerOrAlias)

			var modelDef *domain.ModelDefinition
			if cfg.ModelRegistry != nil {
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
