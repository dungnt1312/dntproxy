package domain

import (
	"sort"
	"strings"
)

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
			"kiro/claude-opus-4.6": {
				ID:              "claude-opus-4.6",
				Name:            "Claude Opus 4.6",
				Provider:        "kiro",
				ContextWindow:   200000,
				MaxOutputTokens: 8192,
				InputPrice:      15.0,
				OutputPrice:     75.0,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"kiro/claude-opus-4.5": {
				ID:              "claude-opus-4.5",
				Name:            "Claude Opus 4.5",
				Provider:        "kiro",
				ContextWindow:   200000,
				MaxOutputTokens: 8192,
				InputPrice:      15.0,
				OutputPrice:     75.0,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"kiro/claude-sonnet-4.6": {
				ID:              "claude-sonnet-4.6",
				Name:            "Claude Sonnet 4.6",
				Provider:        "kiro",
				ContextWindow:   200000,
				MaxOutputTokens: 8192,
				InputPrice:      3.0,
				OutputPrice:     15.0,
				Capabilities:    []string{"vision", "tools", "streaming", "thinking"},
				IsActive:        true,
			},
			"kiro/deepseek-3.1": {
				ID:              "deepseek-3.1",
				Name:            "DeepSeek 3.1",
				Provider:        "kiro",
				ContextWindow:   128000,
				MaxOutputTokens: 8192,
				InputPrice:      0.14,
				OutputPrice:     0.28,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			// OpenAI models
			"openai/gpt-5.4": {
				ID:              "gpt-5.4",
				Name:            "GPT-5.4",
				Provider:        "openai",
				ContextWindow:   400000,
				MaxOutputTokens: 128000,
				InputPrice:      1.25,
				OutputPrice:     10.0,
				Capabilities:    []string{"reasoning", "vision", "tools", "streaming"},
				IsActive:        true,
			},
			"openai/gpt-5.4-mini": {
				ID:              "gpt-5.4-mini",
				Name:            "GPT-5.4 Mini",
				Provider:        "openai",
				ContextWindow:   400000,
				MaxOutputTokens: 128000,
				InputPrice:      0.25,
				OutputPrice:     2.0,
				Capabilities:    []string{"reasoning", "vision", "tools", "streaming"},
				IsActive:        true,
			},
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
			// GLM (Zhipu AI / Z.ai) models
			"glm/glm-5.1": {
				ID:              "glm-5.1",
				Name:            "GLM 5.1",
				Provider:        "glm",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      1.4,
				OutputPrice:     4.4,
				Capabilities:    []string{"vision", "tools", "streaming", "thinking"},
				IsActive:        true,
			},
			"glm/glm-5": {
				ID:              "glm-5",
				Name:            "GLM 5",
				Provider:        "glm",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      1.4,
				OutputPrice:     4.4,
				Capabilities:    []string{"vision", "tools", "streaming", "thinking"},
				IsActive:        true,
			},
			"glm/glm-4.7": {
				ID:              "glm-4.7",
				Name:            "GLM 4.7",
				Provider:        "glm",
				ContextWindow:   128000,
				MaxOutputTokens: 8192,
				InputPrice:      1.0,
				OutputPrice:     3.0,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"glm/glm-4.6": {
				ID:              "glm-4.6",
				Name:            "GLM 4.6",
				Provider:        "glm",
				ContextWindow:   128000,
				MaxOutputTokens: 8192,
				InputPrice:      0.8,
				OutputPrice:     2.5,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"glm/glm-5-turbo": {
				ID:              "glm-5-turbo",
				Name:            "GLM 5 Turbo",
				Provider:        "glm",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      1.4,
				OutputPrice:     4.4,
				Capabilities:    []string{"vision", "tools", "streaming", "thinking"},
				IsActive:        true,
			},
			"glm/glm-5V-turbo": {
				ID:              "glm-5V-turbo",
				Name:            "GLM 5V Turbo",
				Provider:        "glm",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      1.4,
				OutputPrice:     4.4,
				Capabilities:    []string{"vision", "tools", "streaming", "thinking"},
				IsActive:        true,
			},
			"glm/glm-4.7-flash": {
				ID:              "glm-4.7-flash",
				Name:            "GLM 4.7 Flash",
				Provider:        "glm",
				ContextWindow:   128000,
				MaxOutputTokens: 8192,
				InputPrice:      0.5,
				OutputPrice:     1.5,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			// MiniMax models
			"minimax/MiniMax-M2.7": {
				ID:              "MiniMax-M2.7",
				Name:            "MiniMax M2.7",
				Provider:        "minimax",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      1.0,
				OutputPrice:     4.0,
				Capabilities:    []string{"tools", "streaming", "thinking"},
				IsActive:        true,
			},
			"minimax/MiniMax-M2.7-highspeed": {
				ID:              "MiniMax-M2.7-highspeed",
				Name:            "MiniMax M2.7 HighSpeed",
				Provider:        "minimax",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      0.7,
				OutputPrice:     2.8,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			"minimax/MiniMax-M2.5": {
				ID:              "MiniMax-M2.5",
				Name:            "MiniMax M2.5",
				Provider:        "minimax",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      0.8,
				OutputPrice:     3.2,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			"minimax/MiniMax-M2.1": {
				ID:              "MiniMax-M2.1",
				Name:            "MiniMax M2.1",
				Provider:        "minimax",
				ContextWindow:   128000,
				MaxOutputTokens: 16384,
				InputPrice:      0.5,
				OutputPrice:     2.0,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			"minimax/MiniMax-M2": {
				ID:              "MiniMax-M2",
				Name:            "MiniMax M2",
				Provider:        "minimax",
				ContextWindow:   128000,
				MaxOutputTokens: 8192,
				InputPrice:      0.3,
				OutputPrice:     1.2,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			// Qwen models (Alibaba Cloud)
			"qwen/qwen3-coder-plus": {
				ID:              "qwen3-coder-plus",
				Name:            "Qwen3 Coder Plus",
				Provider:        "qwen",
				ContextWindow:   131072,
				MaxOutputTokens: 16384,
				InputPrice:      0.0,
				OutputPrice:     0.0,
				Capabilities:    []string{"tools", "streaming", "thinking"},
				IsActive:        true,
			},
			"qwen/qwen3-coder": {
				ID:              "qwen3-coder",
				Name:            "Qwen3 Coder",
				Provider:        "qwen",
				ContextWindow:   131072,
				MaxOutputTokens: 16384,
				InputPrice:      0.0,
				OutputPrice:     0.0,
				Capabilities:    []string{"tools", "streaming", "thinking"},
				IsActive:        true,
			},
			"qwen/qwen-plus": {
				ID:              "qwen-plus",
				Name:            "Qwen Plus",
				Provider:        "qwen",
				ContextWindow:   131072,
				MaxOutputTokens: 16384,
				InputPrice:      0.0,
				OutputPrice:     0.0,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			"qwen/qwen-turbo": {
				ID:              "qwen-turbo",
				Name:            "Qwen Turbo",
				Provider:        "qwen",
				ContextWindow:   131072,
				MaxOutputTokens: 8192,
				InputPrice:      0.0,
				OutputPrice:     0.0,
				Capabilities:    []string{"tools", "streaming"},
				IsActive:        true,
			},
			// Anthropic models
			"anthropic/claude-sonnet": {
				ID:              "claude-sonnet",
				Name:            "Claude Sonnet",
				Provider:        "anthropic",
				ContextWindow:   200000,
				MaxOutputTokens: 8192,
				InputPrice:      3.0,
				OutputPrice:     15.0,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"anthropic/claude-opus": {
				ID:              "claude-opus",
				Name:            "Claude Opus",
				Provider:        "anthropic",
				ContextWindow:   200000,
				MaxOutputTokens: 8192,
				InputPrice:      15.0,
				OutputPrice:     75.0,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			"anthropic/claude-haiku": {
				ID:              "claude-haiku",
				Name:            "Claude Haiku",
				Provider:        "anthropic",
				ContextWindow:   200000,
				MaxOutputTokens: 8192,
				InputPrice:      0.8,
				OutputPrice:     4.0,
				Capabilities:    []string{"vision", "tools", "streaming"},
				IsActive:        true,
			},
			// Gemini models
			"gemini/gemini-2.5-flash": {
				ID:              "gemini-2.5-flash",
				Name:            "Gemini 2.5 Flash",
				Provider:        "gemini",
				ContextWindow:   1000000,
				MaxOutputTokens: 8192,
				InputPrice:      0.15,
				OutputPrice:     0.6,
				Capabilities:    []string{"vision", "tools", "streaming"},
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

// Deprecated: Use GetProviderConfig("kiro").DefaultModels instead.
func DefaultKiroModels() []string {
	return GetProviderConfig("kiro").DefaultModels
}

// Deprecated: Use GetProviderConfig("glm").DefaultModels instead.
func DefaultGLMModels() []string {
	return GetProviderConfig("glm").DefaultModels
}

// Deprecated: Use GetProviderConfig("minimax").DefaultModels instead.
func DefaultMiniMaxModels() []string {
	return GetProviderConfig("minimax").DefaultModels
}

// Deprecated: Use GetProviderConfig("qwen").DefaultModels instead.
func DefaultQwenModels() []string {
	return GetProviderConfig("qwen").DefaultModels
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

// GetDefaultModelsForProvider returns default model IDs for a provider from the model registry.
func GetDefaultModelsForProvider(providerID string) []string {
	registry := DefaultModelRegistry()
	var models []string
	for key, m := range registry.Models {
		if m.Provider == providerID && m.IsActive {
			parts := strings.Split(key, "/")
			if len(parts) == 2 {
				models = append(models, parts[1])
			}
		}
	}
	sort.Strings(models)
	return models
}
