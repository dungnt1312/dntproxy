package http

import (
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
func RegisterAPIRoutes(r *gin.Engine, store port.CredentialStore, providers port.ProviderRegistry) {
	api := r.Group("/api")
	{
		// Connections
		api.GET("/connections", apiListConnections(store))
		api.POST("/connections/import", apiImportConnection(store))
		api.POST("/connections/add-openai", apiAddOpenAIConnection(store))
		api.POST("/connections/add-custom", apiAddCustomConnection(store))
		api.POST("/connections/detect-kiro", apiDetectKiroToken(store))
		api.DELETE("/connections/:id", apiDeleteConnection(store))
		api.POST("/connections/:id/test", apiTestConnection(store))
		api.PUT("/connections/:id", apiUpdateConnection(store))
		api.POST("/connections/:id/reset-cooldown", apiResetCooldown(store))
		api.POST("/connections/:id/check-quota", apiCheckQuota(store))
		api.POST("/connections/:id/test-model", apiTestModel(store, providers))

		// Combos
		api.GET("/combos", apiListCombos(store))
		api.POST("/combos", apiCreateCombo(store))
		api.PUT("/combos/:id", apiUpdateCombo(store))
		api.DELETE("/combos/:id", apiDeleteCombo(store))

		// Aliases
		api.GET("/aliases", apiListAliases(store))
		api.POST("/aliases", apiSetAlias(store))
		api.DELETE("/aliases/:name", apiDeleteAlias(store))

		// API Keys
		api.GET("/keys", apiListKeys(store))
		api.POST("/keys", apiCreateKey(store))
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
		api.GET("/logs/summary", apiGetLogSummary)
		api.GET("/logs/connections", apiGetLogConnections)
		api.GET("/logs/prices", apiGetLogPrices)
		api.GET("/logs/stream", apiLogStream)
		api.POST("/logs/clear", apiClearLogs)

		// Usage/Quota
		api.GET("/usage/:connectionId", apiGetUsage(store))

		// Backup
		api.GET("/backup/export", apiExportBackup(store))
		api.POST("/backup/import", apiImportBackup(store))
	}
}

// apiGetUsage delegates to the UsageHandler.
func apiGetUsage(store port.CredentialStore) gin.HandlerFunc {
	handler := NewUsageHandler(store)
	return handler.GetUsage
}
