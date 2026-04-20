package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// ParsedModel represents a parsed model string with optional connection pinning.
type ParsedModel struct {
	Provider     string
	Model        string
	ConnectionID string // empty if not pinned
}

// ParseModelString parses "provider/model@connectionId" format.
// Format constraints:
//   - provider: single segment (no slashes)
//   - model: can contain slashes (e.g., "glm-5.1" or "v1/gpt-4")
//   - connectionId: optional, UUID or "auto"
// Examples:
//   - "kr/opus@conn-123" → {Provider: "kiro", Model: "opus", ConnectionID: "conn-123"}
//   - "kr/opus@auto" → {Provider: "kiro", Model: "opus", ConnectionID: ""}
//   - "kr/opus" → {Provider: "kiro", Model: "opus", ConnectionID: ""}
//   - "glm/glm-5.1" → {Provider: "glm", Model: "glm-5.1", ConnectionID: ""}
func ParseModelString(modelStr string) (*ParsedModel, error) {
	// Split @connectionId (only first @)
	atIdx := strings.Index(modelStr, "@")
	modelPart := modelStr
	var connID string
	if atIdx >= 0 {
		modelPart = modelStr[:atIdx]
		connID = modelStr[atIdx+1:]
		// Normalize "auto" to empty (explicit auto-select)
		if connID == "auto" {
			connID = ""
		}
	}

	// Parse provider/model
	idx := strings.Index(modelPart, "/")
	if idx < 0 {
		return nil, fmt.Errorf("invalid model format: %s (expected provider/model)", modelStr)
	}

	providerOrAlias := modelPart[:idx]
	model := modelPart[idx+1:]
	provider := resolveProviderAlias(providerOrAlias)

	return &ParsedModel{
		Provider:     provider,
		Model:        model,
		ConnectionID: connID,
	}, nil
}

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

// normalizeModelStr converts "alias/model@connectionId" to "provider/model@connectionId" using ProviderAliasToID.
// Preserves @connectionId suffix if present.
// Detects and fixes duplicate provider prefix (e.g., "glm/glm/glm-5.1" -> "glm/glm-5.1").
func (r *ModelResolver) normalizeModelStr(modelStr string) (string, error) {
	parsed, err := ParseModelString(modelStr)
	if err != nil {
		return "", err
	}

	// Detect duplicate prefix: if model starts with "provider/", strip it
	// Example: provider="glm", model="glm/glm-5.1" -> model="glm-5.1"
	if strings.HasPrefix(parsed.Model, parsed.Provider+"/") {
		parsed.Model = strings.TrimPrefix(parsed.Model, parsed.Provider+"/")
	}

	// Return normalized format (keep @connectionId if present)
	result := parsed.Provider + "/" + parsed.Model
	if parsed.ConnectionID != "" {
		result += "@" + parsed.ConnectionID
	}
	return result, nil
}

// Resolve parses a model string and returns provider + model.
// Supports: "alias/model", "provider/model", combo names, model aliases.
// Strips @connectionId suffix if present.
func (r *ModelResolver) Resolve(modelStr string) (*domain.ModelInfo, error) {
	// Strip @connectionId suffix before processing
	atIdx := strings.Index(modelStr, "@")
	if atIdx >= 0 {
		modelStr = modelStr[:atIdx]
	}

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
			// Strip @connectionId from alias value
			if atIdx := strings.Index(resolved, "@"); atIdx >= 0 {
				resolved = resolved[:atIdx]
			}

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
