package http

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/kiro"
	openai "github.com/dungnt/dntproxy/internal/adapter/openai"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// === List Connections ===

func apiListConnections(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
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

// === Import Connection ===

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
			ID:              uuid.New().String(),
			Provider:        "kiro",
			AuthType:        "oauth",
			Name:            name,
			Priority:        len(cfg.ProviderConnections) + 1,
			IsActive:        true,
			AccessToken:     result.AccessToken,
			RefreshToken:    result.RefreshToken,
			ExpiresAt:       time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
			ExpiresIn:       expiresIn,
			Email:           email,
			TestStatus:      "active",
			SupportedModels: domain.GetProviderConfig("kiro").DefaultModels,
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

// === Delete Connection ===

func apiDeleteConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		found := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			for i, conn := range cfg.ProviderConnections {
				if conn.ID == id {
					cfg.ProviderConnections = append(cfg.ProviderConnections[:i], cfg.ProviderConnections[i+1:]...)
					found = true
					break
				}
			}
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

// === Test Connection ===

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

		// For OpenAI OAuth connections, probe the Codex Responses API
		if conn.Provider == "openai" && conn.AuthType == "oauth" && conn.AccessToken != "" {
			probeResult := probeCodexAPI(conn.AccessToken)
			if !probeResult.valid {
				// Try refresh if we have a refresh token
				if conn.RefreshToken != "" {
					refreshed, refErr := refreshSvc.Refresh(conn)
					if refErr == nil {
						*conn = *refreshed
						store.Save(cfg)
						probeResult = probeCodexAPI(conn.AccessToken)
					}
				}
			}
			if probeResult.valid {
				c.JSON(200, gin.H{
					"status":    "ok",
					"hasToken":  true,
					"expiresAt": conn.ExpiresAt,
					"email":     email,
				})
			} else {
				c.JSON(200, gin.H{
					"status":  "error",
					"message": "OpenAI OAuth token invalid: " + probeResult.error,
				})
			}
			return
		}

		c.JSON(200, gin.H{
			"status":    "ok",
			"hasToken":  conn.AccessToken != "",
			"expiresAt": conn.ExpiresAt,
			"email":     email,
		})
	}
}

// === Update Connection ===

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

		found := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			for i := range cfg.ProviderConnections {
				if cfg.ProviderConnections[i].ID == id {
					conn := &cfg.ProviderConnections[i]
					found = true
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
					break
				}
			}
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

// === Reset Cooldown ===

