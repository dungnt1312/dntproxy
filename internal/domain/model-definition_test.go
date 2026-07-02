package domain

import "testing"

func TestDefaultModelRegistryIncludesXAIModels(t *testing.T) {
	registry := DefaultModelRegistry()
	models := []string{
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

func TestDefaultXAIModelsIncludeExpandedRegistry(t *testing.T) {
	models := GetProviderConfig("xai").DefaultModels
	want := map[string]bool{
		"grok-build-0.1":               false,
		"grok-4.3":                     false,
		"grok-4.20-0309-reasoning":     false,
		"grok-4.20-0309-non-reasoning": false,
		"grok-4.20-multi-agent-0309":   false,
		"grok-3-mini":                  false,
		"grok-3-mini-fast":             false,
	}
	for _, model := range models {
		if _, ok := want[model]; ok {
			want[model] = true
		}
	}
	for model, found := range want {
		if !found {
			t.Fatalf("default xai models missing %q; got %#v", model, models)
		}
	}
}
