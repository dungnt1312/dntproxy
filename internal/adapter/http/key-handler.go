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
		c.JSON(200, cfg.APIKeys)
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
		if _, err := rand.Read(keyBytes); err != nil {
			c.JSON(500, gin.H{"error": "failed to generate secure API key"})
			return
		}
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

func apiValidateKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Key string `json:"key"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Key == "" {
			c.JSON(400, gin.H{"valid": false, "error": "key is required"})
			return
		}
		if store.ValidateAPIKey(req.Key) {
			c.JSON(200, gin.H{"valid": true})
		} else {
			c.JSON(200, gin.H{"valid": false})
		}
	}
}
