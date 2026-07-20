package http

import (
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// === Settings ===

func apiGetSettings(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// Override with actual running port from context
		settings := cfg.Settings
		if actualPort := GetServerPort(c); actualPort > 0 {
			settings.Port = actualPort
		}

		// Strip secrets for non-admin callers. This endpoint is intentionally
		// exempt from auth middleware (needed for UI bootstrap), so we resolve
		// the key directly from the Authorization header instead of relying on
		// GetTenantID (which is empty when middleware is skipped).
		key := extractAPIKey(c.Request)
		apiKey, ok := store.GetAPIKeyByValue(key)
		if ok && apiKey != nil && !domain.IsLegacyTenant(apiKey.TenantID) {
			settings.Telegram.BotToken = ""
		}

		c.JSON(200, settings)
	}
}

func apiUpdateSettings(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var req domain.Settings
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid settings"})
			return
		}

		var updated domain.Settings
		if err := store.Update(func(cfg *domain.AppConfig) {
			if req.Port > 0 {
				cfg.Settings.Port = req.Port
			}
			if req.ComboStrategy != "" {
				cfg.Settings.ComboStrategy = req.ComboStrategy
			}
			if req.ConnectionStrategy != "" {
				cfg.Settings.ConnectionStrategy = req.ConnectionStrategy
			}
			cfg.Settings.RequireAPIKey = req.RequireAPIKey
			if req.ComboStrategies != nil {
				cfg.Settings.ComboStrategies = req.ComboStrategies
			}
			cfg.Settings.Compression = req.Compression
			cfg.Settings.Compression.Normalize()
			cfg.Settings.LogBodies = req.LogBodies
			if req.DefaultModels != nil {
				cfg.Settings.DefaultModels = req.DefaultModels
			}

			// Telegram settings
			cfg.Settings.Telegram.Enabled = req.Telegram.Enabled
			if req.Telegram.BotToken != "" {
				cfg.Settings.Telegram.BotToken = req.Telegram.BotToken
			}
			if req.Telegram.OwnerID != 0 {
				cfg.Settings.Telegram.OwnerID = req.Telegram.OwnerID
			}

			updated = cfg.Settings
		}); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save config"})
			return
		}

		// Update runtime flag for body logging
		shared.SetLogBodiesEnabled(updated.LogBodies)

		c.JSON(200, updated)
	}
}
