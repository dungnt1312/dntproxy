package http

import (
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

// === Generic Add Connection (uses provider config registry) ===

type addConnectionRequest struct {
	Provider        string   `json:"provider"`
	Name            string   `json:"name"`
	APIKey          string   `json:"apiKey"`
	BaseURL         string   `json:"baseUrl,omitempty"`
	RoutePrefix     string   `json:"routePrefix,omitempty"`
	ModelPrefix     string   `json:"modelPrefix,omitempty"`
	SupportedModels []string `json:"supportedModels,omitempty"`
}

func createAPIKeyConnection(c *gin.Context, store port.CredentialStore, providerID string, req addConnectionRequest) (*domain.ProviderConnection, string, int) {
	provCfg := domain.GetProviderConfig(providerID)

	if providerID == "openai-compatible" {
		if req.BaseURL == "" {
			return nil, "baseUrl is required", 400
		}
	} else if req.APIKey == "" {
		return nil, "apiKey is required", 400
	}

	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = provCfg.DefaultBaseURL
	}
	supportedModels := req.SupportedModels
	if len(supportedModels) == 0 {
		cfg, _ := store.Load()
		var settings *domain.Settings
		if cfg != nil {
			settings = &cfg.Settings
		}
		supportedModels = domain.GetDefaultConnectionModels(settings, providerID)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	conn := domain.ProviderConnection{
		ID:              uuid.New().String(),
		Provider:        providerID,
		AuthType:        "apikey",
		Weight:          100,
		IsActive:        true,
		APIKey:          req.APIKey,
		BaseURL:         baseURL,
		RoutePrefix:     domain.NormalizeRoutePrefix(req.RoutePrefix),
		ModelPrefix:     req.ModelPrefix,
		TestStatus:      "active",
		SupportedModels: supportedModels,
		CreatedAt:       now,
		UpdatedAt:       now,
		TenantID:        GetTenantID(c),
	}

	if err := store.Update(func(appCfg *domain.AppConfig) {
		name := req.Name
		if providerID == "openai-compatible" {
			if name == "" {
				name = "Custom API"
				count := 0
				for _, x := range appCfg.ProviderConnections {
					if x.Provider == "openai-compatible" {
						count++
					}
				}
				if count > 0 {
					name = fmt.Sprintf("Custom API %d", count+1)
				}
			}
			if conn.RoutePrefix == "" {
				conn.RoutePrefix = domain.NormalizeRoutePrefix(name)
			}
			allConns := append(append([]domain.ProviderConnection(nil), appCfg.ProviderConnections...), conn)
			domain.EnsureOpenAICompatibleRoutePrefixes(allConns)
			conn.RoutePrefix = allConns[len(allConns)-1].RoutePrefix
		} else {
			if name == "" {
				name = provCfg.Name + " Account"
				count := 0
				for _, x := range appCfg.ProviderConnections {
					if x.Provider == providerID {
						count++
					}
				}
				if count > 0 {
					name = fmt.Sprintf("%s Account %d", provCfg.Name, count+1)
				}
			}
		}
		conn.Name = name
		appCfg.ProviderConnections = append(appCfg.ProviderConnections, conn)
	}); err != nil {
		return nil, "Failed to save: " + err.Error(), 500
	}
	return &conn, "", 0
}

// apiCreateConnection is the unified entry: POST /api/connections with { "provider": "glm", ... }.
func apiCreateConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req addConnectionRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.Provider == "" {
			c.JSON(400, gin.H{"error": "provider is required"})
			return
		}
		if !domain.HasProvider(req.Provider) && req.Provider != "openai-compatible" {
			// allow unknown IDs as openai-compatible style only if explicitly openai-compatible
			c.JSON(400, gin.H{"error": "unknown provider: " + req.Provider})
			return
		}
		conn, errMsg, code := createAPIKeyConnection(c, store, req.Provider, req)
		if code != 0 {
			c.JSON(code, gin.H{"error": errMsg})
			return
		}
		c.JSON(200, gin.H{"id": conn.ID, "name": conn.Name, "routePrefix": conn.RoutePrefix})
	}
}

// apiAddConnectionByParam: POST /api/connections/:provider
func apiAddConnectionByParam(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerID := c.Param("provider")
		var req addConnectionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}
		req.Provider = providerID
		conn, errMsg, code := createAPIKeyConnection(c, store, providerID, req)
		if code != 0 {
			c.JSON(code, gin.H{"error": errMsg})
			return
		}
		c.JSON(200, gin.H{"id": conn.ID, "name": conn.Name, "routePrefix": conn.RoutePrefix})
	}
}

// apiAddConnection handles adding any API-key-based provider connection (legacy per-route).
func apiAddConnection(store port.CredentialStore, providerID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req addConnectionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}
		req.Provider = providerID
		conn, errMsg, code := createAPIKeyConnection(c, store, providerID, req)
		if code != 0 {
			c.JSON(code, gin.H{"error": errMsg})
			return
		}
		c.JSON(200, gin.H{"id": conn.ID, "name": conn.Name, "routePrefix": conn.RoutePrefix})
	}
}

// Wrapper handlers for backward compatibility with route registration
func apiAddOpenAIConnection(store port.CredentialStore) gin.HandlerFunc {
	return apiAddConnection(store, "openai")
}
func apiAddCustomConnection(store port.CredentialStore) gin.HandlerFunc {
	return apiAddConnection(store, "openai-compatible")
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

		cfg, err := store.Load()
		var settings *domain.Settings
		if err == nil && cfg != nil {
			settings = &cfg.Settings
		}

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

		now := time.Now().UTC().Format(time.RFC3339)
		expiresIn := result.ExpiresIn
		if expiresIn == 0 {
			expiresIn = 3600
		}

		conn := domain.ProviderConnection{
			ID:              uuid.New().String(),
			Provider:        "kiro",
			AuthType:        "oauth",
			Weight:          100,
			IsActive:        true,
			AccessToken:     result.AccessToken,
			RefreshToken:    result.RefreshToken,
			ExpiresAt:       time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
			ExpiresIn:       expiresIn,
			Email:           email,
			TestStatus:      "active",
			SupportedModels: domain.GetDefaultConnectionModels(settings, "kiro"),
			ProviderSpecificData: map[string]interface{}{
				"profileArn": result.ProfileArn,
				"authMethod": result.AuthMethod,
				"provider":   providerLabel,
			},
			CreatedAt: now,
			UpdatedAt: now,
			TenantID:  GetTenantID(c),
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

		if err := store.Update(func(cfg *domain.AppConfig) {
			name := email
			if name == "" {
				name = providerLabel + " Account"
				name += fmt.Sprintf(" %d", len(cfg.ProviderConnections)+1)
			}
			conn.Name = name
			cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
		}); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"id": conn.ID, "name": conn.Name, "email": email})
	}
}

// === Detect Kiro Token ===

func apiDetectKiroToken(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
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
