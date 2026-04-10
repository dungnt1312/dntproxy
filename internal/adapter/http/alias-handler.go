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
		c.JSON(200, cfg.ModelAliases)
	}
}

func apiSetAlias(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Alias string `json:"alias"`
			Model string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Alias == "" || req.Model == "" {
			c.JSON(400, gin.H{"error": "alias and model required"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		if cfg.ModelAliases == nil {
			cfg.ModelAliases = make(domain.AliasMap)
		}
		cfg.ModelAliases[req.Alias] = req.Model

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

func apiDeleteAlias(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		if _, ok := cfg.ModelAliases[name]; !ok {
			c.JSON(404, gin.H{"error": "Alias not found"})
			return
		}
		delete(cfg.ModelAliases, name)

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}
