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

		key := extractAPIKey(c.Request)
		apiKey, ok := store.GetAPIKeyByValue(key)
		authenticated := ok && apiKey != nil && apiKey.IsActive
		if !authenticated {
			c.JSON(200, gin.H{})
			return
		}

		s := cfg.Settings
		c.JSON(200, settingsAPIView{
			ComboStrategy:            s.ComboStrategy,
			ComboStrategies:          s.ComboStrategies,
			ConnectionStrategy:       s.ConnectionStrategy,
			ConnectionStrategies:     s.ConnectionStrategies,
			Compression:              s.Compression,
			LogBodies:                s.LogBodies,
			DefaultModels:            s.DefaultModels,
			DisableImageGeneration:   s.DisableImageGeneration,
		})
	}
}

func apiUpdateSettings(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var req settingsUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid settings"})
			return
		}

		var updated domain.Settings
		if err := store.Update(func(cfg *domain.AppConfig) {
			if req.ComboStrategy != "" {
				cfg.Settings.ComboStrategy = req.ComboStrategy
			}
			if req.ConnectionStrategy != "" {
				cfg.Settings.ConnectionStrategy = req.ConnectionStrategy
			}
			if req.ConnectionStrategies != nil {
				cfg.Settings.ConnectionStrategies = req.ConnectionStrategies
			}
			if req.ComboStrategies != nil {
				cfg.Settings.ComboStrategies = req.ComboStrategies
			}
			cfg.Settings.Compression = req.Compression
			cfg.Settings.Compression.Normalize()
			cfg.Settings.LogBodies = req.LogBodies
			if req.DefaultModels != nil {
				cfg.Settings.DefaultModels = req.DefaultModels
			}
			if req.DisableImageGeneration != nil {
				cfg.Settings.DisableImageGeneration = *req.DisableImageGeneration
			}

			updated = cfg.Settings
		}); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save config"})
			return
		}

		// Update runtime flag for body logging
		shared.SetLogBodiesEnabled(updated.LogBodies)

		c.JSON(200, settingsAPIView{
			ComboStrategy:          updated.ComboStrategy,
			ComboStrategies:        updated.ComboStrategies,
			ConnectionStrategy:     updated.ConnectionStrategy,
			ConnectionStrategies:   updated.ConnectionStrategies,
			Compression:            updated.Compression,
			LogBodies:              updated.LogBodies,
			DefaultModels:          updated.DefaultModels,
			DisableImageGeneration: updated.DisableImageGeneration,
		})
	}
}

// Port and requireApiKey are not accepted here. Listen port comes from
// PORT / --port; API keys are always required.
type settingsUpdateRequest struct {
	ComboStrategy        string              `json:"comboStrategy"`
	ConnectionStrategy   string              `json:"connectionStrategy"`
	ConnectionStrategies map[string]string   `json:"connectionStrategies"`
	ComboStrategies      map[string]string   `json:"comboStrategies"`
	Compression          domain.CompressionSettings `json:"compression"`
	LogBodies                bool                 `json:"logBodies"`
	DefaultModels            map[string][]string  `json:"defaultModels"`
	DisableImageGeneration   *bool                `json:"disableImageGeneration"`
}

type settingsAPIView struct {
	ComboStrategy          string                       `json:"comboStrategy"`
	ComboStrategies        map[string]string            `json:"comboStrategies,omitempty"`
	ConnectionStrategy     string                       `json:"connectionStrategy,omitempty"`
	ConnectionStrategies   map[string]string            `json:"connectionStrategies,omitempty"`
	Compression            domain.CompressionSettings   `json:"compression"`
	LogBodies              bool                         `json:"logBodies"`
	DefaultModels          map[string][]string          `json:"defaultModels,omitempty"`
	DisableImageGeneration bool                         `json:"disableImageGeneration"`
}
