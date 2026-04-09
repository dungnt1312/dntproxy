package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/kiro"
	openai "github.com/dungnt/dntproxy/internal/adapter/openai"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/logger"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const backupVersion = "1.0"

// RegisterAPIRoutes adds all /api/* dashboard endpoints.
func RegisterAPIRoutes(r *gin.Engine, store port.CredentialStore) {
	api := r.Group("/api")
	{
		// Connections
		api.GET("/connections", apiListConnections(store))
		api.POST("/connections/import", apiImportConnection(store))
		api.POST("/connections/add-openai", apiAddOpenAIConnection(store))
		api.POST("/connections/add-custom", apiAddCustomConnection(store))
		api.POST("/connections/detect-kiro", apiDetectKiroToken(store))
		api.DELETE("/connections/:id", apiDeleteConnection(store))
		api.POST("/connections/:id/test", apiTestConnection(store))
		api.PUT("/connections/:id", apiUpdateConnection(store))
		api.POST("/connections/:id/reset-cooldown", apiResetCooldown(store))
		api.POST("/connections/:id/check-quota", apiCheckQuota(store))
		api.POST("/connections/:id/test-model", apiTestModel(store))

		// Combos
		api.GET("/combos", apiListCombos(store))
		api.POST("/combos", apiCreateCombo(store))
		api.PUT("/combos/:id", apiUpdateCombo(store))
		api.DELETE("/combos/:id", apiDeleteCombo(store))

		// Aliases
		api.GET("/aliases", apiListAliases(store))
		api.POST("/aliases", apiSetAlias(store))
		api.DELETE("/aliases/:name", apiDeleteAlias(store))

		// API Keys
		api.GET("/keys", apiListKeys(store))
		api.POST("/keys", apiCreateKey(store))
		api.DELETE("/keys/:id", apiDeleteKey(store))

		// Settings
		api.GET("/settings", apiGetSettings(store))
		api.PUT("/settings", apiUpdateSettings(store))

		// Models
		api.GET("/models", apiListModels(store))

		// Logs
		api.GET("/logs", apiGetLogs)
		api.GET("/logs/stream", apiLogStream)
		api.POST("/logs/clear", apiClearLogs)

		// Backup
		api.GET("/backup/export", apiExportBackup(store))
		api.POST("/backup/import", apiImportBackup(store))
	}
}

// === Connections ===

func apiListConnections(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// Mask tokens before returning
		type safeConn struct {
			ID              string            `json:"id"`
			Provider        string            `json:"provider"`
			AuthType        string            `json:"authType"`
			Name            string            `json:"name"`
			Priority        int               `json:"priority"`
			IsActive        bool              `json:"isActive"`
			Email           string            `json:"email,omitempty"`
			ExpiresAt       string            `json:"expiresAt,omitempty"`
			TestStatus      string            `json:"testStatus,omitempty"`
			LastError       string            `json:"lastError,omitempty"`
			LastErrorAt     string            `json:"lastErrorAt,omitempty"`
			RateLimited     string            `json:"rateLimitedUntil,omitempty"`
			BackoffLevel    int               `json:"backoffLevel"`
			AuthMethod      string            `json:"authMethod,omitempty"`
			ProviderName    string            `json:"providerName,omitempty"`
			ModelLocks      map[string]string `json:"modelLocks,omitempty"`
			SupportedModels []string          `json:"supportedModels,omitempty"`
			BaseURL         string            `json:"baseUrl,omitempty"`
			CreatedAt       string            `json:"createdAt,omitempty"`
			HasToken        bool              `json:"hasToken"`
			HasAPIKey       bool              `json:"hasApiKey"`
			ExpiresIn       int               `json:"expiresIn,omitempty"`
		}

		var result []safeConn
		for _, conn := range cfg.ProviderConnections {
			sc := safeConn{
				ID:              conn.ID,
				Provider:        conn.Provider,
				AuthType:        conn.AuthType,
				Name:            conn.Name,
				Priority:        conn.Priority,
				IsActive:        conn.IsActive,
				Email:           conn.Email,
				ExpiresAt:       conn.ExpiresAt,
				TestStatus:      conn.TestStatus,
				LastError:       conn.LastError,
				LastErrorAt:     conn.LastErrorAt,
				RateLimited:     conn.RateLimitedUntil,
				BackoffLevel:    conn.BackoffLevel,
				ModelLocks:      conn.ModelLocks,
				SupportedModels: conn.SupportedModels,
				BaseURL:         conn.BaseURL,
				CreatedAt:       conn.CreatedAt,
				HasToken:        conn.AccessToken != "",
				HasAPIKey:       conn.APIKey != "",
				ExpiresIn:       conn.ExpiresIn,
			}
			if conn.ProviderSpecificData != nil {
				if m, ok := conn.ProviderSpecificData["authMethod"].(string); ok {
					sc.AuthMethod = m
				}
				if p, ok := conn.ProviderSpecificData["provider"].(string); ok {
					sc.ProviderName = p
				}
			}
			result = append(result, sc)
		}

		c.JSON(200, result)
	}
}

func apiImportConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refreshToken"`
			ClientID     string `json:"clientId,omitempty"`
			ClientSecret string `json:"clientSecret,omitempty"`
			Region       string `json:"region,omitempty"`
			AuthMethod   string `json:"authMethod,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}
		if req.RefreshToken == "" {
			c.JSON(400, gin.H{"error": "refreshToken is required"})
			return
		}

		result, err := auth.ValidateAndImportToken(req.RefreshToken, req.ClientID, req.ClientSecret, req.Region, req.AuthMethod)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		email := auth.ExtractEmailFromJWT(result.AccessToken)

		providerLabel := "AWS Builder ID"
		switch result.AuthMethod {
		case "idc":
			providerLabel = "AWS IAM Identity Center"
		case "google":
			providerLabel = "Google"
		case "github":
			providerLabel = "GitHub"
		case "imported":
			providerLabel = "Imported"
		}

		name := email
		cfg, _ := store.Load()
		if name == "" {
			name = providerLabel + " Account"
			if cfg != nil {
				name += " " + string(rune('1'+len(cfg.ProviderConnections)))
			}
		}

		now := time.Now().UTC().Format(time.RFC3339)
		expiresIn := result.ExpiresIn
		if expiresIn == 0 {
			expiresIn = 3600
		}

		conn := domain.ProviderConnection{
			ID:           uuid.New().String(),
			Provider:     "kiro",
			AuthType:     "oauth",
			Name:         name,
			Priority:     len(cfg.ProviderConnections) + 1,
			IsActive:     true,
			AccessToken:  result.AccessToken,
			RefreshToken: result.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
			ExpiresIn:    expiresIn,
			Email:        email,
			TestStatus:   "active",
			ProviderSpecificData: map[string]interface{}{
				"profileArn": result.ProfileArn,
				"authMethod": result.AuthMethod,
				"provider":   providerLabel,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if result.ClientID != "" {
			conn.ProviderSpecificData["clientId"] = result.ClientID
		}
		if result.ClientSecret != "" {
			conn.ProviderSpecificData["clientSecret"] = result.ClientSecret
		}
		if result.Region != "" {
			conn.ProviderSpecificData["region"] = result.Region
		}

		cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"id": conn.ID, "name": conn.Name, "email": email})
	}
}

func apiDeleteConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		found := false
		for i, conn := range cfg.ProviderConnections {
			if conn.ID == id {
				cfg.ProviderConnections = append(cfg.ProviderConnections[:i], cfg.ProviderConnections[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

func apiTestConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		var conn *domain.ProviderConnection
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == id {
				conn = &cfg.ProviderConnections[i]
				break
			}
		}
		if conn == nil {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}

		refreshSvc := auth.NewTokenRefreshService(store)
		if refreshSvc.NeedsRefresh(conn) {
			refreshed, err := refreshSvc.Refresh(conn)
			if err != nil {
				c.JSON(200, gin.H{"status": "error", "message": "Token refresh failed: " + err.Error()})
				return
			}
			*conn = *refreshed
			store.Save(cfg)
		}

		email := ""
		if conn.AccessToken != "" {
			email = auth.ExtractEmailFromJWT(conn.AccessToken)
		}

		c.JSON(200, gin.H{
			"status":    "ok",
			"hasToken":  conn.AccessToken != "",
			"expiresAt": conn.ExpiresAt,
			"email":     email,
		})
	}
}

func apiUpdateConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name            *string  `json:"name,omitempty"`
			IsActive        *bool    `json:"isActive,omitempty"`
			Priority        *int     `json:"priority,omitempty"`
			SupportedModels []string `json:"supportedModels,omitempty"`
			SetModels       bool     `json:"setModels,omitempty"`
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

		var conn *domain.ProviderConnection
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == id {
				conn = &cfg.ProviderConnections[i]
				break
			}
		}
		if conn == nil {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}

		if req.Name != nil {
			conn.Name = *req.Name
		}
		if req.IsActive != nil {
			conn.IsActive = *req.IsActive
		}
		if req.Priority != nil {
			conn.Priority = *req.Priority
		}
		if req.SetModels {
			conn.SupportedModels = req.SupportedModels
		}
		conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

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
		if len(req.Models) == 0 && len(req.ConnectionIDs) == 0 {
			c.JSON(400, gin.H{"error": "at least one model or connection required"})
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
			Key       string `json:"key"`
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
				Key:       k.Key,
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

// === Settings ===

func apiGetSettings(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, cfg.Settings)
	}
}

func apiUpdateSettings(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.Settings
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid settings"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		cfg.Settings = req
		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, cfg.Settings)
	}
}

// === Models ===

func apiListModels(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		allModels := []gin.H{
			// Kiro models
			{"id": "kr/claude-sonnet-4.5", "name": "Claude Sonnet 4.5", "provider": "kiro"},
			{"id": "kr/claude-haiku-4.5", "name": "Claude Haiku 4.5", "provider": "kiro"},
			{"id": "kr/deepseek-3.2", "name": "DeepSeek 3.2", "provider": "kiro"},
			{"id": "kr/deepseek-3.1", "name": "DeepSeek 3.1", "provider": "kiro"},
			{"id": "kr/qwen3-coder-next", "name": "Qwen3 Coder Next", "provider": "kiro"},
			// OpenAI models
			{"id": "oai/gpt-4.1", "name": "GPT-4.1", "provider": "openai"},
			{"id": "oai/gpt-4.1-mini", "name": "GPT-4.1 Mini", "provider": "openai"},
			{"id": "oai/gpt-4.1-nano", "name": "GPT-4.1 Nano", "provider": "openai"},
			{"id": "oai/gpt-4o", "name": "GPT-4o", "provider": "openai"},
			{"id": "oai/gpt-4o-mini", "name": "GPT-4o Mini", "provider": "openai"},
			{"id": "oai/o3", "name": "o3", "provider": "openai"},
			{"id": "oai/o3-mini", "name": "o3-mini", "provider": "openai"},
			{"id": "oai/o4-mini", "name": "o4-mini", "provider": "openai"},
		}

		cfg, _ := store.Load()
		if cfg != nil {
			for alias, model := range cfg.ModelAliases {
				allModels = append(allModels, gin.H{"id": alias, "name": alias + " → " + model, "provider": "alias"})
			}
			for _, combo := range cfg.Combos {
				allModels = append(allModels, gin.H{"id": combo.Name, "name": combo.Name, "provider": "combo", "models": combo.Models})
			}
		}

		c.JSON(200, allModels)
	}
}

// maskString masks a string, showing first n and last m characters.
func maskString(s string, first, last int) string {
	if len(s) <= first+last {
		return strings.Repeat("*", len(s))
	}
	return s[:first] + "..." + s[len(s)-last:]
}

// === Detect Kiro Token ===

func apiDetectKiroToken(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Search for kiro-auth-token.json in known locations
		candidates := getKiroTokenPaths()

		for _, path := range candidates {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			var tokenFile struct {
				AccessToken  string `json:"accessToken"`
				RefreshToken string `json:"refreshToken"`
				ExpiresAt    string `json:"expiresAt"`
				AuthMethod   string `json:"authMethod"`
				Provider     string `json:"provider"`
				Region       string `json:"region"`
				ClientID     string `json:"clientId"`
				ClientSecret string `json:"clientSecret"`
				ClientIDHash string `json:"clientIdHash"`
			}
			if err := json.Unmarshal(data, &tokenFile); err != nil {
				continue
			}

			if tokenFile.RefreshToken == "" {
				continue
			}

			// If clientId is missing but clientIdHash exists,
			// try to read the cached client registration file: ~/.aws/sso/cache/{clientIdHash}.json
			if tokenFile.ClientID == "" && tokenFile.ClientIDHash != "" {
				cacheDir := filepath.Dir(path)
				clientCachePath := filepath.Join(cacheDir, tokenFile.ClientIDHash+".json")
				if clientData, err := os.ReadFile(clientCachePath); err == nil {
					var clientFile struct {
						ClientID     string `json:"clientId"`
						ClientSecret string `json:"clientSecret"`
					}
					if err := json.Unmarshal(clientData, &clientFile); err == nil {
						tokenFile.ClientID = clientFile.ClientID
						tokenFile.ClientSecret = clientFile.ClientSecret
					}
				}
			}

			c.JSON(200, gin.H{
				"found":        true,
				"path":         path,
				"refreshToken": tokenFile.RefreshToken,
				"authMethod":   strings.ToLower(tokenFile.AuthMethod),
				"provider":     tokenFile.Provider,
				"region":       tokenFile.Region,
				"clientId":     tokenFile.ClientID,
				"clientSecret": tokenFile.ClientSecret,
				"expiresAt":    tokenFile.ExpiresAt,
			})
			return
		}

		c.JSON(200, gin.H{
			"found": false,
			"error": "No Kiro token found. Searched: " + strings.Join(candidates, ", "),
		})
	}
}

func getKiroTokenPaths() []string {
	home, _ := os.UserHomeDir()
	var paths []string

	// Primary: ~/.aws/sso/cache/kiro-auth-token.json
	paths = append(paths, filepath.Join(home, ".aws", "sso", "cache", "kiro-auth-token.json"))

	// Windows-specific paths
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "Kiro", "kiro-auth-token.json"))
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "Kiro", "kiro-auth-token.json"))
		}
	}

	// macOS
	if runtime.GOOS == "darwin" {
		paths = append(paths, filepath.Join(home, "Library", "Application Support", "Kiro", "kiro-auth-token.json"))
	}

	// Linux
	if runtime.GOOS == "linux" {
		paths = append(paths, filepath.Join(home, ".config", "kiro", "kiro-auth-token.json"))
	}

	return paths
}

// === OpenAI Connection ===

func apiAddOpenAIConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name            string   `json:"name"`
			APIKey          string   `json:"apiKey"`
			SupportedModels []string `json:"supportedModels,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.APIKey == "" {
			c.JSON(400, gin.H{"error": "apiKey is required"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		name := req.Name
		if name == "" {
			name = "OpenAI Account"
			if cfg != nil {
				count := 0
				for _, conn := range cfg.ProviderConnections {
					if conn.Provider == "openai" {
						count++
					}
				}
				if count > 0 {
					name = fmt.Sprintf("OpenAI Account %d", count+1)
				}
			}
		}

		now := time.Now().UTC().Format(time.RFC3339)
		conn := domain.ProviderConnection{
			ID:              uuid.New().String(),
			Provider:        "openai",
			AuthType:        "apikey",
			Name:            name,
			Priority:        len(cfg.ProviderConnections) + 1,
			IsActive:        true,
			APIKey:          req.APIKey,
			TestStatus:      "active",
			SupportedModels: req.SupportedModels,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"id": conn.ID, "name": conn.Name})
	}
}

