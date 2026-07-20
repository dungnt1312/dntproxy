package http

import (
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// === Combos ===

// normalizeModelString validates and normalizes "provider/model@connectionId" format.
// Prevents duplicate provider prefix (e.g., "glm/glm/glm-5.1" -> "glm/glm-5.1").
func normalizeModelString(modelStr string) string {
	// Split @connectionId
	atIdx := strings.Index(modelStr, "@")
	modelPart := modelStr
	connSuffix := ""
	if atIdx >= 0 {
		modelPart = modelStr[:atIdx]
		connSuffix = modelStr[atIdx:]
	}

	// Split by / and check for duplicate prefix
	parts := strings.Split(modelPart, "/")
	if len(parts) < 2 {
		return modelStr // Invalid format, let resolver handle error
	}

	// If first two parts are identical, it's a duplicate prefix
	if len(parts) >= 3 && parts[0] == parts[1] {
		// Remove duplicate: "glm/glm/glm-5.1" -> "glm/glm-5.1"
		parts = append([]string{parts[0]}, parts[2:]...)
	}

	return strings.Join(parts, "/") + connSuffix
}

func apiListCombos(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		tenantID := GetTenantID(c)
		c.JSON(200, domain.FilterCombosByTenant(cfg.Combos, tenantID))
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

		// Normalize all model strings to prevent duplicate prefixes
		normalizedModels := make([]string, len(req.Models))
		for i, m := range req.Models {
			normalizedModels[i] = normalizeModelString(m)
		}

		tenantID := GetTenantID(c)

		now := time.Now().UTC().Format(time.RFC3339)
		combo := domain.Combo{
			ID:            uuid.New().String(),
			Name:          req.Name,
			Models:        normalizedModels,
			ConnectionIDs: req.ConnectionIDs,
			CreatedAt:     now,
			UpdatedAt:     now,
			TenantID:      tenantID,
		}

		errConflict := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			for _, existing := range cfg.Combos {
				if existing.Name == req.Name && domain.SameTenant(existing.TenantID, tenantID) {
					errConflict = true
					return
				}
			}
			cfg.Combos = append(cfg.Combos, combo)
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if errConflict {
			c.JSON(409, gin.H{"error": "Combo already exists: " + req.Name})
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

		tenantID := GetTenantID(c)

		var (
			updated     *domain.Combo
			notFound    bool
			emptyModels bool
		)
		if err := store.Update(func(cfg *domain.AppConfig) {
			var combo *domain.Combo
			for i := range cfg.Combos {
				if (cfg.Combos[i].ID == id || cfg.Combos[i].Name == id) &&
					domain.SameTenant(cfg.Combos[i].TenantID, tenantID) {
					combo = &cfg.Combos[i]
					break
				}
			}
			if combo == nil {
				notFound = true
				return
			}
			if req.Name != nil {
				combo.Name = *req.Name
			}
			if req.SetModels {
				if len(req.Models) == 0 {
					emptyModels = true
					return
				}
				normalizedModels := make([]string, len(req.Models))
				for i, m := range req.Models {
					normalizedModels[i] = normalizeModelString(m)
				}
				combo.Models = normalizedModels
			}
			if req.SetConnections {
				combo.ConnectionIDs = req.ConnectionIDs
			}
			combo.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			cp := *combo
			updated = &cp
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if notFound {
			c.JSON(404, gin.H{"error": "Combo not found"})
			return
		}
		if emptyModels {
			c.JSON(400, gin.H{"error": "combo models cannot be empty"})
			return
		}
		c.JSON(200, updated)
	}
}

func apiDeleteCombo(store port.CredentialStore, onComboDelete func(string)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		tenantID := GetTenantID(c)
		found := false
		deletedName := ""
		if err := store.Update(func(cfg *domain.AppConfig) {
			for i, combo := range cfg.Combos {
				if (combo.ID == id || combo.Name == id) && domain.SameTenant(combo.TenantID, tenantID) {
					deletedName = combo.Name
					cfg.Combos = append(cfg.Combos[:i], cfg.Combos[i+1:]...)
					found = true
					break
				}
			}
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Combo not found"})
			return
		}

		// Clear rotation state for deleted combo
		if onComboDelete != nil && deletedName != "" {
			onComboDelete(deletedName)
		}

		c.JSON(200, gin.H{"ok": true})
	}
}
