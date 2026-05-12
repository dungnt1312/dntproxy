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
		c.JSON(200, cfg.APIKeys)
	}
}

func apiCreateKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name                 string   `json:"name"`
			DashboardAccess      *bool    `json:"dashboardAccess"`
			AllowedConnectionIDs []string `json:"allowedConnectionIds"`
			AllowedModels        []string `json:"allowedModels"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(400, gin.H{"error": "name required"})
			return
		}
		req.AllowedConnectionIDs = uniqueNonEmpty(req.AllowedConnectionIDs)
		req.AllowedModels = uniqueNonEmpty(req.AllowedModels)
		if err := validateAllowedConnectionIDs(store, req.AllowedConnectionIDs); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Default dashboardAccess to false if not specified
		dashboardAccess := false
		if req.DashboardAccess != nil {
			dashboardAccess = *req.DashboardAccess
		}

		keyBytes := make([]byte, 24)
		if _, err := rand.Read(keyBytes); err != nil {
			c.JSON(500, gin.H{"error": "failed to generate secure API key"})
			return
		}
		key := "sk-dnt-" + hex.EncodeToString(keyBytes)

		apiKey := domain.APIKey{
			ID:                   uuid.New().String(),
			Name:                 req.Name,
			Key:                  key,
			IsActive:             true,
			DashboardAccess:      dashboardAccess,
			CreatedAt:            time.Now().UTC().Format(time.RFC3339),
			AllowedConnectionIDs: req.AllowedConnectionIDs,
			AllowedModels:        req.AllowedModels,
		}

		if err := store.Update(func(cfg *domain.AppConfig) {
			cfg.APIKeys = append(cfg.APIKeys, apiKey)
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"id": apiKey.ID, "name": apiKey.Name, "key": key})
	}
}

func apiDeleteKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
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
		var req struct {
			Name                 *string  `json:"name"`
			IsActive             *bool    `json:"isActive"`
			DashboardAccess      *bool    `json:"dashboardAccess"`
			AllowedConnectionIDs []string `json:"allowedConnectionIds"`
			AllowedModels        []string `json:"allowedModels"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}
		req.AllowedConnectionIDs = uniqueNonEmpty(req.AllowedConnectionIDs)
		req.AllowedModels = uniqueNonEmpty(req.AllowedModels)
		if err := validateAllowedConnectionIDs(store, req.AllowedConnectionIDs); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
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
					// Always update these (nil in JSON → empty slice → unrestricted)
					cfg.APIKeys[i].AllowedConnectionIDs = req.AllowedConnectionIDs
					cfg.APIKeys[i].AllowedModels = req.AllowedModels
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

func validateAllowedConnectionIDs(store port.CredentialStore, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	cfg, err := store.Load()
	if err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(cfg.ProviderConnections))
	for _, conn := range cfg.ProviderConnections {
		existing[conn.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := existing[id]; !ok {
			return fmt.Errorf("unknown connection id: %s", id)
		}
	}
	return nil
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
			c.JSON(200, gin.H{"valid": true, "dashboardAccess": apiKey.DashboardAccess})
		} else {
			c.JSON(200, gin.H{"valid": false})
		}
	}
}
