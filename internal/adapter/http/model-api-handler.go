package http

import (
	"log"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/gin-gonic/gin"
)

// === Provider Metadata + Dashboard Model List ===

// apiListProviders returns rich metadata for all registered providers.
// Used by frontend to render dynamic Add Connection forms.
func apiListProviders() gin.HandlerFunc {
	return func(c *gin.Context) {
		metadata := domain.GetAllProviderMetadata()
		c.JSON(200, metadata)
	}
}

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
			reachablePublicModels := make(map[string]bool)
			// Track which connections each model is available on
			modelConnections := make(map[string][]gin.H)

			// Build from active connections' SupportedModels
			tenantID := GetTenantID(c)
			conns := domain.FilterConnectionsByTenant(cfg.ProviderConnections, tenantID)
			for _, conn := range conns {
				if !conn.IsActive {
					continue
				}
				if !service.ConnectionAllowed(conn.ID, policy) {
					continue
				}
				modelProvider := modelProviderForConnection(conn)
				if providerFilter != "" && conn.Provider != providerFilter && modelProvider != providerFilter {
					continue
				}

				connInfo := gin.H{
					"id":          conn.ID,
					"name":        conn.Name,
					"provider":    conn.Provider,
					"routePrefix": conn.RoutePrefix,
					"isActive":    conn.IsActive,
				}

				// Runtime fill: use RecommendedModels if SupportedModels is empty
				// This ensures the model list shown in UI respects the curated defaults
				effectiveModels := conn.SupportedModels
				if len(effectiveModels) == 0 {
					provCfg := domain.GetProviderConfig(conn.Provider)
					if len(provCfg.RecommendedModels) > 0 {
						effectiveModels = provCfg.RecommendedModels
					}
				}

				if len(effectiveModels) == 0 {
					// No restriction — add all registry models for this provider
					if cfg.ModelRegistry != nil {
						for key, m := range cfg.ModelRegistry.Models {
							if m.Provider != conn.Provider || !m.IsActive {
								continue
							}
							publicKey := key
							publicModel := m.Name
							if modelProvider != conn.Provider {
								publicKey = modelProvider + "/" + key[len(m.Provider)+1:]
							}
							reachableModels[publicKey] = true
							reachablePublicModels[publicKey] = true
							if !service.ModelAllowedByPolicy(publicKey, policy) {
								continue
							}
							if !seen[publicKey] {
								seen[publicKey] = true
								modelConnections[publicKey] = []gin.H{connInfo}
								allModels = append(allModels, gin.H{
									"id":              publicKey,
									"name":            publicModel,
									"provider":        modelProvider,
									"routePrefix":     conn.RoutePrefix,
									"contextWindow":   m.ContextWindow,
									"maxOutputTokens": m.MaxOutputTokens,
									"inputPrice":      m.InputPrice,
									"outputPrice":     m.OutputPrice,
									"capabilities":    m.Capabilities,
								})
							} else {
								// Model already seen, just add connection info
								modelConnections[publicKey] = append(modelConnections[publicKey], connInfo)
							}
						}
					}
				} else {
					for _, modelID := range effectiveModels {
						key := modelProvider + "/" + modelID
						reachableModels[key] = true
						reachablePublicModels[key] = true
						if !service.ModelAllowedByPolicy(key, policy) {
							continue
						}
						entry := gin.H{
							"id":          key,
							"name":        modelID,
							"provider":    modelProvider,
							"routePrefix": conn.RoutePrefix,
						}
						// Enrich with registry metadata if available
						if cfg.ModelRegistry != nil {
							if m := cfg.ModelRegistry.GetModel(conn.Provider + "/" + modelID); m != nil {
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
				if !aliasAllowedByPolicy(alias, model, reachableModels, reachablePublicModels, policy) {
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
				if !comboAllowedByPolicy(combo.Models, combo.Name, reachableModels, reachablePublicModels, policy) {
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

func aliasAllowedByPolicy(alias string, target string, directModels map[string]bool, publicModels map[string]bool, policy *port.APIKeyPolicy) bool {
	if policy == nil {
		return true
	}
	if len(policy.AllowedConnectionIDs) > 0 && !hasReachableModel(target, directModels, publicModels) {
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

func comboAllowedByPolicy(models []string, comboName string, directModels map[string]bool, publicModels map[string]bool, policy *port.APIKeyPolicy) bool {
	if policy == nil {
		return true
	}
	for _, allowed := range policy.AllowedModels {
		if allowed == comboName {
			if len(policy.AllowedConnectionIDs) == 0 {
				return true
			}
			return comboHasReachableMember(models, directModels, publicModels)
		}
	}
	for _, model := range models {
		normalized := service.NormalizeModelPolicyString(model)
		if hasReachableModel(model, directModels, publicModels) && service.ModelAllowedByPolicy(normalized, policy) {
			return true
		}
	}
	return false
}

func comboHasReachableMember(models []string, directModels map[string]bool, publicModels map[string]bool) bool {
	for _, model := range models {
		if hasReachableModel(model, directModels, publicModels) {
			return true
		}
	}
	return false
}

func hasReachableModel(model string, directModels map[string]bool, publicModels map[string]bool) bool {
	if directModels[service.StripConnectionPin(model)] {
		return true
	}
	normalized := service.NormalizeModelPolicyString(model)
	stripped := service.StripConnectionPin(normalized)
	if directModels[stripped] {
		return true
	}
	if publicModels == nil {
		return false
	}
	parsed, err := service.ParseModelString(stripped)
	if err == nil && parsed.Provider == "openai-compatible" {
		for key := range directModels {
			keyParsed, keyErr := service.ParseModelString(key)
			if keyErr == nil && keyParsed.Provider != "openai-compatible" && keyParsed.Model == parsed.Model && publicModels[key] {
				return true
			}
		}
	}
	return false
}

func modelProviderForConnection(conn domain.ProviderConnection) string {
	if conn.Provider == "openai-compatible" {
		prefix := domain.NormalizeRoutePrefix(conn.RoutePrefix)
		if prefix == "" {
			prefix = domain.NormalizeRoutePrefix(conn.Name)
		}
		if prefix != "" {
			return prefix
		}
	}
	return conn.Provider
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

		// Support filtering by provider for the "Edit Models" modal.
		// When ?provider=openai is passed, only return models for that provider.
		// This prevents the modal from showing outdated models (gpt-4.1, gpt-4o, o3, etc.)
		// that are not in the RecommendedModels list.
		providerFilter := c.Query("provider")
		if providerFilter != "" {
			filtered := &domain.ModelRegistry{
				Models: make(map[string]*domain.ModelDefinition),
			}
			for key, m := range cfg.ModelRegistry.Models {
				if m.Provider == providerFilter && m.IsActive {
					filtered.Models[key] = m
				}
			}
			c.JSON(200, filtered)
			return
		}

		c.JSON(200, cfg.ModelRegistry)
	}
}

func apiAddModelDefinition(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
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
		if !requireAdmin(c) {
			return
		}
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
		if !requireAdmin(c) {
			return
		}
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
