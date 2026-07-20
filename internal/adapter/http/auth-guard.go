package http

import (
	"net/http"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// requireAdmin aborts the request with 403 unless the caller is authenticated
// with a global/legacy (admin) key. Admin status is derived from an empty
// (or "global") tenant ID on the resolved API key.
//
// Returns true when the caller IS an admin (caller should continue), false
// when the request was already aborted.
func requireAdmin(c *gin.Context) bool {
	if !domain.IsLegacyTenant(GetTenantID(c)) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": gin.H{"message": "Admin access required"},
		})
		return false
	}
	return true
}

// requireTenantOwnsConnection verifies the connection identified by id belongs
// to the requesting tenant. Admins (legacy/global tenant) bypass this check
// and receive the connection regardless of ownership.
//
// Returns the resolved connection + true when access is granted. When the
// connection does not exist or belongs to another tenant, the request is
// aborted with 404 (we deliberately use 404, not 403, to avoid leaking which
// IDs exist) and false is returned.
func requireTenantOwnsConnection(c *gin.Context, store port.CredentialStore, id string) (*domain.ProviderConnection, bool) {
	conn, err := store.GetConnectionByID(id)
	if err != nil || conn == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": gin.H{"message": "Connection not found"},
		})
		return nil, false
	}
	tenantID := GetTenantID(c)
	if !domain.SameTenant(conn.TenantID, tenantID) {
		// Same 404 to avoid cross-tenant enumeration.
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": gin.H{"message": "Connection not found"},
		})
		return nil, false
	}
	return conn, true
}

// resolveOwnedAPIKey loads an API key by ID and verifies the requesting tenant
// owns it. Admins bypass. Returns the key + true when granted; otherwise the
// request is aborted and false returned.
func resolveOwnedAPIKey(c *gin.Context, store port.CredentialStore, id string) (*domain.APIKey, bool) {
	cfg, err := store.Load()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return nil, false
	}
	for i := range cfg.APIKeys {
		if cfg.APIKeys[i].ID != id {
			continue
		}
		k := &cfg.APIKeys[i]
		tenantID := GetTenantID(c)
		if !domain.SameTenant(k.TenantID, tenantID) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": gin.H{"message": "Key not found"},
			})
			return nil, false
		}
		return k, true
	}
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
		"error": gin.H{"message": "Key not found"},
	})
	return nil, false
}
