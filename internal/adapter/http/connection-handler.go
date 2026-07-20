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

// redactConnectionSecrets strips credential material from list/detail API
// responses. Full secrets remain available via export endpoints only.
func redactConnectionSecrets(conn domain.ProviderConnection) domain.ProviderConnection {
	out := conn
	out.AccessToken = ""
	out.RefreshToken = ""
	out.APIKey = ""
	if len(out.ProviderSpecificData) > 0 {
		safe := make(map[string]interface{}, len(out.ProviderSpecificData))
		for k, v := range out.ProviderSpecificData {
			lk := strings.ToLower(k)
			if strings.Contains(lk, "secret") || strings.Contains(lk, "token") ||
				strings.Contains(lk, "password") || lk == "clientsecret" ||
				lk == "client_secret" || lk == "idtoken" || lk == "id_token" {
				continue
			}
			safe[k] = v
		}
		out.ProviderSpecificData = safe
	}
	return out
}

func apiListConnections(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}
		// Filter connections by tenant (no-op in legacy single-tenant mode).
		tenantID := GetTenantID(c)
		conns := domain.FilterConnectionsByTenant(cfg.ProviderConnections, tenantID)
		views := make([]connectionView, len(conns))
		for i, conn := range conns {
			provCfg := domain.GetProviderConfig(conn.Provider)
			supportsQuota := provCfg.SupportsQuota
			if conn.Provider == "openai" && conn.AuthType != "oauth" {
				supportsQuota = false
			}

			// Runtime fill: if connection has no SupportedModels, use RecommendedModels
			// This ensures old connections (created before RecommendedModels existed)
			// still show the correct curated model list in the UI.
			displayConn := redactConnectionSecrets(conn)
			if len(displayConn.SupportedModels) == 0 && len(provCfg.RecommendedModels) > 0 {
				displayConn.SupportedModels = provCfg.RecommendedModels
			}

			views[i] = connectionView{
				ProviderConnection: displayConn,
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
		if _, ok := requireTenantOwnsConnection(c, store, id); !ok {
			return
		}
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
		if _, ok := requireTenantOwnsConnection(c, store, id); !ok {
			return
		}
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
		if _, ok := requireTenantOwnsConnection(c, store, id); !ok {
			return
		}
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
		if _, ok := requireTenantOwnsConnection(c, store, id); !ok {
			return
		}
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

// ensureTenantOwnsConnection returns false if a specific tenant is requesting
// access to a connection that does not belong to it. Legacy (empty tenant)
// always passes. Use this in mutation handlers to enforce isolation.
func ensureTenantOwnsConnection(c *gin.Context, store port.CredentialStore, id string) (*domain.ProviderConnection, bool) {
	conn, err := store.GetConnectionByID(id)
	if err != nil || conn == nil {
		return nil, false
	}
	tenantID := GetTenantID(c)
	if !domain.SameTenant(conn.TenantID, tenantID) {
		return nil, false
	}
	return conn, true
}

// logAction logs an admin action.
func logAction(action string, args ...interface{}) {
	log.Printf("[ADMIN] "+action, args...)
}
