package http

import (
	"log"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/gin-gonic/gin"
)

// === Dashboard Model List (different from OpenAI /v1/models) ===

func apiListModels(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerFilter := c.Query("provider")
		policy := extractAPIKeyPolicy(c)
		var allModels []gin.H

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}

		if cfg != nil {
			seen := make(map[string]bool)
			reachableModels := make(map[string]bool)
			// Track which connections each model is available on
			modelConnections := make(map[string][]gin.H)

			// Build from active connections' SupportedModels
			for _, conn := range cfg.ProviderConnections {
				if !conn.IsActive {
					continue
				}
				if !service.ConnectionAllowed(conn.ID, policy) {
					continue
				}
				if providerFilter != "" && conn.Provider != providerFilter {
					continue
				}

				connInfo := gin.H{
					"id":       conn.ID,
					"name":     conn.Name,
					"provider": conn.Provider,
					"isActive": conn.IsActive,
				}

				if len(conn.SupportedModels) == 0 {
					// No restriction — add all registry models for this provider
					if cfg.ModelRegistry != nil {
						for key, m := range cfg.ModelRegistry.Models {
							if m.Provider == conn.Provider && m.IsActive {
								reachableModels[key] = true
							}
							if !service.ModelAllowedByPolicy(key, policy) {
								continue
							}
							if m.Provider == conn.Provider && m.IsActive && !seen[key] {
								seen[key] = true
								modelConnections[key] = []gin.H{connInfo}
								allModels = append(allModels, gin.H{
									"id":              key,
									"name":            m.Name,
									"provider":        m.Provider,
									"contextWindow":   m.ContextWindow,
									"maxOutputTokens": m.MaxOutputTokens,
									"inputPrice":      m.InputPrice,
									"outputPrice":     m.OutputPrice,
									"capabilities":    m.Capabilities,
								})
							} else if m.Provider == conn.Provider && m.IsActive && seen[key] {
								// Model already seen, just add connection info
								modelConnections[key] = append(modelConnections[key], connInfo)
							}
						}
					}
				} else {
					for _, modelID := range conn.SupportedModels {
						key := conn.Provider + "/" + modelID
						reachableModels[key] = true
						if !service.ModelAllowedByPolicy(key, policy) {
							continue
						}
						entry := gin.H{
							"id":       key,
							"name":     modelID,
							"provider": conn.Provider,
						}
						// Enrich with registry metadata if available
						if cfg.ModelRegistry != nil {
							if m := cfg.ModelRegistry.GetModel(key); m != nil {
								entry["name"] = m.Name
								entry["contextWindow"] = m.ContextWindow
								entry["maxOutputTokens"] = m.MaxOutputTokens
								entry["inputPrice"] = m.InputPrice
								entry["outputPrice"] = m.OutputPrice
								entry["capabilities"] = m.Capabilities
							}
						}

						if !seen[key] {
							seen[key] = true
							modelConnections[key] = []gin.H{connInfo}
							allModels = append(allModels, entry)
						} else {
							// Model already seen, just add connection info
							modelConnections[key] = append(modelConnections[key], connInfo)
						}
					}
				}
			}

			// Fallback: no connections → show full registry
			if len(cfg.ProviderConnections) == 0 && cfg.ModelRegistry != nil {
				for key, m := range cfg.ModelRegistry.Models {
					if !m.IsActive {
						continue
					}
					reachableModels[key] = true
					if providerFilter != "" && m.Provider != providerFilter {
						continue
					}
					if !service.ModelAllowedByPolicy(key, policy) {
						continue
					}
					if !seen[key] {
						seen[key] = true
						allModels = append(allModels, gin.H{
							"id":              key,
							"name":            m.Name,
							"provider":        m.Provider,
							"contextWindow":   m.ContextWindow,
							"maxOutputTokens": m.MaxOutputTokens,
							"inputPrice":      m.InputPrice,
							"outputPrice":     m.OutputPrice,
							"capabilities":    m.Capabilities,
						})
					}
				}
			}

			// Add connection info to each model
			for i, model := range allModels {
				if modelID, ok := model["id"].(string); ok {
					if conns, exists := modelConnections[modelID]; exists {
						allModels[i]["connections"] = conns
					}
				}
			}

			// Add aliases
			for alias, model := range cfg.ModelAliases {
				if !aliasAllowedByPolicy(alias, model, reachableModels, policy) {
					continue
				}
				allModels = append(allModels, gin.H{
					"id":       alias,
					"name":     alias + " → " + model,
					"provider": "alias",
					"target":   model,
				})
			}

			// Add combos
			for _, combo := range cfg.Combos {
				if !comboAllowedByPolicy(combo.Models, combo.Name, reachableModels, policy) {
					continue
				}
				allModels = append(allModels, gin.H{
					"id":       combo.Name,
					"name":     combo.Name,
					"provider": "combo",
					"models":   combo.Models,
				})
			}
		}

		c.JSON(200, allModels)
	}
}

