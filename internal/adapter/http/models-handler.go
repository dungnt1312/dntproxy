package http

import (
	"net/http"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/gin-gonic/gin"
)

// OpenAI-compatible model list response.
type modelObject struct {
	ID                string                    `json:"id"`
	Object            string                    `json:"object"`
	Created           int64                     `json:"created"`
	OwnedBy           string                    `json:"owned_by"`
	Capabilities      []string                  `json:"capabilities,omitempty"`
	ImageCapabilities *domain.ImageCapabilities `json:"image_capabilities,omitempty"`
}

func modelsHandler(modelAccess *service.ModelAccessService, store port.CredentialStore, registries ...port.ImageProviderRegistry) gin.HandlerFunc {
	var imageProviders port.ImageProviderRegistry
	if len(registries) > 0 {
		imageProviders = registries[0]
	}
	return func(c *gin.Context) {
		policy := extractAPIKeyPolicy(c)
		tenantID := GetTenantID(c)
		modelType := strings.TrimSpace(c.Query("type")) // e.g. "image"

		pool, err := modelAccess.BuildPoolForTenant(policy, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build model pool"})
			return
		}

		// Load model registry for capability lookup
		cfg, _ := store.Load()
		var registry map[string]*domain.ModelDefinition
		if cfg != nil && cfg.ModelRegistry != nil {
			registry = cfg.ModelRegistry.Models
		}

		var models []modelObject

		for _, m := range pool.Models {
			// Find capabilities from registry
			caps := lookupCapabilities(m.Provider, m.Model, registry)
			var runtimeImageCaps *domain.ImageCapabilities
			if imageProviders != nil {
				if imageProvider := imageProviders.GetImageProvider(m.Provider); imageProvider != nil {
					resolved := imageProvider.Capabilities(m.Model)
					if resolved.Generate || resolved.Edit {
						runtimeImageCaps = &resolved
					}
				}
			}

			// Filter by type if requested
			if modelType == "image" && !hasCapability(caps, "image-generation") && runtimeImageCaps == nil {
				continue
			}

			ownedBy := m.DisplayProvider
			if ownedBy == "" {
				ownedBy = m.Provider
			}
			entry := newModelObject(m.QualifiedID, ownedBy, caps)
			entry.ImageCapabilities = runtimeImageCaps
			models = append(models, entry)
		}

		// Combos and aliases are only included when no type filter is set
		if modelType == "" {
			for _, combo := range pool.Combos {
				models = append(models, newModelObject(combo.Name, "combo", nil))
			}
			for _, alias := range pool.Aliases {
				models = append(models, newModelObject(alias.Name, "alias", nil))
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   models,
		})
	}
}

func newModelObject(id string, ownedBy string, capabilities []string) modelObject {
	return modelObject{
		ID:           id,
		Object:       "model",
		Created:      1700000000,
		OwnedBy:      ownedBy,
		Capabilities: capabilities,
	}
}

// lookupCapabilities finds capabilities for a model from the registry.
func lookupCapabilities(provider, model string, registry map[string]*domain.ModelDefinition) []string {
	if registry == nil {
		return nil
	}
	key := provider + "/" + model
	if entry, ok := registry[key]; ok {
		return entry.Capabilities
	}
	return nil
}

// hasCapability checks if capabilities list contains the target.
func hasCapability(caps []string, target string) bool {
	for _, c := range caps {
		if strings.EqualFold(c, target) {
			return true
		}
	}
	return false
}
