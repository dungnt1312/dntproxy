package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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

		// Combos
		api.GET("/combos", apiListCombos(store))
		api.POST("/combos", apiCreateCombo(store))
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
			AuthMethod      string            `json:"authMethod,omitempty"`
			ProviderName    string            `json:"providerName,omitempty"`
			ModelLocks      map[string]string `json:"modelLocks,omitempty"`
			SupportedModels []string          `json:"supportedModels,omitempty"`
			BaseURL         string            `json:"baseUrl,omitempty"`
			CreatedAt       string            `json:"createdAt,omitempty"`
			HasToken        bool              `json:"hasToken"`
			HasAPIKey       bool              `json:"hasApiKey"`
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
				ModelLocks:      conn.ModelLocks,
				SupportedModels: conn.SupportedModels,
				BaseURL:         conn.BaseURL,
				CreatedAt:   conn.CreatedAt,
				HasToken:    conn.AccessToken != "",
				HasAPIKey:   conn.APIKey != "",
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
			ID:          uuid.New().String(),
			Provider:    "kiro",
			AuthType:    "oauth",
			Name:        name,
			Priority:    len(cfg.ProviderConnections) + 1,
			IsActive:    true,
			AccessToken: result.AccessToken,
			RefreshToken: result.RefreshToken,
			ExpiresAt:   time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
			ExpiresIn:   expiresIn,
			Email:       email,
			TestStatus:  "active",
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
			Name   string   `json:"name"`
			Models []string `json:"models"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || len(req.Models) == 0 {
			c.JSON(400, gin.H{"error": "name and models[] required"})
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
			ID:        uuid.New().String(),
			Name:      req.Name,
			Models:    req.Models,
			CreatedAt: now,
			UpdatedAt: now,
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
