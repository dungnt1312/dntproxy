package http

import (
	"log"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// RegisterAPIRoutes adds all /api/* dashboard endpoints.
// Handler implementations are split across separate files:
//   - connection-handler.go  — connections CRUD, test, detect, quota
//   - combo-api-handler.go   — combos CRUD
//   - alias-handler.go       — aliases CRUD
//   - key-handler.go         — API keys CRUD
//   - settings-handler.go    — settings get/update
//   - model-api-handler.go   — model list + registry CRUD
//   - quota-handler.go       — quota check + Codex probing
//   - backup-handler.go      — backup export/import
func RegisterAPIRoutes(r *gin.Engine, store port.CredentialStore, providers port.ProviderRegistry, onComboDelete func(string)) {
	api := r.Group("/api")
	api.Use(dashboardKeyMiddleware(store))
	{
		// Debug endpoint
		api.GET("/debug/providers", apiDebugProviders(store))
		api.GET("/debug/accounts/:provider", apiDebugAccounts(store))

		// Connections
		api.GET("/connections", apiListConnections(store))
		api.POST("/connections/import", apiImportConnection(store))
		api.POST("/connections/import-file", apiImportConnectionFromFile(store))
		api.POST("/connections/import-multiple", apiImportConnectionsFromFile(store))
		api.POST("/connections/export-multiple", apiExportConnections(store))
		api.POST("/connections/add-openai", apiAddOpenAIConnection(store))
		api.POST("/connections/add-custom", apiAddCustomConnection(store))
		api.POST("/connections/add-glm", apiAddGLMConnection(store))
		api.POST("/connections/add-minimax", apiAddMiniMaxConnection(store))
		api.POST("/connections/add-qwen", apiAddQwenConnection(store))
		api.POST("/connections/add-anthropic", apiAddConnection(store, "anthropic"))
		api.POST("/connections/add-gemini", apiAddConnection(store, "gemini"))
		api.POST("/connections/detect-kiro", apiDetectKiroToken(store))
		api.DELETE("/connections/:id", apiDeleteConnection(store))
		api.GET("/connections/:id/export", apiExportConnection(store))
		api.POST("/connections/:id/test", apiTestConnection(store))
		api.PUT("/connections/:id", apiUpdateConnection(store))
		api.POST("/connections/:id/reset-cooldown", apiResetCooldown(store))
		api.POST("/connections/:id/check-quota", apiCheckQuota(store))
		api.POST("/connections/:id/test-model", apiTestModel(store, providers))

		// Combos
		api.GET("/combos", apiListCombos(store))
		api.POST("/combos", apiCreateCombo(store))
		api.PUT("/combos/:id", apiUpdateCombo(store))
		api.DELETE("/combos/:id", apiDeleteCombo(store, onComboDelete))

		// Aliases
		api.GET("/aliases", apiListAliases(store))
		api.POST("/aliases", apiSetAlias(store))
		api.DELETE("/aliases/:name", apiDeleteAlias(store))

		// API Keys
		api.GET("/keys", apiListKeys(store))
		api.POST("/keys", apiCreateKey(store))
		api.PUT("/keys/:id", apiUpdateKey(store))
		api.DELETE("/keys/:id", apiDeleteKey(store))

		// Model registry management
		api.GET("/models/registry", apiGetModelRegistry(store))
		api.POST("/models/registry", apiAddModelDefinition(store))
		api.PUT("/models/registry/:key", apiUpdateModelDefinition(store))
		api.DELETE("/models/registry/:key", apiDeleteModelDefinition(store))

		// Settings
		api.GET("/settings", apiGetSettings(store))
		api.PUT("/settings", apiUpdateSettings(store))

		// Models
		api.GET("/models", apiListModels(store))

		// Logs
		api.GET("/logs", apiGetLogs)
		api.GET("/logs/detail/:id", apiGetLogByID)
		api.GET("/logs/summary", apiGetLogSummary)
		api.GET("/logs/connections", apiGetLogConnections)
		api.GET("/logs/daily", apiGetLogDaily)
		api.GET("/logs/prices", apiGetLogPrices)
		api.POST("/logs/prices", apiCreatePrice)
		api.PUT("/logs/prices/:id", apiUpdatePrice)
		api.DELETE("/logs/prices/:id", apiDeletePrice)
		api.GET("/logs/stream", apiLogStream)
		api.POST("/logs/clear", apiClearLogs)

		// Usage Analytics
		api.GET("/usage/stats", apiGetUsageStats)
		api.GET("/usage/chart", apiGetUsageChart)
		api.GET("/usage/request-details", apiGetRequestDetails)

		// Usage/Quota (per-connection)
		api.GET("/usage/:connectionId", apiGetUsage(store))

		// Backup
		api.GET("/backup/export", apiExportBackup(store))
		api.POST("/backup/import", apiImportBackup(store))

		// Profiles
		RegisterProfileRoutes(api, store)

		// Auth validation (exempt from middleware, used by UI to verify stored key)
		api.POST("/auth/validate-key", apiValidateKey(store))
	}
}

// apiGetUsage delegates to the UsageHandler.
func apiGetUsage(store port.CredentialStore) gin.HandlerFunc {
	handler := NewUsageHandler(store)
	return handler.GetUsage
}

// apiDebugProviders dumps all registered provider configs and active connections.
func apiDebugProviders(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		providers := domain.ListProviders()
		result := make([]gin.H, 0, len(providers))
		for _, id := range providers {
			cfg := domain.GetProviderConfig(id)
			conns, _ := store.GetActiveConnections(id)
			result = append(result, gin.H{
				"id":             cfg.ID,
				"name":           cfg.Name,
				"icon":           cfg.Icon,
				"authMethods":    cfg.AuthMethods,
				"defaultBaseURL": cfg.DefaultBaseURL,
				"chatPath":       cfg.ChatPath,
				"defaultModels":  len(cfg.DefaultModels),
				"activeConns":    len(conns),
			})
		}
		c.JSON(200, gin.H{"providers": result})
	}
}

