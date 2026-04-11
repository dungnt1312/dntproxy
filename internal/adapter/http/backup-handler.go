package http

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

const backupVersion = "1.0"

// === Backup Types ===

type BackupData struct {
	Version             string                     `json:"version"`
	ExportedAt          string                     `json:"exportedAt"`
	ProviderConnections []ProviderConnectionBackup `json:"providerConnections"`
	Combos              []ComboBackup              `json:"combos"`
	ModelAliases        domain.AliasMap            `json:"modelAliases"`
	APIKeys             []APIKeyBackup             `json:"apiKeys"`
	Settings            domain.Settings            `json:"settings"`
}

type ProviderConnectionBackup struct {
	ID                   string                 `json:"id"`
	Provider             string                 `json:"provider"`
	AuthType             string                 `json:"authType"`
	Name                 string                 `json:"name"`
	Priority             int                    `json:"priority"`
	IsActive             bool                   `json:"isActive"`
	AccessToken          string                 `json:"accessToken,omitempty"`
	RefreshToken         string                 `json:"refreshToken,omitempty"`
	ExpiresAt            string                 `json:"expiresAt,omitempty"`
	ExpiresIn            int                    `json:"expiresIn,omitempty"`
	Email                string                 `json:"email,omitempty"`
	APIKey               string                 `json:"apiKey,omitempty"`
	TestStatus           string                 `json:"testStatus,omitempty"`
	LastError            string                 `json:"lastError,omitempty"`
	LastErrorAt          string                 `json:"lastErrorAt,omitempty"`
	RateLimitedUntil     string                 `json:"rateLimitedUntil,omitempty"`
	BackoffLevel         int                    `json:"backoffLevel,omitempty"`
	ConsecutiveUseCount  int                    `json:"consecutiveUseCount,omitempty"`
	ProviderSpecificData map[string]interface{} `json:"providerSpecificData,omitempty"`
	ModelLocks           map[string]string      `json:"modelLocks,omitempty"`
	SupportedModels      []string               `json:"supportedModels,omitempty"`
	BaseURL              string                 `json:"baseUrl,omitempty"`
	CreatedAt            string                 `json:"createdAt,omitempty"`
	UpdatedAt            string                 `json:"updatedAt,omitempty"`
}

type ComboBackup struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Models    []string `json:"models"`
	CreatedAt string   `json:"createdAt,omitempty"`
	UpdatedAt string   `json:"updatedAt,omitempty"`
}

type APIKeyBackup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	IsActive  bool   `json:"isActive"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// === Export ===

func apiExportBackup(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		maskTokens := c.Query("mask") != "false"
		connections := make([]ProviderConnectionBackup, len(cfg.ProviderConnections))
		for i, conn := range cfg.ProviderConnections {
			connections[i] = ProviderConnectionBackup{
				ID:                   conn.ID,
				Provider:             conn.Provider,
				AuthType:             conn.AuthType,
				Name:                 conn.Name,
				Priority:             conn.Priority,
				IsActive:             conn.IsActive,
				AccessToken:          conn.AccessToken,
				RefreshToken:         conn.RefreshToken,
				ExpiresAt:            conn.ExpiresAt,
				ExpiresIn:            conn.ExpiresIn,
				Email:                conn.Email,
				APIKey:               conn.APIKey,
				TestStatus:           conn.TestStatus,
				LastError:            conn.LastError,
				LastErrorAt:          conn.LastErrorAt,
				RateLimitedUntil:     conn.RateLimitedUntil,
				BackoffLevel:         conn.BackoffLevel,
				ConsecutiveUseCount:  conn.ConsecutiveUseCount,
				ProviderSpecificData: conn.ProviderSpecificData,
				ModelLocks:           conn.ModelLocks,
				SupportedModels:      conn.SupportedModels,
				BaseURL:              conn.BaseURL,
				CreatedAt:            conn.CreatedAt,
				UpdatedAt:            conn.UpdatedAt,
			}
			if maskTokens {
				connections[i].AccessToken = maskString(conn.AccessToken, 4, 4)
				connections[i].RefreshToken = maskString(conn.RefreshToken, 4, 4)
				connections[i].APIKey = maskString(conn.APIKey, 4, 4)
			}
		}

		combos := make([]ComboBackup, len(cfg.Combos))
		for i, combo := range cfg.Combos {
			combos[i] = ComboBackup{
				ID:        combo.ID,
				Name:      combo.Name,
				Models:    combo.Models,
				CreatedAt: combo.CreatedAt,
				UpdatedAt: combo.UpdatedAt,
			}
		}

		apiKeys := make([]APIKeyBackup, len(cfg.APIKeys))
		for i, k := range cfg.APIKeys {
			apiKeys[i] = APIKeyBackup{
				ID:        k.ID,
				Name:      k.Name,
				Key:       maskString(k.Key, 10, 4),
				IsActive:  k.IsActive,
				CreatedAt: k.CreatedAt,
			}
		}

		backup := BackupData{
			Version:             backupVersion,
			ExportedAt:          time.Now().UTC().Format(time.RFC3339),
			ProviderConnections: connections,
			Combos:              combos,
			ModelAliases:        cfg.ModelAliases,
			APIKeys:             apiKeys,
			Settings:            cfg.Settings,
		}

		filename := fmt.Sprintf("dntproxy-backup-%s.json", time.Now().Format("20060102-150405"))
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.JSON(200, backup)
	}
}

// === Import ===

func apiImportBackup(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Mode                string                     `json:"mode"`
			Version             string                     `json:"version"`
			ExportedAt          string                     `json:"exportedAt"`
			ProviderConnections []ProviderConnectionBackup `json:"providerConnections"`
			Combos              []ComboBackup              `json:"combos"`
			ModelAliases        domain.AliasMap            `json:"modelAliases"`
			APIKeys             []APIKeyBackup             `json:"apiKeys"`
			Settings            domain.Settings            `json:"settings"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid backup data: " + err.Error()})
			return
		}

		mode := req.Mode
		if mode == "" {
			mode = "merge"
		}
		if mode != "replace" && mode != "merge" {
			c.JSON(400, gin.H{"error": "Invalid mode. Must be 'replace' or 'merge'"})
			return
		}

		backup := BackupData{
			Version:             req.Version,
			ExportedAt:          req.ExportedAt,
			ProviderConnections: req.ProviderConnections,
			Combos:              req.Combos,
			ModelAliases:        req.ModelAliases,
			APIKeys:             req.APIKeys,
			Settings:            req.Settings,
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		imported := 0
		skipped := 0

		if mode == "replace" {
			cfg.ProviderConnections = nil
			cfg.Combos = nil
			cfg.ModelAliases = nil
			cfg.APIKeys = nil
		}

		// Import connections
		for _, conn := range backup.ProviderConnections {
			if conn.ID == "" {
				skipped++
				continue
			}
			imported += importConnection(cfg, conn, mode)
		}

		// Import combos
		for _, combo := range backup.Combos {
			if combo.ID == "" || combo.Name == "" {
				skipped++
				continue
			}
			imported += importCombo(cfg, combo, mode)
		}

		// Import aliases
		if cfg.ModelAliases == nil {
			cfg.ModelAliases = make(domain.AliasMap)
		}
		for alias, model := range backup.ModelAliases {
			cfg.ModelAliases[alias] = model
			imported++
		}

		// Import API keys (only if not masked)
		for _, k := range backup.APIKeys {
			if k.ID == "" || k.Key == "" || strings.HasSuffix(k.Key, "...") {
				skipped++
				continue
			}
			imported += importAPIKey(cfg, k, mode)
		}

		// Import settings (only non-zero values)
		if backup.Settings.Port > 0 {
			cfg.Settings.Port = backup.Settings.Port
		}
		if backup.Settings.ComboStrategy != "" {
			cfg.Settings.ComboStrategy = backup.Settings.ComboStrategy
		}
		if backup.Settings.RequireAPIKey {
			cfg.Settings.RequireAPIKey = true
		}
		if backup.Settings.StickyRoundRobinLimit > 0 {
			cfg.Settings.StickyRoundRobinLimit = backup.Settings.StickyRoundRobinLimit
		}

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		log.Printf("[BACKUP] Imported %d items (%d skipped) in mode=%s", imported, skipped, mode)

		c.JSON(200, gin.H{
			"ok":       true,
			"imported": imported,
			"skipped":  skipped,
			"mode":     mode,
		})
	}
}