func aliasAllowedByPolicy(alias string, target string, directModels map[string]bool, policy *port.APIKeyPolicy) bool {
	if policy == nil {
		return true
	}
	if len(policy.AllowedConnectionIDs) > 0 && !directModels[service.StripConnectionPin(service.NormalizeModelPolicyString(target))] {
		return false
	}
	if len(policy.AllowedModels) > 0 {
		for _, allowed := range policy.AllowedModels {
			if allowed == alias || service.ModelPolicyMatch(target, allowed) {
				return true
			}
		}
		return false
	}
	return true
}

func comboAllowedByPolicy(models []string, comboName string, directModels map[string]bool, policy *port.APIKeyPolicy) bool {
	if policy == nil {
		return true
	}
	for _, allowed := range policy.AllowedModels {
		if allowed == comboName {
			if len(policy.AllowedConnectionIDs) == 0 {
				return true
			}
			return comboHasReachableMember(models, directModels)
		}
	}
	for _, model := range models {
		normalized := service.NormalizeModelPolicyString(model)
		if directModels[service.StripConnectionPin(normalized)] && service.ModelAllowedByPolicy(normalized, policy) {
			return true
		}
	}
	return false
}

func comboHasReachableMember(models []string, directModels map[string]bool) bool {
	for _, model := range models {
		normalized := service.NormalizeModelPolicyString(model)
		if directModels[service.StripConnectionPin(normalized)] {
			return true
		}
	}
	return false
}

// === Model Registry Management ===

func apiGetModelRegistry(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}

		if cfg.ModelRegistry == nil {
			cfg.ModelRegistry = domain.DefaultModelRegistry()
		}

		c.JSON(200, cfg.ModelRegistry)
	}
}

func apiAddModelDefinition(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Key   string                 `json:"key"` // e.g. "kiro/my-model"
			Model domain.ModelDefinition `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
			return
		}

		if req.Key == "" {
			c.JSON(400, gin.H{"error": "Key is required"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}

		if cfg.ModelRegistry == nil {
			cfg.ModelRegistry = domain.DefaultModelRegistry()
		}

		// Check if model already exists
		if cfg.ModelRegistry.GetModel(req.Key) != nil {
			c.JSON(400, gin.H{"error": "Model already exists"})
			return
		}

		cfg.ModelRegistry.AddOrUpdateModel(req.Key, &req.Model)

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		log.Printf("[MODEL] Added model definition: %s", req.Key)
		c.JSON(200, gin.H{"ok": true, "key": req.Key})
	}
}

func apiUpdateModelDefinition(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		if key == "" {
			c.JSON(400, gin.H{"error": "Key is required"})
			return
		}

		// Handle provider/model format in URL params
		if p := c.Param("provider"); p != "" {
			key = p + "/" + key
		}

		var model domain.ModelDefinition
		if err := c.ShouldBindJSON(&model); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}

		if cfg.ModelRegistry == nil {
			cfg.ModelRegistry = domain.DefaultModelRegistry()
		}

		// Check if model exists
		if cfg.ModelRegistry.GetModel(key) == nil {
			c.JSON(404, gin.H{"error": "Model not found"})
			return
		}

		cfg.ModelRegistry.AddOrUpdateModel(key, &model)

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		log.Printf("[MODEL] Updated model definition: %s", key)
		c.JSON(200, gin.H{"ok": true, "key": key})
	}
}

func apiDeleteModelDefinition(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		if key == "" {
			c.JSON(400, gin.H{"error": "Key is required"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}

		if cfg.ModelRegistry == nil {
			c.JSON(404, gin.H{"error": "Model registry not found"})
			return
		}

		// Check if model exists
		if cfg.ModelRegistry.GetModel(key) == nil {
			c.JSON(404, gin.H{"error": "Model not found"})
			return
		}

		cfg.ModelRegistry.RemoveModel(key)

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		log.Printf("[MODEL] Deleted model definition: %s", key)
		c.JSON(200, gin.H{"ok": true, "key": key})
	}
}