// apiDebugAccounts dumps account selection state for debugging (credentials masked).
func apiDebugAccounts(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := c.Param("provider")
		connections, err := store.GetActiveConnections(provider)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[DEBUG] GetActiveConnections(%s) -> %d connections", provider, len(connections))

		type safeDebugConn struct {
			ID               string            `json:"id"`
			Name             string            `json:"name"`
			IsActive         bool              `json:"isActive"`
			Weight           int               `json:"weight"`
			SupportedModels  []string          `json:"supportedModels"`
			RateLimitedUntil string            `json:"rateLimitedUntil"`
			ModelLocks       map[string]string `json:"modelLocks"`
			BackoffLevel     int               `json:"backoffLevel"`
			HasToken         bool              `json:"hasToken"`
			HasAPIKey        bool              `json:"hasApiKey"`
		}

		var safe []safeDebugConn
		for i, conn := range connections {
			log.Printf("[DEBUG]   [%d] id=%s name=%s isActive=%v supportedModels=%v rateLimitedUntil=%s modelLocks=%v",
				i, conn.ID, conn.Name, conn.IsActive, conn.SupportedModels, conn.RateLimitedUntil, conn.ModelLocks)
			safe = append(safe, safeDebugConn{
				ID:               conn.ID,
				Name:             conn.Name,
				IsActive:         conn.IsActive,
				Weight:           conn.Weight,
				SupportedModels:  conn.SupportedModels,
				RateLimitedUntil: conn.RateLimitedUntil,
				ModelLocks:       conn.ModelLocks,
				BackoffLevel:     conn.BackoffLevel,
				HasToken:         conn.AccessToken != "",
				HasAPIKey:        conn.APIKey != "",
			})
		}
		c.JSON(200, gin.H{
			"provider":    provider,
			"count":       len(connections),
			"connections": safe,
		})
	}
}
