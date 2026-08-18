package http

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// === API Keys ===

func apiListKeys(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		tenantID := GetTenantID(c)
		// In legacy mode (empty tenant) list all keys (admin view).
		// In tenant mode, only list keys for that tenant.
		keys := domain.FilterAPIKeysByTenant(cfg.APIKeys, tenantID)
		out := make([]domain.APIKey, len(keys))
		for i, k := range keys {
			out[i] = k
			out[i].Key = maskAPIKey(k.Key)
		}
		c.JSON(200, out)
	}
}

func apiCreateKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name                 string   `json:"name"`
			DashboardAccess      *bool    `json:"dashboardAccess"`
			AllowedConnectionIDs []string `json:"allowedConnectionIds"`
			AllowedModels        []string `json:"allowedModels"`
			TenantID             string   `json:"tenantId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(400, gin.H{"error": "name required"})
			return
		}
		req.AllowedConnectionIDs = uniqueNonEmpty(req.AllowedConnectionIDs)
		req.AllowedModels = uniqueNonEmpty(req.AllowedModels)
		callerTenant := GetTenantID(c)
		if err := validateAllowedConnectionIDs(store, req.AllowedConnectionIDs, callerTenant); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		isAdmin := domain.IsLegacyTenant(callerTenant)

		// Non-admin tenants can only create keys for their own tenant and
		// cannot grant dashboard access (which would elevate privileges).
		if !isAdmin {
			req.TenantID = callerTenant
			req.DashboardAccess = nil // force false below
		}

		// Default dashboardAccess to false if not specified
		dashboardAccess := false
		if req.DashboardAccess != nil {
			dashboardAccess = *req.DashboardAccess
		}

		key := GenerateAPIKey(req.TenantID)

		apiKey := domain.APIKey{
			ID:                   uuid.New().String(),
			Name:                 req.Name,
			Key:                  key,
			IsActive:             true,
			DashboardAccess:      dashboardAccess,
			CreatedAt:            time.Now().UTC().Format(time.RFC3339),
			AllowedConnectionIDs: req.AllowedConnectionIDs,
			AllowedModels:        req.AllowedModels,
			TenantID:             req.TenantID,
		}

		if err := store.Update(func(cfg *domain.AppConfig) {
			cfg.APIKeys = append(cfg.APIKeys, apiKey)
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"id": apiKey.ID, "name": apiKey.Name, "key": key, "tenantId": apiKey.TenantID})
	}
}

// GenerateAPIKey produces a tenant-aware API key string.
// Format: sk-dnt-{tenantShortID}-{random48hex}
// When tenantID is empty, format is: sk-dnt-{random48hex} (legacy).
func GenerateAPIKey(tenantID string) string {
	keyBytes := make([]byte, 24)
	if _, err := rand.Read(keyBytes); err != nil {
		// Fallback to a UUID-based key (should never happen in practice)
		return "sk-dnt-" + uuid.New().String()
	}
	random := hex.EncodeToString(keyBytes)
	if tenantID == "" {
		return "sk-dnt-" + random
	}
	// Use a short, sanitized tenant identifier in the key prefix.
	short := sanitizeTenantPrefix(tenantID)
	if short == "" {
		return "sk-dnt-" + random
	}
	return "sk-dnt-" + short + "-" + random
}

// sanitizeTenantPrefix reduces a tenant ID to a short alphanumeric slug suitable
// for embedding in an API key. Returns "" if no usable characters remain.
func sanitizeTenantPrefix(tenantID string) string {
	var b strings.Builder
	count := 0
	for _, r := range strings.ToLower(tenantID) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			count++
			if count >= 12 {
				break
			}
		}
	}
	return b.String()
}

func apiDeleteKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if _, ok := resolveOwnedAPIKey(c, store, id); !ok {
			return
		}
		found := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			for i, k := range cfg.APIKeys {
				if k.ID == id {
					cfg.APIKeys = append(cfg.APIKeys[:i], cfg.APIKeys[i+1:]...)
					found = true
					break
				}
			}
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Key not found"})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

func apiUpdateKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if _, ok := resolveOwnedAPIKey(c, store, id); !ok {
			return
		}
		var req struct {
			Name                 *string   `json:"name"`
			IsActive             *bool     `json:"isActive"`
			DashboardAccess      *bool     `json:"dashboardAccess"`
			AllowedConnectionIDs *[]string `json:"allowedConnectionIds"`
			AllowedModels        *[]string `json:"allowedModels"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}
		var allowedConns []string
		var allowedModels []string
		if req.AllowedConnectionIDs != nil {
			allowedConns = uniqueNonEmpty(*req.AllowedConnectionIDs)
			if err := validateAllowedConnectionIDs(store, allowedConns, GetTenantID(c)); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
		}
		if req.AllowedModels != nil {
			allowedModels = uniqueNonEmpty(*req.AllowedModels)
		}

		// Non-admin tenants cannot grant dashboard access.
		if !domain.IsLegacyTenant(GetTenantID(c)) {
			req.DashboardAccess = nil
		}

		found := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			for i := range cfg.APIKeys {
				if cfg.APIKeys[i].ID == id {
					if req.Name != nil {
						cfg.APIKeys[i].Name = *req.Name
					}
					if req.IsActive != nil {
						cfg.APIKeys[i].IsActive = *req.IsActive
					}
					if req.DashboardAccess != nil {
						cfg.APIKeys[i].DashboardAccess = *req.DashboardAccess
					}
					if req.AllowedConnectionIDs != nil {
						cfg.APIKeys[i].AllowedConnectionIDs = allowedConns
					}
					if req.AllowedModels != nil {
						cfg.APIKeys[i].AllowedModels = allowedModels
					}
					found = true
					break
				}
			}
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Key not found"})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:7] + "..." + key[len(key)-4:]
}

