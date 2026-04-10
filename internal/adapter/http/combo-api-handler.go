package http

import (
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// === Combos ===

func apiListCombos(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, cfg.Combos)
	}
}

func apiCreateCombo(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name          string   `json:"name"`
			Models        []string `json:"models"`
			ConnectionIDs []string `json:"connectionIds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(400, gin.H{"error": "name required"})
			return
		}
		if len(req.Models) == 0 {
			c.JSON(400, gin.H{"error": "at least one model required"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		for _, combo := range cfg.Combos {
			if combo.Name == req.Name {
				c.JSON(409, gin.H{"error": "Combo already exists: " + req.Name})
				return
			}
		}

		now := time.Now().UTC().Format(time.RFC3339)
		combo := domain.Combo{
			ID:            uuid.New().String(),
			Name:          req.Name,
			Models:        req.Models,
			ConnectionIDs: req.ConnectionIDs,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		cfg.Combos = append(cfg.Combos, combo)

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, combo)
	}
}

func apiUpdateCombo(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name           *string  `json:"name,omitempty"`
			Models         []string `json:"models,omitempty"`
			ConnectionIDs  []string `json:"connectionIds,omitempty"`
			SetModels      bool     `json:"setModels,omitempty"`
			SetConnections bool     `json:"setConnections,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		var combo *domain.Combo
		for i := range cfg.Combos {
			if cfg.Combos[i].ID == id || cfg.Combos[i].Name == id {
				combo = &cfg.Combos[i]
				break
			}
		}
		if combo == nil {
			c.JSON(404, gin.H{"error": "Combo not found"})
			return
		}

		if req.Name != nil {
			combo.Name = *req.Name
		}
		if req.SetModels {
			if len(req.Models) == 0 {
				c.JSON(400, gin.H{"error": "combo models cannot be empty"})
				return
			}
			combo.Models = req.Models
		}
		if req.SetConnections {
			combo.ConnectionIDs = req.ConnectionIDs
		}
		combo.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, combo)
	}
}

func apiDeleteCombo(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		found := false
		for i, combo := range cfg.Combos {
			if combo.ID == id || combo.Name == id {
				cfg.Combos = append(cfg.Combos[:i], cfg.Combos[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			c.JSON(404, gin.H{"error": "Combo not found"})
			return
		}

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}