func backupConnToDomain(conn ProviderConnectionBackup) domain.ProviderConnection {
	return domain.ProviderConnection{
		ID:                   conn.ID,
		Provider:             conn.Provider,
		AuthType:             conn.AuthType,
		Name:                 conn.Name,
		Priority:             conn.Priority,
		IsActive:             conn.IsActive,
		AccessToken:          conn.AccessToken,
		RefreshToken:         conn.RefreshToken,
		ExpiresAt:            conn.ExpiresAt,
		ExpiresIn:            conn.ExpiresIn,
		Email:                conn.Email,
		APIKey:               conn.APIKey,
		TestStatus:           conn.TestStatus,
		LastError:            conn.LastError,
		LastErrorAt:          conn.LastErrorAt,
		RateLimitedUntil:     conn.RateLimitedUntil,
		BackoffLevel:         conn.BackoffLevel,
		ConsecutiveUseCount:  conn.ConsecutiveUseCount,
		ProviderSpecificData: conn.ProviderSpecificData,
		ModelLocks:           conn.ModelLocks,
		SupportedModels:      conn.SupportedModels,
		BaseURL:              conn.BaseURL,
		CreatedAt:            conn.CreatedAt,
		UpdatedAt:            time.Now().UTC().Format(time.RFC3339),
	}
}

func importConnection(cfg *domain.AppConfig, conn ProviderConnectionBackup, mode string) int {
	dc := backupConnToDomain(conn)

	if mode == "merge" {
		for i, existing := range cfg.ProviderConnections {
			if existing.ID == conn.ID {
				cfg.ProviderConnections[i] = dc
				return 1
			}
		}
	}

	cfg.ProviderConnections = append(cfg.ProviderConnections, dc)
	return 1
}

func importCombo(cfg *domain.AppConfig, combo ComboBackup, mode string) int {
	dc := domain.Combo{
		ID:        combo.ID,
		Name:      combo.Name,
		Models:    combo.Models,
		CreatedAt: combo.CreatedAt,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	for i, existing := range cfg.Combos {
		if existing.ID == combo.ID || existing.Name == combo.Name {
			cfg.Combos[i] = dc
			return 1
		}
	}

	cfg.Combos = append(cfg.Combos, dc)
	return 1
}

func importAPIKey(cfg *domain.AppConfig, k APIKeyBackup, mode string) int {
	dk := domain.APIKey{
		ID:        k.ID,
		Name:      k.Name,
		Key:       k.Key,
		IsActive:  k.IsActive,
		CreatedAt: k.CreatedAt,
	}

	for i, existing := range cfg.APIKeys {
		if existing.ID == k.ID {
			cfg.APIKeys[i] = dk
			return 1
		}
	}

	cfg.APIKeys = append(cfg.APIKeys, dk)
	return 1
}