func validateAllowedConnectionIDs(store port.CredentialStore, ids []string, tenantID string) error {
	if len(ids) == 0 {
		return nil
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}
	conns := domain.FilterConnectionsByTenant(cfg.ProviderConnections, tenantID)
	existing := make(map[string]struct{}, len(conns))
	for _, conn := range conns {
		existing[conn.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := existing[id]; !ok {
			return fmt.Errorf("unknown connection id: %s", id)
		}
	}
	return nil
}

// apiSession returns session info for the currently-authenticated key.
// Used by the UI on page reload to recover tenant/admin context without
// re-prompting for the key. Reads the key from the Authorization header
// (or ?key= query param for browsers that strip headers on refresh).
//
// A key only counts as an authenticated dashboard session when it is active,
// has DashboardAccess, and (if tenant-scoped) the tenant is not disabled.
func apiSession(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := extractAPIKey(c.Request)
		if key == "" {
			key = c.Query("key")
		}
		if key == "" {
			c.JSON(200, gin.H{"authenticated": false})
			return
		}
		apiKey, valid := store.GetAPIKeyByValue(key)
		if !valid || apiKey == nil || !apiKey.IsActive || !apiKey.DashboardAccess {
			c.JSON(200, gin.H{"authenticated": false})
			return
		}
		if isTenantDisabledCached(store, apiKey.TenantID) {
			c.JSON(200, gin.H{"authenticated": false})
			return
		}
		c.JSON(200, gin.H{
			"authenticated":   true,
			"dashboardAccess": true,
			"tenantId":        apiKey.TenantID,
			"isAdmin":         domain.IsLegacyTenant(apiKey.TenantID),
			"keyId":           apiKey.ID,
			"keyName":         apiKey.Name,
		})
	}
}

func apiValidateKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Key string `json:"key"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Key == "" {
			c.JSON(400, gin.H{"valid": false, "error": "key is required"})
			return
		}
		apiKey, valid := store.GetAPIKeyByValue(req.Key)
		if valid && apiKey != nil {
			c.JSON(200, gin.H{
				"valid":           true,
				"dashboardAccess": apiKey.DashboardAccess,
				"tenantId":        apiKey.TenantID,
				// isAdmin is true for legacy/global keys (no tenant) — they see all tenants.
				"isAdmin": domain.IsLegacyTenant(apiKey.TenantID),
			})
		} else {
			c.JSON(200, gin.H{"valid": false})
		}
	}
}
