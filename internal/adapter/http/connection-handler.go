package http

import (
	"log"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// Connection handler implements CRUD operations for provider connections.
// Handler implementations are split across separate files:
//   - connection-handler.go      — List, delete, update, reset cooldown, helpers
//   - connection-add-handler.go  — Add, import, detect Kiro token
//   - connection-test-handler.go — Test connection, test model, testProviderAPI

// === List Connections ===

// connectionView is the API response shape for a single connection.
// It embeds ProviderConnection and adds computed fields from provider config.
type connectionView struct {
	domain.ProviderConnection
	SupportsQuota bool `json:"supportsQuota"`
}

func apiListConnections(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}
		views := make([]connectionView, len(cfg.ProviderConnections))
		for i, conn := range cfg.ProviderConnections {
			provCfg := domain.GetProviderConfig(conn.Provider)
			supportsQuota := provCfg.SupportsQuota
			if conn.Provider == "openai" && conn.AuthType != "oauth" {
				supportsQuota = false
			}
			views[i] = connectionView{
				ProviderConnection: conn,
				SupportsQuota:      supportsQuota,
			}
		}
		c.JSON(200, views)
	}
}

// === Delete Connection ===

func apiDeleteConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		found := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			for i, conn := range cfg.ProviderConnections {
				if conn.ID == id {
					cfg.ProviderConnections = append(cfg.ProviderConnections[:i], cfg.ProviderConnections[i+1:]...)
					found = true
					break
				}
			}
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

// === Update Connection ===

func apiUpdateConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name            *string  `json:"name,omitempty"`
			IsActive        *bool    `json:"isActive,omitempty"`
			Weight          *int     `json:"weight,omitempty"`
			SupportedModels []string `json:"supportedModels,omitempty"`
			SetModels       bool     `json:"setModels,omitempty"`
			APIKey          *string  `json:"apiKey,omitempty"`
			BaseURL         *string  `json:"baseUrl,omitempty"`
			ModelPrefix     *string  `json:"modelPrefix,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		found := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			for i := range cfg.ProviderConnections {
				if cfg.ProviderConnections[i].ID == id {
					conn := &cfg.ProviderConnections[i]
					found = true
					if req.Name != nil {
						conn.Name = *req.Name
					}
					if req.IsActive != nil {
						conn.IsActive = *req.IsActive
					}
					if req.Weight != nil {
						conn.Weight = *req.Weight
					}
					if req.SetModels {
						conn.SupportedModels = req.SupportedModels
					}
					if req.APIKey != nil {
						conn.APIKey = *req.APIKey
					}
					if req.BaseURL != nil {
						conn.BaseURL = *req.BaseURL
					}
					if req.ModelPrefix != nil {
						conn.ModelPrefix = *req.ModelPrefix
					}
					conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
					break
				}
			}
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

// === Reset Cooldown ===

func apiResetCooldown(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		found := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			for i := range cfg.ProviderConnections {
				if cfg.ProviderConnections[i].ID == id {
					conn := &cfg.ProviderConnections[i]
					found = true
					conn.RateLimitedUntil = ""
					conn.BackoffLevel = 0
					conn.LastError = ""
					conn.LastErrorAt = ""
					conn.ModelLocks = nil
					conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
					break
				}
			}
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

// findConnectionByID is a helper to find a connection by ID in a config.
func findConnectionByID(cfg *domain.AppConfig, id string) *domain.ProviderConnection {
	for i := range cfg.ProviderConnections {
		if cfg.ProviderConnections[i].ID == id {
			return &cfg.ProviderConnections[i]
		}
	}
	return nil
}

// logAction logs an admin action.
func logAction(action string, args ...interface{}) {
	log.Printf("[ADMIN] "+action, args...)
}
