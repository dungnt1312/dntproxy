package http

import (
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// === Aliases ===

func apiListAliases(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		// Aliases are currently global (shared namespace). Acceptable for admin routing,
		// but still require dashboard auth via middleware.
		c.JSON(200, cfg.ModelAliases)
	}
}

func apiSetAlias(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var req struct {
			Alias string `json:"alias"`
			Model string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Alias == "" || req.Model == "" {
			c.JSON(400, gin.H{"error": "alias and model required"})
			return
		}

		if err := store.Update(func(cfg *domain.AppConfig) {
			if cfg.ModelAliases == nil {
				cfg.ModelAliases = make(domain.AliasMap)
			}
			cfg.ModelAliases[req.Alias] = req.Model
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

func apiDeleteAlias(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		name := c.Param("name")
		found := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			if _, ok := cfg.ModelAliases[name]; !ok {
				return
			}
			delete(cfg.ModelAliases, name)
			found = true
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Alias not found"})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}
