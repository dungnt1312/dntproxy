package http

import (
	"crypto/rand"
	"encoding/hex"
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

		type safeKey struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			KeyMasked string `json:"keyMasked"`
			IsActive  bool   `json:"isActive"`
			CreatedAt string `json:"createdAt"`
		}

		var keys []safeKey
		for _, k := range cfg.APIKeys {
			masked := k.Key
			if len(masked) > 14 {
				masked = masked[:10] + "..." + masked[len(masked)-4:]
			}
			keys = append(keys, safeKey{
				ID:        k.ID,
				Name:      k.Name,
				KeyMasked: masked,
				IsActive:  k.IsActive,
				CreatedAt: k.CreatedAt,
			})
		}
		c.JSON(200, keys)
	}
}

func apiCreateKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(400, gin.H{"error": "name required"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		keyBytes := make([]byte, 24)
		rand.Read(keyBytes)
		key := "sk-dnt-" + hex.EncodeToString(keyBytes)

		apiKey := domain.APIKey{
			ID:        uuid.New().String(),
			Name:      req.Name,
			Key:       key,
			IsActive:  true,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		cfg.APIKeys = append(cfg.APIKeys, apiKey)

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"id": apiKey.ID, "name": apiKey.Name, "key": key})
	}
}

func apiDeleteKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		found := false
		for i, k := range cfg.APIKeys {
			if k.ID == id {
				cfg.APIKeys = append(cfg.APIKeys[:i], cfg.APIKeys[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			c.JSON(404, gin.H{"error": "Key not found"})
			return
		}

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}
