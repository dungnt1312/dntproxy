package http

import (
	"github.com/dungnt/dntproxy/internal/adapter/commandcode"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

func apiDetectCommandCodeAuth(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var req struct {
			Import bool `json:"import"`
		}
		_ = c.ShouldBindJSON(&req)

		auth, err := commandcode.FindAuthFile(nil)
		if err != nil {
			c.JSON(200, gin.H{"found": false, "error": err.Error()})
			return
		}

		resp := gin.H{
			"found":    true,
			"path":     auth.Source,
			"userName": auth.UserName,
			"keyName":  auth.KeyName,
			"name":     auth.DisplayName(),
		}
		if !req.Import {
			c.JSON(200, resp)
			return
		}

		if existing := findCommandCodeConnection(store, auth.APIKey, GetTenantID(c)); existing != nil {
			resp["imported"] = false
			resp["duplicate"] = true
			resp["id"] = existing.ID
			resp["name"] = existing.Name
			c.JSON(200, resp)
			return
		}

		conn, errMsg, code := createAPIKeyConnection(c, store, "commandcode", addConnectionRequest{
			Provider: "commandcode",
			Name:     auth.DisplayName(),
			APIKey:   auth.APIKey,
		})
		if code != 0 {
			c.JSON(code, gin.H{"error": errMsg})
			return
		}
		resp["imported"] = true
		resp["id"] = conn.ID
		resp["name"] = conn.Name
		c.JSON(200, resp)
	}
}

func findCommandCodeConnection(store port.CredentialStore, apiKey, tenantID string) *domain.ProviderConnection {
	cfg, err := store.Load()
	if err != nil || cfg == nil {
		return nil
	}
	for i := range cfg.ProviderConnections {
		conn := &cfg.ProviderConnections[i]
		if conn.Provider != "commandcode" || conn.APIKey != apiKey {
			continue
		}
		if tenantID != "" && conn.TenantID != "" && conn.TenantID != tenantID {
			continue
		}
		return conn
	}
	return nil
}
