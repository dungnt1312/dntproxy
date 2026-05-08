package http

import (
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

		c.JSON(200, settings)
	}
}

func apiUpdateSettings(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
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
			cfg.Settings.RequireAPIKey = req.RequireAPIKey
			if req.ComboStrategies != nil {
				cfg.Settings.ComboStrategies = req.ComboStrategies
			}
			cfg.Settings.Compression = req.Compression
			cfg.Settings.Compression.Normalize()
			updated = cfg.Settings
		}); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save config"})
			return
		}
		c.JSON(200, updated)
	}
}