func apiResetCooldown(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		found := false
		if err := store.Update(func(cfg *domain.AppConfig) {
			for i := range cfg.ProviderConnections {
				if cfg.ProviderConnections[i].ID == id {
					conn := &cfg.ProviderConnections[i]
					found = true
					conn.RateLimitedUntil = ""
					conn.BackoffLevel = 0
					conn.LastError = ""
					conn.LastErrorAt = ""
					conn.ModelLocks = nil
					conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
					break
				}
			}
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

// === Generic Add Connection (uses provider config registry) ===

// apiAddConnection handles adding any API-key-based provider connection.
// Reads defaults from domain.ProviderConfigs, so adding a new provider
// only requires registering it in the config registry + main.go.
func apiAddConnection(store port.CredentialStore, providerID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := domain.GetProviderConfig(providerID)

		var req struct {
			Name            string   `json:"name"`
			APIKey          string   `json:"apiKey"`
			BaseURL         string   `json:"baseUrl,omitempty"`
			SupportedModels []string `json:"supportedModels,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.APIKey == "" {
			c.JSON(400, gin.H{"error": "apiKey is required"})
			return
		}

		appCfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// Auto-name
		name := req.Name
		if name == "" {
			name = cfg.Name + " Account"
			count := 0
			for _, conn := range appCfg.ProviderConnections {
				if conn.Provider == providerID {
					count++
				}
			}
			if count > 0 {
				name = fmt.Sprintf("%s Account %d", cfg.Name, count+1)
			}
		}

		// Default base URL from provider config
		baseURL := req.BaseURL
		if baseURL == "" {
			baseURL = cfg.DefaultBaseURL
		}

		// Default models from provider config
		supportedModels := req.SupportedModels
		if len(supportedModels) == 0 {
			supportedModels = cfg.DefaultModels
		}

		now := time.Now().UTC().Format(time.RFC3339)
		conn := domain.ProviderConnection{
			ID:              uuid.New().String(),
			Provider:        providerID,
			AuthType:        "apikey",
			Name:            name,
			Priority:        len(appCfg.ProviderConnections) + 1,
			IsActive:        true,
			APIKey:          req.APIKey,
			BaseURL:         baseURL,
			TestStatus:      "active",
			SupportedModels: supportedModels,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		appCfg.ProviderConnections = append(appCfg.ProviderConnections, conn)
		if err := store.Save(appCfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"id": conn.ID, "name": conn.Name})
	}
}

// Wrapper handlers for backward compatibility with route registration
func apiAddOpenAIConnection(store port.CredentialStore) gin.HandlerFunc {
	return apiAddConnection(store, "openai")
}
func apiAddCustomConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name    string `json:"name"`
			APIKey  string `json:"apiKey"`
			BaseURL string `json:"baseUrl"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.BaseURL == "" {
			c.JSON(400, gin.H{"error": "baseUrl is required"})
			return
		}

		appCfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		name := req.Name
		if name == "" {
			name = "Custom API"
			count := 0
			for _, conn := range appCfg.ProviderConnections {
				if conn.Provider == "openai-compatible" {
					count++
				}
			}
			if count > 0 {
				name = fmt.Sprintf("Custom API %d", count+1)
			}
		}

		now := time.Now().UTC().Format(time.RFC3339)
		conn := domain.ProviderConnection{
			ID:        uuid.New().String(),
			Provider:  "openai-compatible",
			AuthType:  "apikey",
			Name:      name,
			Priority:  len(appCfg.ProviderConnections) + 1,
			IsActive:  true,
			APIKey:    req.APIKey,
			BaseURL:   req.BaseURL,
			CreatedAt: now,
			UpdatedAt: now,
		}

		appCfg.ProviderConnections = append(appCfg.ProviderConnections, conn)
		if err := store.Save(appCfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"id": conn.ID, "name": conn.Name})
	}
}
func apiAddGLMConnection(store port.CredentialStore) gin.HandlerFunc {
	return apiAddConnection(store, "glm")
}
func apiAddMiniMaxConnection(store port.CredentialStore) gin.HandlerFunc {
	return apiAddConnection(store, "minimax")
}
func apiAddQwenConnection(store port.CredentialStore) gin.HandlerFunc {
	return apiAddConnection(store, "qwen")
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

// === Test Model ===

func apiTestModel(store port.CredentialStore, providers port.ProviderRegistry) gin.HandlerFunc {
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

		creds := shared.ConnectionToCredentials(conn)

		// Build the test body based on the connection type.
		// OpenAI OAuth → Codex Responses API: does NOT support max_tokens.
		var testBody map[string]interface{}
		if conn.Provider == "openai" && conn.AuthType == "oauth" {
			testBody = map[string]interface{}{
				"model": req.Model,
				"messages": []map[string]string{
					{"role": "user", "content": "Hi"},
				},
				"stream": false,
			}
		} else {
			testBody = map[string]interface{}{
				"model": req.Model,
				"messages": []map[string]string{
					{"role": "user", "content": "Hi"},
				},
				"stream":     false,
				"max_tokens": 5,
			}
		}
		bodyBytes, _ := json.Marshal(testBody)

		// Try provider registry first, fallback to direct instantiation
		provider := conn.Provider
		executor := providers.GetExecutor(provider)
		if executor == nil {
			// Fallback for providers not yet registered
			switch provider {
			case "kiro":
				executor = kiro.NewExecutor()
			case "openai", "openai-compatible":
				executor = openai.NewExecutor()
			default:
				c.JSON(400, gin.H{"status": "error", "message": "Unsupported provider: " + provider})
				return
			}
		}

		stream, statusCode, execErr := executor.Execute(req.Model, bodyBytes, creds, uuid.New().String())
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

// maskString masks a string, showing first n and last m characters.
func maskString(s string, first, last int) string {
	if len(s) <= first+last {
		return strings.Repeat("*", len(s))
	}
	return s[:first] + "..." + s[len(s)-last:]
}

// findConnectionByID is a helper to find a connection by ID in a config.
func findConnectionByID(cfg *domain.AppConfig, id string) *domain.ProviderConnection {
	for i := range cfg.ProviderConnections {
		if cfg.ProviderConnections[i].ID == id {
			return &cfg.ProviderConnections[i]
		}
	}
	return nil
}

// logAction logs an admin action.
func logAction(action string, args ...interface{}) {
	log.Printf("[ADMIN] "+action, args...)
}
