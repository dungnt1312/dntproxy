package domain

import "testing"

func TestMiniMaxImageModelRegistration(t *testing.T) {
	model := DefaultModelRegistry().GetModel("minimax/image-01")
	if model == nil {
		t.Fatal("minimax/image-01 is not registered")
	}
	if model.Provider != "minimax" || !model.IsActive {
		t.Fatalf("unexpected model registration: %#v", model)
	}
	foundCapability := false
	for _, capability := range model.Capabilities {
		if capability == "image-generation" {
			foundCapability = true
			break
		}
	}
	if !foundCapability {
		t.Fatalf("capabilities = %#v, want image-generation", model.Capabilities)
	}
}

func TestMiniMaxRecommendedModelsIncludeImage(t *testing.T) {
	config := GetProviderConfig("minimax")
	for _, model := range config.RecommendedModels {
		if model == "image-01" {
			return
		}
	}
	t.Fatalf("MiniMax recommended models = %#v, want image-01", config.RecommendedModels)
}
