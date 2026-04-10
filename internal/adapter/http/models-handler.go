package http

import (
	"net/http"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// OpenAI-compatible model list response.
type modelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func modelsHandler(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var models []modelObject

		cfg, err := store.Load()
		if err == nil && cfg != nil {
			seen := make(map[string]bool)

			activeConns := make([]interface{ GetProvider() string }, 0)
			_ = activeConns

			// Collect model IDs from active connections' SupportedModels
			// Key: "provider/modelID" — same format as registry
			for _, conn := range cfg.ProviderConnections {
				if !conn.IsActive {
					continue
				}
				if len(conn.SupportedModels) == 0 {
					// No restriction — add all registry models for this provider
					if cfg.ModelRegistry != nil {
						for key, m := range cfg.ModelRegistry.Models {
							if m.Provider == conn.Provider && m.IsActive && !seen[key] {
								seen[key] = true
								models = append(models, modelObject{
									ID:      key,
									Object:  "model",
									Created: 1700000000,
									OwnedBy: m.Provider,
								})
							}
						}
					}
				} else {
					for _, modelID := range conn.SupportedModels {
						key := conn.Provider + "/" + modelID
						if seen[key] {
							continue
						}
						seen[key] = true
						ownedBy := conn.Provider
						models = append(models, modelObject{
							ID:      key,
							Object:  "model",
							Created: 1700000000,
							OwnedBy: ownedBy,
						})
					}
				}
			}

			// If no connections at all, fall back to full registry
			if len(cfg.ProviderConnections) == 0 && cfg.ModelRegistry != nil {
				for key, m := range cfg.ModelRegistry.Models {
					if m.IsActive && !seen[key] {
						seen[key] = true
						models = append(models, modelObject{
							ID:      key,
							Object:  "model",
							Created: 1700000000,
							OwnedBy: m.Provider,
						})
					}
				}
			}
		}

		// Add combos as models
		combos, err := store.GetCombos()
		if err == nil {
			for _, combo := range combos {
				models = append(models, modelObject{
					ID:      combo.Name,
					Object:  "model",
					Created: 1700000000,
					OwnedBy: "combo",
				})
			}
		}

		// Add aliases as models
		aliases, err := store.GetModelAliases()
		if err == nil {
			for alias := range aliases {
				models = append(models, modelObject{
					ID:      alias,
					Object:  "model",
					Created: 1700000000,
					OwnedBy: "alias",
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   models,
		})
	}
}
