package http

import (
	"log"
	"strings"
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
			Priority        *int     `json:"priority,omitempty"`
			Weight          *int     `json:"weight,omitempty"`
			SupportedModels []string `json:"supportedModels,omitempty"`
			SetModels       bool     `json:"setModels,omitempty"`
			APIKey          *string  `json:"apiKey,omitempty"`
			BaseURL         *string  `json:"baseUrl,omitempty"`
			RoutePrefix     *string  `json:"routePrefix,omitempty"`
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
					if req.Priority != nil {
						conn.Priority = *req.Priority
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
					if req.RoutePrefix != nil {
						conn.RoutePrefix = domain.NormalizeRoutePrefix(*req.RoutePrefix)
					}
					if req.ModelPrefix != nil {
						conn.ModelPrefix = *req.ModelPrefix
					}
					conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
					domain.EnsureOpenAICompatibleRoutePrefixes(cfg.ProviderConnections)
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

func apiClearConnectionError(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Model string `json:"model,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		found := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			for i := range cfg.ProviderConnections {
				if cfg.ProviderConnections[i].ID != id {
					continue
				}
				conn := &cfg.ProviderConnections[i]
				found = true
				if req.Model != "" {
					model := clearErrorModelKey(req.Model, *conn)
					if conn.ModelLocks != nil {
						delete(conn.ModelLocks, model)
						if len(conn.ModelLocks) == 0 {
							conn.ModelLocks = nil
						}
					}
					if strings.Contains(conn.LastError, req.Model) || strings.Contains(conn.LastError, model) {
						conn.LastError = ""
						conn.LastErrorAt = ""
					}
				} else {
					conn.RateLimitedUntil = ""
					conn.BackoffLevel = 0
					conn.LastError = ""
					conn.LastErrorAt = ""
					conn.ModelLocks = nil
				}
				conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				break
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

func clearErrorModelKey(model string, conn domain.ProviderConnection) string {
	if conn.Provider != "openai-compatible" || !strings.Contains(model, "/") {
		return model
	}
	prefix := domain.NormalizeRoutePrefix(conn.RoutePrefix)
	if prefix == "" {
		prefix = domain.NormalizeRoutePrefix(conn.Name)
	}
	if prefix != "" && strings.HasPrefix(model, prefix+"/") {
		return strings.TrimPrefix(model, prefix+"/")
	}
	return model
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
