package domain

// ModelDefinition represents metadata about a model.
type ModelDefinition struct {
	ID              string                 `json:"id"`              // e.g. "claude-sonnet-4.5"
	Name            string                 `json:"name"`            // Display name
	Provider        string                 `json:"provider"`        // "kiro", "openai", etc.
	ContextWindow   int                    `json:"contextWindow"`   // Max tokens
	MaxOutputTokens int                    `json:"maxOutputTokens"` // Max output
	InputPrice      float64                `json:"inputPrice"`      // Per 1M tokens
	OutputPrice     float64                `json:"outputPrice"`     // Per 1M tokens
	Capabilities    []string               `json:"capabilities"`    // ["vision", "tools", "streaming"]
	IsActive        bool                   `json:"isActive"`        // Available for use
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ModelRegistry holds all available model definitions.
type ModelRegistry struct {
	Models map[string]*ModelDefinition `json:"models"` // key: "provider/model-id"
}

// DefaultModelRegistry returns a registry with common models.
func DefaultModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		Models: map[string]*ModelDefinition{
			// Kiro models
			"kiro/claude-sonnet-4.5": {
				ID:              "claude-sonnet-4.5",
				Name:            "Claude Sonnet 4.5",
				Provider:        "kiro",
				ContextWindow:   200000,
				MaxOutputTokens: 8192,
				InputPrice:      3.0,
				OutputPrice:     15.0,
				Capabilities:    []string{"vision", "tools", "streaming", "thinking"},
				IsActive:        true,
			},
			"kiro/claude-haiku-4.5": {
				ID:              "claude-haiku-4.5",
				Name:            "Claude Haiku 4.5",
				Provider:        "kiro",
				ContextWindow:   200000,
				MaxOutputTokens: 8192,
				InputPrice:      0.8,
				OutputPrice:     4.0,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"kiro/deepseek-3.2": {
				ID:              "deepseek-3.2",
				Name:            "DeepSeek 3.2",
				Provider:        "kiro",
				ContextWindow:   128000,
				MaxOutputTokens: 8192,
				InputPrice:      0.14,
				OutputPrice:     0.28,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			"kiro/deepseek-3.1": {
				ID:              "deepseek-3.1",
				Name:            "DeepSeek 3.1",
				Provider:        "kiro",
				ContextWindow:   64000,
				MaxOutputTokens: 8192,
				InputPrice:      0.27,
				OutputPrice:     1.1,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			"kiro/qwen3-coder-next": {
				ID:              "qwen3-coder-next",
				Name:            "Qwen3 Coder Next",
				Provider:        "kiro",
				ContextWindow:   32000,
				MaxOutputTokens: 8192,
				InputPrice:      0.0,
				OutputPrice:     0.0,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			// OpenAI models
			"openai/gpt-4.1": {
				ID:              "gpt-4.1",
				Name:            "GPT-4.1",
				Provider:        "openai",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      2.5,
				OutputPrice:     10.0,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"openai/gpt-4.1-mini": {
				ID:              "gpt-4.1-mini",
				Name:            "GPT-4.1 Mini",
				Provider:        "openai",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      0.15,
				OutputPrice:     0.6,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"openai/gpt-4.1-nano": {
				ID:              "gpt-4.1-nano",
				Name:            "GPT-4.1 Nano",
				Provider:        "openai",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      0.04,
				OutputPrice:     0.16,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			"openai/gpt-4o": {
				ID:              "gpt-4o",
				Name:            "GPT-4o",
				Provider:        "openai",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      2.5,
				OutputPrice:     10.0,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"openai/gpt-4o-mini": {
				ID:              "gpt-4o-mini",
				Name:            "GPT-4o Mini",
				Provider:        "openai",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      0.15,
				OutputPrice:     0.6,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"openai/o3": {
				ID:              "o3",
				Name:            "o3",
				Provider:        "openai",
				ContextWindow:   128000,
				MaxOutputTokens: 100000,
				InputPrice:      10.0,
				OutputPrice:     40.0,
				Capabilities:    []string{"reasoning", "tools", "streaming"},
				IsActive:        true,
			},
			"openai/o3-mini": {
				ID:              "o3-mini",
				Name:            "o3-mini",
				Provider:        "openai",
				ContextWindow:   128000,
				MaxOutputTokens: 65536,
				InputPrice:      1.1,
				OutputPrice:     4.4,
				Capabilities:    []string{"reasoning", "tools", "streaming"},
				IsActive:        true,
			},
			"openai/o4-mini": {
				ID:              "o4-mini",
				Name:            "o4-mini",
				Provider:        "openai",
				ContextWindow:   128000,
				MaxOutputTokens: 65536,
				InputPrice:      1.1,
				OutputPrice:     4.4,
				Capabilities:    []string{"reasoning", "tools", "streaming"},
				IsActive:        true,
			},
		},
	}
}

// GetModel returns a model definition by full key (provider/model-id).
func (r *ModelRegistry) GetModel(key string) *ModelDefinition {
	if r.Models == nil {
		return nil
	}
	return r.Models[key]
}

// DefaultKiroModels returns the default model IDs for Kiro connections.
func DefaultKiroModels() []string {
	registry := DefaultModelRegistry()
	var models []string
	for key, m := range registry.Models {
		if m.Provider == "kiro" && m.IsActive {
			// Strip "kiro/" prefix to get bare model ID
			_ = key
			models = append(models, m.ID)
		}
	}
	return models
}

// GetModelsByProvider returns all models for a given provider.
func (r *ModelRegistry) GetModelsByProvider(provider string) []*ModelDefinition {
	var result []*ModelDefinition
	for _, model := range r.Models {
		if model.Provider == provider && model.IsActive {
			result = append(result, model)
		}
	}
	return result
}

// AddOrUpdateModel adds or updates a model definition.
func (r *ModelRegistry) AddOrUpdateModel(key string, model *ModelDefinition) {
	if r.Models == nil {
		r.Models = make(map[string]*ModelDefinition)
	}
	r.Models[key] = model
}

// RemoveModel removes a model definition.
func (r *ModelRegistry) RemoveModel(key string) {
	if r.Models != nil {
		delete(r.Models, key)
	}
}