// === Custom OpenAI Compatible Connection ===

func apiAddCustomConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name            string   `json:"name"`
			APIKey          string   `json:"apiKey"`
			BaseURL         string   `json:"baseUrl"`
			SupportedModels []string `json:"supportedModels,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.BaseURL == "" {
			c.JSON(400, gin.H{"error": "baseUrl is required"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		name := req.Name
		if name == "" {
			name = "Custom API"
			if cfg != nil {
				count := 0
				for _, conn := range cfg.ProviderConnections {
					if conn.Provider == "openai-compatible" {
						count++
					}
				}
				if count > 0 {
					name = fmt.Sprintf("Custom API %d", count+1)
				}
			}
		}

		now := time.Now().UTC().Format(time.RFC3339)
		conn := domain.ProviderConnection{
			ID:              uuid.New().String(),
			Provider:        "openai-compatible",
			AuthType:        "apikey",
			Name:            name,
			Priority:        len(cfg.ProviderConnections) + 1,
			IsActive:        true,
			APIKey:          req.APIKey,
			BaseURL:         req.BaseURL,
			TestStatus:      "active",
			SupportedModels: req.SupportedModels,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"id": conn.ID, "name": conn.Name})
	}
}

// === Reset Cooldown ===

func apiResetCooldown(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		var conn *domain.ProviderConnection
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == id {
				conn = &cfg.ProviderConnections[i]
				break
			}
		}
		if conn == nil {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}

		conn.RateLimitedUntil = ""
		conn.BackoffLevel = 0
		conn.LastError = ""
		conn.LastErrorAt = ""
		conn.ModelLocks = nil
		conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

// === Test Model ===
// Sends a minimal chat request ("Hi") with a specific model through a specific connection.
// Used to verify that a model works with a particular connection/credentials.

func apiTestModel(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Model string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
			c.JSON(400, gin.H{"error": "model is required"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		var conn *domain.ProviderConnection
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == id {
				conn = &cfg.ProviderConnections[i]
				break
			}
		}
		if conn == nil {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}

		refreshSvc := auth.NewTokenRefreshService(store)
		if refreshSvc.NeedsRefresh(conn) {
			refreshed, err := refreshSvc.Refresh(conn)
			if err != nil {
				c.JSON(200, gin.H{"status": "error", "message": "Token refresh failed: " + err.Error()})
				return
			}
			*conn = *refreshed
			store.Save(cfg)
		}

		creds := connectionToCreds(conn)

		testBody := map[string]interface{}{
			"model": req.Model,
			"messages": []map[string]string{
				{"role": "user", "content": "Hi"},
			},
			"stream":     false,
			"max_tokens": 5,
		}
		bodyBytes, _ := json.Marshal(testBody)

		provider := conn.Provider
		var executor port.ProviderExecutor
		switch provider {
		case "kiro":
			executor = kiro.NewExecutor()
		case "openai", "openai-compatible":
			executor = openai.NewExecutor()
		default:
			c.JSON(400, gin.H{"status": "error", "message": "Unsupported provider: " + provider})
			return
		}

		stream, statusCode, execErr := executor.Execute(req.Model, bodyBytes, creds)
		if stream != nil {
			stream.Close()
		}

		if execErr != nil {
			c.JSON(200, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("HTTP %d: %s", statusCode, execErr.Error()),
				"code":    statusCode,
			})
			return
		}

		if statusCode != 200 {
			c.JSON(200, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("Upstream returned HTTP %d", statusCode),
				"code":    statusCode,
			})
			return
		}

		c.JSON(200, gin.H{
			"status": "ok",
			"model":  req.Model,
		})
	}
}

func connectionToCreds(conn *domain.ProviderConnection) *domain.Credentials {
	creds := &domain.Credentials{
		ConnectionID:         conn.ID,
		ConnectionName:       conn.Name,
		AccessToken:          conn.AccessToken,
		RefreshToken:         conn.RefreshToken,
		APIKey:               conn.APIKey,
		BaseURL:              conn.BaseURL,
		ProviderSpecificData: conn.ProviderSpecificData,
	}
	if conn.ProviderSpecificData != nil {
		if v, ok := conn.ProviderSpecificData["profileArn"]; ok {
			if s, ok := v.(string); ok {
				creds.ProfileArn = s
			}
		}
	}
	return creds
}

// === Quota Check ===
// Makes a lightweight upstream call and reads rate-limit headers.
// For OpenAI: GET /v1/models → reads x-ratelimit-* headers.
// For Kiro: returns token expiry info only (no quota API available).

func apiCheckQuota(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		var conn *domain.ProviderConnection
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == id {
				conn = &cfg.ProviderConnections[i]
				break
			}
		}
		if conn == nil {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}

		result := gin.H{
			"provider": conn.Provider,
			"name":     conn.Name,
		}

		// For Kiro: Fetch actual usage limits from Amazon Q / CodeWhisperer
		if conn.Provider == "kiro" {
			if conn.ExpiresAt != "" {
				expTime, parseErr := time.Parse(time.RFC3339, conn.ExpiresAt)
				if parseErr == nil {
					secsLeft := int(time.Until(expTime).Seconds())
					pct := 0
					if conn.ExpiresIn > 0 {
						pct = secsLeft * 100 / conn.ExpiresIn
						if pct < 0 {
							pct = 0
						}
						if pct > 100 {
							pct = 100
						}
					}
					result["tokenSecsLeft"] = secsLeft
					result["tokenPct"] = pct
					result["expiresAt"] = conn.ExpiresAt
					result["expired"] = secsLeft <= 0
				}
			}

			profileArn := ""
			if conn.ProviderSpecificData != nil {
				if v, ok := conn.ProviderSpecificData["profileArn"].(string); ok {
					profileArn = v
				}
			}

			client := &http.Client{Timeout: 10 * time.Second}
			var qResp *http.Response
			var err error

			if profileArn == "" {
				req, _ := http.NewRequest("GET", "https://q.us-east-1.amazonaws.com/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST", nil)
				req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
				req.Header.Set("Accept", "application/json")
				qResp, err = client.Do(req)
			} else {
				body := fmt.Sprintf(`{"origin":"AI_EDITOR","profileArn":"%s","resourceType":"AGENTIC_REQUEST"}`, profileArn)
				req, _ := http.NewRequest("POST", "https://codewhisperer.us-east-1.amazonaws.com", strings.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
				req.Header.Set("Content-Type", "application/x-amz-json-1.0")
				req.Header.Set("x-amz-target", "AmazonCodeWhispererService.GetUsageLimits")
				req.Header.Set("Accept", "application/json")
				qResp, err = client.Do(req)
				if err == nil && (qResp.StatusCode != 200 && qResp.StatusCode != 401 && qResp.StatusCode != 403) {
					qResp.Body.Close()
					// Fallback to Q endpoint
					qUrl := fmt.Sprintf("https://q.us-east-1.amazonaws.com/getUsageLimits?origin=AI_EDITOR&profileArn=%s&resourceType=AGENTIC_REQUEST", profileArn)
					req2, _ := http.NewRequest("GET", qUrl, nil)
					req2.Header.Set("Authorization", "Bearer "+conn.AccessToken)
					req2.Header.Set("Accept", "application/json")
					qResp, err = client.Do(req2)
				}
			}

			result["quotaSupported"] = false // default, will be overridden if we parse info

			if err == nil && qResp != nil {
				defer qResp.Body.Close()
				qBodyBytes, _ := io.ReadAll(qResp.Body)
				fmt.Printf("[Kiro Quota] ProfileArn: %q, StatusCode: %d, Body: %s\n", profileArn, qResp.StatusCode, string(qBodyBytes))
				if qResp.StatusCode == 200 {
					var data map[string]interface{}
					if json.Unmarshal(qBodyBytes, &data) == nil {
						if list, ok := data["usageBreakdownList"].([]interface{}); ok && len(list) > 0 {
							for _, obj := range list {
								if b, ok := obj.(map[string]interface{}); ok {
									resType, _ := b["resourceType"].(string)
									if resType == "" {
										if disp, ok := b["displayName"].(string); ok {
											resType = disp // Fallback if no resourceType
										}
									}
									used, _ := b["currentUsageWithPrecision"].(float64)
									limit, _ := b["usageLimitWithPrecision"].(float64)

									// Map AGENTIC_REQUEST or CREDIT to "requests"
									if strings.EqualFold(resType, "AGENTIC_REQUEST") || strings.EqualFold(resType, "CREDIT") {
										result["requestsRemaining"] = int(limit - used)
										result["requestsLimit"] = int(limit)
										if limit > 0 {
											pct := int((used / limit) * 100)
											if pct > 100 {
												pct = 100
											}
											result["requestsPct"] = pct
										}
										result["quotaSupported"] = true

										if ft, ok := b["freeTrialInfo"].(map[string]interface{}); ok && ft != nil {
											if ftStatus, _ := ft["freeTrialStatus"].(string); ftStatus == "ACTIVE" {
												ftUsed, _ := ft["currentUsageWithPrecision"].(float64)
												ftLimit, _ := ft["usageLimitWithPrecision"].(float64)
												result["freeTrialRemaining"] = int(ftLimit - ftUsed)
												result["freeTrialLimit"] = int(ftLimit)
												if ftLimit > 0 {
													pct := int((ftUsed / ftLimit) * 100)
													if pct > 100 {
														pct = 100
													}
													result["freeTrialPct"] = pct
												}
												if ftExpiry, _ := ft["freeTrialExpiry"].(float64); ftExpiry > 0 {
													result["freeTrialExpiresAt"] = time.Unix(int64(ftExpiry), 0).Format("2006-01-02 15:04")
												}
											}
										}

										if oc, _ := b["overageCharges"].(float64); oc > 0 {
											result["overageCharges"] = oc
										}

									} else if strings.EqualFold(resType, "INLINE_INVOCATION") || strings.EqualFold(resType, "CHAT_REQUEST") {
										// Secondary quota check map to "tokens" in panel just to visualize
										result["tokensRemaining"] = int(limit - used)
										result["tokensLimit"] = int(limit)
										if limit > 0 {
											pct := int((used / limit) * 100)
											if pct > 100 {
												pct = 100
											}
											result["tokensPct"] = pct
										}
										result["quotaSupported"] = true
									}
								}
							}
						}
						// Try multiple reset fields
						if resetStr, ok := data["nextDateReset"].(string); ok {
							result["resetRequests"] = resetStr
						} else if resetNum, ok := data["nextDateReset"].(float64); ok {
							result["resetRequests"] = time.Unix(int64(resetNum), 0).Format("2006-01-02 15:04")
						} else if resetStr, ok := data["resetDate"].(string); ok {
							result["resetRequests"] = resetStr
						} else if resetNum, ok := data["resetDate"].(float64); ok {
							result["resetRequests"] = time.Unix(int64(resetNum), 0).Format("2006-01-02 15:04")
						}
					}
				}
			}

			c.JSON(200, result)
			return
		}

		// For OpenAI OAuth (ChatGPT web)
		// It doesn't support /v1/models or rate limit headers.
		if conn.Provider == "openai" && conn.AuthType == "oauth" {
			if conn.ExpiresAt != "" {
				expTime, parseErr := time.Parse(time.RFC3339, conn.ExpiresAt)
				if parseErr == nil {
					secsLeft := int(time.Until(expTime).Seconds())
					elapsed := 0
					pct := 0
					if conn.ExpiresIn > 0 {
						elapsed = conn.ExpiresIn - secsLeft
						if elapsed < 0 {
							elapsed = 0
						}
						pct = elapsed * 100 / conn.ExpiresIn
						if pct < 0 {
							pct = 0
						}
						if pct > 100 {
							pct = 100
						}
					} else {
						// Fallback if no ExpiresIn
						elapsed = 3600 - secsLeft
						if elapsed < 0 {
							elapsed = 0
						}
					}
					// The UI currently uses tokenSecsLeft as the "used" value and tokenPct as the "used percentage"
					result["tokenSecsLeft"] = elapsed
					result["tokenPct"] = pct
					result["expiresAt"] = conn.ExpiresAt
					result["expired"] = secsLeft <= 0
				}
			}

			// Try to refresh if expired
			if expired, ok := result["expired"].(bool); ok && expired && conn.RefreshToken != "" {
				updatedConn, refErr := auth.RefreshOpenAIToken(conn, store)
				if refErr == nil {
					conn = updatedConn
					result["expired"] = false
					result["tokenSecsLeft"] = conn.ExpiresIn
					result["tokenPct"] = 100
					result["expiresAt"] = conn.ExpiresAt
				} else {
					c.JSON(401, gin.H{"error": "Token expired and auto-refresh failed: " + refErr.Error()})
					return
				}
			}

			result["quotaSupported"] = false
			c.JSON(200, result)
			return
		}

		// For OpenAI API Key / OpenAI-compatible: call /v1/models and read rate-limit headers
		baseURL := conn.BaseURL
		if baseURL == "" && conn.Provider == "openai" {
			baseURL = "https://api.openai.com"
		}
		if baseURL == "" {
			c.JSON(400, gin.H{"error": "No base URL configured"})
			return
		}

		req, _ := http.NewRequest("GET", baseURL+"/v1/models", nil)
		if conn.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+conn.APIKey)
		} else if conn.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
		}
		req.Header.Set("User-Agent", "dntproxy/1.0")

		httpClient := &http.Client{Timeout: 10 * time.Second}
		resp, err := httpClient.Do(req)
		if err != nil {
			c.JSON(502, gin.H{"error": "Request failed: " + err.Error()})
			return
		}

		if (resp.StatusCode == 401 || resp.StatusCode == 403) && conn.Provider == "openai" && conn.RefreshToken != "" {
			resp.Body.Close() // close failed response
			updatedConn, refErr := auth.RefreshOpenAIToken(conn, store)
			if refErr == nil {
				conn = updatedConn
				req, _ = http.NewRequest("GET", baseURL+"/v1/models", nil)
				req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
				req.Header.Set("User-Agent", "dntproxy/1.0")
				resp, err = httpClient.Do(req)
				if err != nil {
					c.JSON(502, gin.H{"error": "Retry failed: " + err.Error()})
					return
				}
			} else {
				c.JSON(resp.StatusCode, gin.H{"error": "Token expired and auto-refresh failed: " + refErr.Error()})
				return
			}
		}
		defer resp.Body.Close()
		io.ReadAll(resp.Body) // drain

		// Parse rate-limit headers
		parseHeader := func(h string) int {
			v := resp.Header.Get(h)
			if v == "" {
				return -1
			}
			n := 0
			fmt.Sscanf(v, "%d", &n)
			return n
		}

		result["quotaSupported"] = true
		result["statusCode"] = resp.StatusCode

		reqLimit := parseHeader("x-ratelimit-limit-requests")
		reqRemaining := parseHeader("x-ratelimit-remaining-requests")
		tokLimit := parseHeader("x-ratelimit-limit-tokens")
		tokRemaining := parseHeader("x-ratelimit-remaining-tokens")

		if reqLimit >= 0 {
			result["requestsLimit"] = reqLimit
			result["requestsRemaining"] = reqRemaining
			if reqLimit > 0 {
				result["requestsPct"] = reqRemaining * 100 / reqLimit
			}
		}
		if tokLimit >= 0 {
			result["tokensLimit"] = tokLimit
			result["tokensRemaining"] = tokRemaining
			if tokLimit > 0 {
				result["tokensPct"] = tokRemaining * 100 / tokLimit
			}
		}

		// Also capture reset times
		if v := resp.Header.Get("x-ratelimit-reset-requests"); v != "" {
			result["resetRequests"] = v
		}
		if v := resp.Header.Get("x-ratelimit-reset-tokens"); v != "" {
			result["resetTokens"] = v
		}

		if reqLimit < 0 && tokLimit < 0 {
			result["note"] = "No rate-limit headers returned by upstream"
		}

		c.JSON(200, result)
	}
}

// === Logs ===

func apiGetLogs(c *gin.Context) {
	appLogger := logger.Get()
	logs := appLogger.GetAll()
	c.JSON(200, logs)
}

func apiLogStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	appLogger := logger.Get()
	ch := appLogger.Subscribe()
	defer appLogger.Unsubscribe(ch)

	logs := appLogger.GetAll()
	data, _ := json.Marshal(logs)
	c.Writer.Write([]byte("data: " + string(data) + "\n\n"))
	c.Writer.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			return
		case logs := <-ch:
			data, _ := json.Marshal(logs)
			c.Writer.Write([]byte("data: " + string(data) + "\n\n"))
			c.Writer.Flush()
		case <-ticker.C:
			c.Writer.Write([]byte(": keepalive\n\n"))
			c.Writer.Flush()
		}
	}
}

func apiClearLogs(c *gin.Context) {
	appLogger := logger.Get()
	appLogger.Clear()
	log.Printf("[LOG] Logs cleared by admin")
	c.JSON(200, gin.H{"ok": true})
}

// === Backup ===

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

func apiExportBackup(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// Backup connections (mask sensitive data option)
		maskTokens := c.Query("mask") == "true"
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

		// Backup combos
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

		// Backup API keys (mask actual key)
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

func apiImportBackup(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Mode string `json:"mode"` // "replace" or "merge"
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}

		if req.Mode == "" {
			req.Mode = "merge"
		}
		if req.Mode != "replace" && req.Mode != "merge" {
			c.JSON(400, gin.H{"error": "Invalid mode. Must be 'replace' or 'merge'"})
			return
		}

		var backup BackupData
		if err := c.ShouldBindJSON(&backup); err != nil {
			c.JSON(400, gin.H{"error": "Invalid backup data: " + err.Error()})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		imported := 0
		skipped := 0

		if req.Mode == "replace" {
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

			if req.Mode == "merge" {
				found := false
				for i, existing := range cfg.ProviderConnections {
					if existing.ID == conn.ID {
						cfg.ProviderConnections[i] = domain.ProviderConnection{
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
						found = true
						imported++
						break
					}
				}
				if !found {
					cfg.ProviderConnections = append(cfg.ProviderConnections, domain.ProviderConnection{
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
					})
					imported++
				}
			} else {
				cfg.ProviderConnections = append(cfg.ProviderConnections, domain.ProviderConnection{
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
				})
				imported++
			}
		}

		// Import combos
		for _, combo := range backup.Combos {
			if combo.ID == "" || combo.Name == "" {
				skipped++
				continue
			}

			found := false
			for i, existing := range cfg.Combos {
				if existing.ID == combo.ID || existing.Name == combo.Name {
					cfg.Combos[i] = domain.Combo{
						ID:        combo.ID,
						Name:      combo.Name,
						Models:    combo.Models,
						CreatedAt: combo.CreatedAt,
						UpdatedAt: time.Now().UTC().Format(time.RFC3339),
					}
					found = true
					imported++
					break
				}
			}
			if !found {
				cfg.Combos = append(cfg.Combos, domain.Combo{
					ID:        combo.ID,
					Name:      combo.Name,
					Models:    combo.Models,
					CreatedAt: combo.CreatedAt,
					UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				})
				imported++
			}
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

			found := false
			for i, existing := range cfg.APIKeys {
				if existing.ID == k.ID {
					cfg.APIKeys[i] = domain.APIKey{
						ID:        k.ID,
						Name:      k.Name,
						Key:       k.Key,
						IsActive:  k.IsActive,
						CreatedAt: k.CreatedAt,
					}
					found = true
					imported++
					break
				}
			}
			if !found {
				cfg.APIKeys = append(cfg.APIKeys, domain.APIKey{
					ID:        k.ID,
					Name:      k.Name,
					Key:       k.Key,
					IsActive:  k.IsActive,
					CreatedAt: k.CreatedAt,
				})
				imported++
			}
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

		log.Printf("[BACKUP] Imported %d items (%d skipped) in mode=%s", imported, skipped, req.Mode)

		c.JSON(200, gin.H{
			"ok":       true,
			"imported": imported,
			"skipped":  skipped,
			"mode":     req.Mode,
		})
	}
}
