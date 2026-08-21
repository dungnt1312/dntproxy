package domain

import "testing"

func TestDefaultModelRegistryIncludesXAIModels(t *testing.T) {
	registry := DefaultModelRegistry()
	models := []string{
		"xai/grok-4.6",
		"xai/grok-4.5",
		"xai/grok-build-0.1",
		"xai/grok-4.3",
		"xai/grok-4.20-0309-reasoning",
		"xai/grok-4.20-0309-non-reasoning",
		"xai/grok-4.20-multi-agent-0309",
		"xai/grok-3-mini",
		"xai/grok-3-mini-fast",
	}

	for _, model := range models {
		def := registry.GetModel(model)
		if def == nil {
			t.Fatalf("missing model definition %q", model)
		}
		if def.Provider != "xai" {
			t.Fatalf("model %q provider = %q, want xai", model, def.Provider)
		}
		if !def.IsActive {
			t.Fatalf("model %q should be active", model)
		}
	}
}

// TestDefaultXAIModelsIncludeExpandedRegistry verifies that the xAI curated
// RecommendedModels (= DefaultModels) contains at least the three primary models.
// Full registry coverage is tested by TestDefaultModelRegistryIncludesXAIModels.
func TestDefaultXAIModelsIncludeExpandedRegistry(t *testing.T) {
	models := GetProviderConfig("xai").DefaultModels
	// These are the primary models that must always be in the recommended list.
	required := []string{
		"grok-4.6",
		"grok-4.5",
		"grok-4.3",
	}
	modelSet := make(map[string]bool, len(models))
	for _, m := range models {
		modelSet[m] = true
	}
	for _, model := range required {
		if !modelSet[model] {
			t.Fatalf("xai RecommendedModels missing %q; got %#v", model, models)
		}
	}
	if len(models) == 0 {
		t.Fatalf("xai DefaultModels should not be empty")
	}
}

func TestDefaultModelRegistryIncludesCommandCodeModels(t *testing.T) {
	registry := DefaultModelRegistry()
	models := []string{
		"commandcode/deepseek/deepseek-v4-pro",
		"commandcode/deepseek/deepseek-v4-flash",
		"commandcode/MiniMaxAI/MiniMax-M3",
		"commandcode/Qwen/Qwen3.7-Max",
		"commandcode/xiaomi/mimo-v2.5-pro",
	}
	for _, model := range models {
		def := registry.GetModel(model)
		if def == nil {
			t.Fatalf("missing model definition %q", model)
		}
		if def.Provider != "commandcode" {
			t.Fatalf("model %q provider = %q, want commandcode", model, def.Provider)
		}
		if !def.IsActive {
			t.Fatalf("model %q should be active", model)
		}
	}
}

func TestPublicProviderPrefixCommandCode(t *testing.T) {
	if got := PublicProviderPrefix("commandcode"); got != "cmc" {
		t.Fatalf("PublicProviderPrefix(commandcode) = %q, want cmc", got)
	}
	if got := PublicProviderPrefix("kiro"); got != "kiro" {
		t.Fatalf("PublicProviderPrefix(kiro) = %q, want kiro", got)
	}
}

func TestCommandCodeRecommendedModels(t *testing.T) {
	cfg := GetProviderConfig("commandcode")
	if cfg.Format != FormatCommandCode {
		t.Fatalf("format = %q", cfg.Format)
	}
	if cfg.ChatPath != "/alpha/generate" {
		t.Fatalf("chat path = %q", cfg.ChatPath)
	}
	required := []string{"deepseek/deepseek-v4-pro", "MiniMaxAI/MiniMax-M3", "Qwen/Qwen3.7-Max"}
	have := map[string]bool{}
	for _, m := range cfg.RecommendedModels {
		have[m] = true
	}
	for _, model := range required {
		if !have[model] {
			t.Fatalf("RecommendedModels missing %q; got %#v", model, cfg.RecommendedModels)
		}
	}
}
