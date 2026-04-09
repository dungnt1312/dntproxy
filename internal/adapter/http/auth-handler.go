package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// In-memory auth session storage (cleaned up after use or expiry)
var (
	authSessions   = make(map[string]*authSession)
	authSessionsMu sync.Mutex
)

type authSession struct {
	// Builder ID / IDC device code flow
	ClientID     string
	ClientSecret string
	DeviceCode   string
	Region       string
	StartURL     string
	Interval     int
	AuthMethod   string // "builder-id" or "idc"

	// Social login (PKCE)
	CodeVerifier string
	State        string
	Provider     string // "google" or "github"

	CreatedAt time.Time
}

func init() {
	// Background cleanup of expired sessions (>15 min)
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			authSessionsMu.Lock()
			for id, s := range authSessions {
				if time.Since(s.CreatedAt) > 15*time.Minute {
					delete(authSessions, id)
				}
			}
			authSessionsMu.Unlock()
		}
	}()
}

// RegisterAuthRoutes adds auth flow endpoints.
func RegisterAuthRoutes(r *gin.Engine, store port.CredentialStore) {
	authGroup := r.Group("/api/auth")
	{
		authGroup.POST("/kiro/start-builderid", authStartBuilderID())
		authGroup.POST("/kiro/start-idc", authStartIDC())
		authGroup.POST("/kiro/poll", authPoll(store))
		authGroup.POST("/kiro/start-social", authStartSocial())
		authGroup.POST("/kiro/exchange-social", authExchangeSocial(store))

		// OpenAI OAuth (PKCE, Authorization Code)
		authGroup.POST("/openai/start", authOpenAIStart())
		authGroup.POST("/openai/exchange", authOpenAIExchange(store))
	}

	// Fetch models from provider API
	r.POST("/api/connections/:id/fetch-models", apiFetchConnectionModels(store))
}

// === Builder ID Device Code Flow ===

func authStartBuilderID() gin.HandlerFunc {
	return func(c *gin.Context) {
		client, deviceAuth, err := auth.StartBuilderIDDeviceAuth()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to start Builder ID auth: " + err.Error()})
			return
		}

		sessionID := uuid.New().String()
		authSessionsMu.Lock()
		authSessions[sessionID] = &authSession{
			ClientID:     client.ClientID,
			ClientSecret: client.ClientSecret,
			DeviceCode:   deviceAuth.DeviceCode,
			Region:       "us-east-1",
			Interval:     deviceAuth.Interval,
			AuthMethod:   "builder-id",
			CreatedAt:    time.Now(),
		}
		authSessionsMu.Unlock()

		c.JSON(200, gin.H{
			"sessionId":               sessionID,
			"userCode":                deviceAuth.UserCode,
			"verificationUri":         deviceAuth.VerificationURI,
			"verificationUriComplete": deviceAuth.VerificationURIComplete,
			"expiresIn":               deviceAuth.ExpiresIn,
			"interval":                deviceAuth.Interval,
		})
	}
}

// === IDC Device Code Flow ===

func authStartIDC() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			StartURL string `json:"startUrl"`
			Region   string `json:"region"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.StartURL == "" {
			c.JSON(400, gin.H{"error": "startUrl is required"})
			return
		}
		if req.Region == "" {
			req.Region = "us-east-1"
		}

		client, deviceAuth, err := auth.StartIDCDeviceAuth(req.StartURL, req.Region)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to start IDC auth: " + err.Error()})
			return
		}

		sessionID := uuid.New().String()
		authSessionsMu.Lock()
		authSessions[sessionID] = &authSession{
			ClientID:     client.ClientID,
			ClientSecret: client.ClientSecret,
			DeviceCode:   deviceAuth.DeviceCode,
			Region:       req.Region,
			StartURL:     req.StartURL,
			Interval:     deviceAuth.Interval,
			AuthMethod:   "idc",
			CreatedAt:    time.Now(),
		}
		authSessionsMu.Unlock()

		c.JSON(200, gin.H{
			"sessionId":               sessionID,
			"userCode":                deviceAuth.UserCode,
			"verificationUri":         deviceAuth.VerificationURI,
			"verificationUriComplete": deviceAuth.VerificationURIComplete,
			"expiresIn":               deviceAuth.ExpiresIn,
			"interval":                deviceAuth.Interval,
		})
	}
}

// === Device Code Polling (shared by Builder ID and IDC) ===

func authPoll(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SessionID string `json:"sessionId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
			c.JSON(400, gin.H{"error": "sessionId is required"})
			return
		}

		authSessionsMu.Lock()
		session, ok := authSessions[req.SessionID]
		authSessionsMu.Unlock()

		if !ok {
			c.JSON(404, gin.H{"error": "Session not found or expired"})
			return
		}

		result, err := auth.PollDeviceToken(session.ClientID, session.ClientSecret, session.DeviceCode, session.Region)
		if err != nil {
			c.JSON(500, gin.H{"error": "Poll failed: " + err.Error()})
			return
		}

		if result.Pending {
			c.JSON(200, gin.H{"status": "pending"})
			return
		}

		if !result.Success {
			authSessionsMu.Lock()
			delete(authSessions, req.SessionID)
			authSessionsMu.Unlock()
			c.JSON(200, gin.H{"status": "error", "error": result.Error, "errorDescription": result.ErrorDescription})
			return
		}

		// Success — create connection
		tokens := result.Tokens
		email := auth.ExtractEmailFromJWT(tokens.AccessToken)

		providerLabel := "AWS Builder ID"
		if session.AuthMethod == "idc" {
			providerLabel = "AWS IAM Identity Center"
		}

		name := email
		cfg, _ := store.Load()
		if name == "" {
			name = providerLabel + " Account"
			if cfg != nil {
				name += fmt.Sprintf(" %d", len(cfg.ProviderConnections)+1)
			}
		}

		now := time.Now().UTC().Format(time.RFC3339)
		expiresIn := tokens.ExpiresIn
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
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
			ExpiresIn:    expiresIn,
			Email:        email,
			TestStatus:   "active",
			ProviderSpecificData: map[string]interface{}{
				"authMethod":   session.AuthMethod,
				"provider":     providerLabel,
				"clientId":     session.ClientID,
				"clientSecret": session.ClientSecret,
				"region":       session.Region,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		authSessionsMu.Lock()
		delete(authSessions, req.SessionID)
		authSessionsMu.Unlock()

		c.JSON(200, gin.H{
			"status": "success",
			"id":     conn.ID,
			"name":   conn.Name,
			"email":  email,
		})
	}
}

// === Social Login (Google/GitHub PKCE) ===

func authStartSocial() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Provider string `json:"provider"` // "google" or "github"
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "provider is required"})
			return
		}
		if req.Provider != "google" && req.Provider != "github" {
			c.JSON(400, gin.H{"error": "provider must be 'google' or 'github'"})
			return
		}

		codeVerifier, codeChallenge, state, err := auth.GeneratePKCE()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to generate PKCE: " + err.Error()})
			return
		}

		loginURL, err := auth.BuildSocialLoginURL(req.Provider, codeChallenge, state)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to build login URL: " + err.Error()})
			return
		}

		sessionID := uuid.New().String()
		authSessionsMu.Lock()
		authSessions[sessionID] = &authSession{
			CodeVerifier: codeVerifier,
			State:        state,
			Provider:     req.Provider,
			AuthMethod:   req.Provider,
			CreatedAt:    time.Now(),
		}
		authSessionsMu.Unlock()

		c.JSON(200, gin.H{
			"sessionId": sessionID,
			"loginUrl":  loginURL,
			"state":     state,
		})
	}
}

func authExchangeSocial(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SessionID   string `json:"sessionId"`
			CallbackURL string `json:"callbackUrl"`
			Code        string `json:"code"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
			c.JSON(400, gin.H{"error": "sessionId is required"})
			return
		}

		authSessionsMu.Lock()
		session, ok := authSessions[req.SessionID]
		authSessionsMu.Unlock()

		if !ok {
			c.JSON(404, gin.H{"error": "Session not found or expired"})
			return
		}

		// Extract code from callback URL if not provided directly
		code := req.Code
		if code == "" && req.CallbackURL != "" {
			parsed, err := url.Parse(strings.Replace(req.CallbackURL, "kiro://", "https://", 1))
			if err == nil {
				code = parsed.Query().Get("code")
			}
		}
		if code == "" {
			c.JSON(400, gin.H{"error": "No authorization code found. Paste the full callback URL or the code parameter."})
			return
		}

		// Exchange code for tokens
		tokens, err := auth.ExchangeSocialCode(code, session.CodeVerifier)
		if err != nil {
			c.JSON(400, gin.H{"error": "Failed to exchange code: " + err.Error()})
			return
		}

		email := auth.ExtractEmailFromJWT(tokens.AccessToken)
		providerLabel := "Google"
		if session.Provider == "github" {
			providerLabel = "GitHub"
		}

		name := email
		cfg, _ := store.Load()
		if name == "" {
			name = providerLabel + " Account"
			if cfg != nil {
				name += fmt.Sprintf(" %d", len(cfg.ProviderConnections)+1)
			}
		}

		now := time.Now().UTC().Format(time.RFC3339)
		expiresIn := tokens.ExpiresIn
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
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
			ExpiresIn:    expiresIn,
			Email:        email,
			TestStatus:   "active",
			ProviderSpecificData: map[string]interface{}{
				"profileArn": tokens.ProfileArn,
				"authMethod": session.Provider,
				"provider":   providerLabel,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		authSessionsMu.Lock()
		delete(authSessions, req.SessionID)
		authSessionsMu.Unlock()

		c.JSON(200, gin.H{
			"id":    conn.ID,
			"name":  conn.Name,
			"email": email,
		})
	}
}

// === Fetch Models from Provider API ===

func apiFetchConnectionModels(store port.CredentialStore) gin.HandlerFunc {
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

		if conn.Provider != "openai" && conn.Provider != "openai-compatible" {
			c.JSON(400, gin.H{"error": "Model fetching is only supported for OpenAI and OpenAI-compatible providers"})
			return
		}

		baseURL := conn.BaseURL
		if baseURL == "" && conn.Provider == "openai" {
			if conn.AuthType == "oauth" {
				baseURL = "https://chatgpt.com/backend-api"
			} else {
				baseURL = "https://api.openai.com"
			}
		}
		if baseURL == "" {
			c.JSON(400, gin.H{"error": "No base URL configured for this connection"})
			return
		}

		var modelsURL string
		// chatgpt.com/backend-api/models instead of /v1/models
		if conn.Provider == "openai" && conn.AuthType == "oauth" {
			modelsURL = baseURL + "/models"
		} else {
			modelsURL = baseURL + "/v1/models"
		}

		req, _ := http.NewRequest("GET", modelsURL, nil)
		if conn.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+conn.APIKey)
		} else if conn.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
		}

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(502, gin.H{"error": "Failed to reach provider: " + err.Error()})
			return
		}

		if (resp.StatusCode == 401 || resp.StatusCode == 403) && conn.Provider == "openai" && conn.RefreshToken != "" {
			resp.Body.Close()
			updatedConn, refErr := auth.RefreshOpenAIToken(conn, store)
			if refErr == nil {
				conn = updatedConn
				req, _ = http.NewRequest("GET", modelsURL, nil)
				req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
				resp, err = client.Do(req)
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

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Provider returned %d: %s", resp.StatusCode, string(body))})
			return
		}

		var modelsResp map[string]interface{}
		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &modelsResp); err != nil {
			c.JSON(500, gin.H{"error": "Failed to parse models response"})
			return
		}

		var modelIDs []string
		if data, ok := modelsResp["data"].([]interface{}); ok {
			// standard OpenAI API format
			for _, mInter := range data {
				if m, ok := mInter.(map[string]interface{}); ok {
					if id, ok := m["id"].(string); ok {
						modelIDs = append(modelIDs, id)
					}
				}
			}
		} else if models, ok := modelsResp["models"].([]interface{}); ok {
			// ChatGPT backend API format
			for _, mInter := range models {
				if m, ok := mInter.(map[string]interface{}); ok {
					if slug, ok := m["slug"].(string); ok {
						modelIDs = append(modelIDs, slug)
					}
				}
			}
		}
		sort.Strings(modelIDs)

		c.JSON(200, gin.H{"models": modelIDs})
	}
}

// === OpenAI OAuth (PKCE, Authorization Code Flow) ===
// Mirrors the Codex CLI / 9router CODEX_CONFIG implementation.
// client_id: app_EMoamEEZ73f0CkXaXp7hrann
// auth URL:  https://auth.openai.com/oauth/authorize
// token URL: https://auth.openai.com/oauth/token

const (
	openaiClientID   = "app_EMoamEEZ73f0CkXaXp7hrann"
	openaiAuthURL    = "https://auth.openai.com/oauth/authorize"
	openaiTokenURL   = "https://auth.openai.com/oauth/token"
	openaiScope      = "openid profile email offline_access"
	openaiCallbackPort = 1455
)

// openaiSession extends authSession with OpenAI-specific captured code.
type openaiSession struct {
	CodeVerifier string
	State        string
	Code         string // filled when callback received
	Done         bool
	Error        string
	CreatedAt    time.Time
}

var (
	openaiSessions   = make(map[string]*openaiSession)
	openaiSessionsMu sync.Mutex
)

func authOpenAIStart() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate PKCE
		codeVerifier, codeChallenge, state, err := auth.GeneratePKCE()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to generate PKCE: " + err.Error()})
			return
		}

		redirectURI := fmt.Sprintf("http://localhost:%d/auth/callback", openaiCallbackPort)

		// Build OpenAI auth URL (space encoded as %20 like real Codex CLI)
		params := map[string]string{
			"response_type":              "code",
			"client_id":                  openaiClientID,
			"redirect_uri":               redirectURI,
			"scope":                      openaiScope,
			"code_challenge":             codeChallenge,
			"code_challenge_method":      "S256",
			"state":                      state,
			"id_token_add_organizations": "true",
			"originator":                 "openai_native",
		}

		queryParts := []string{}
		for k, v := range params {
			queryParts = append(queryParts, k+"="+url.QueryEscape(v))
		}
		authURL := openaiAuthURL + "?" + strings.Join(queryParts, "&")

		sessionID := uuid.New().String()
		sess := &openaiSession{
			CodeVerifier: codeVerifier,
			State:        state,
			CreatedAt:    time.Now(),
		}
		openaiSessionsMu.Lock()
		openaiSessions[sessionID] = sess
		openaiSessionsMu.Unlock()

		// Start one-shot local callback server
		go func() {
			mux := http.NewServeMux()
			server := &http.Server{Addr: fmt.Sprintf(":%d", openaiCallbackPort), Handler: mux}
			mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				openaiSessionsMu.Lock()
				if q.Get("error") != "" {
					sess.Error = q.Get("error_description")
					if sess.Error == "" {
						sess.Error = q.Get("error")
					}
				} else {
					sess.Code = q.Get("code")
				}
				sess.Done = true
				openaiSessionsMu.Unlock()

				w.Header().Set("Content-Type", "text/html")
				if sess.Error != "" {
					fmt.Fprintf(w, `<html><body><h2>❌ Authorization failed</h2><p>%s</p><p>You can close this tab.</p></body></html>`, sess.Error)
				} else {
					fmt.Fprintf(w, `<html><body><h2>✅ Connected to OpenAI!</h2><p>You can close this tab and return to dntproxy.</p></body></html>`)
				}
				go func() { time.Sleep(2 * time.Second); server.Close() }()
			})
			// Timeout after 5 min
			go func() {
				time.Sleep(5 * time.Minute)
				openaiSessionsMu.Lock()
				if !sess.Done {
					sess.Done = true
					sess.Error = "Authentication timed out"
				}
				openaiSessionsMu.Unlock()
				server.Close()
			}()
			server.ListenAndServe()
		}()

		c.JSON(200, gin.H{
			"sessionId":   sessionID,
			"authUrl":     authURL,
			"redirectUri": redirectURI,
		})
	}
}

func authOpenAIExchange(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SessionID string `json:"sessionId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
			c.JSON(400, gin.H{"error": "sessionId is required"})
			return
		}

		openaiSessionsMu.Lock()
		sess, ok := openaiSessions[req.SessionID]
		openaiSessionsMu.Unlock()
		if !ok {
			c.JSON(404, gin.H{"error": "Session not found or expired"})
			return
		}

		// Check if callback was received
		openaiSessionsMu.Lock()
		done := sess.Done
		code := sess.Code
		errMsg := sess.Error
		openaiSessionsMu.Unlock()

		if !done {
			c.JSON(200, gin.H{"status": "pending"})
			return
		}
		if errMsg != "" {
			c.JSON(200, gin.H{"status": "error", "error": errMsg})
			return
		}
		if code == "" {
			c.JSON(200, gin.H{"status": "error", "error": "No authorization code received"})
			return
		}

		redirectURI := fmt.Sprintf("http://localhost:%d/auth/callback", openaiCallbackPort)

		// Exchange code for tokens
		formData := url.Values{
			"grant_type":    {"authorization_code"},
			"client_id":     {openaiClientID},
			"code":          {code},
			"redirect_uri":  {redirectURI},
			"code_verifier": {sess.CodeVerifier},
		}

		resp, err := http.Post(openaiTokenURL, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
		if err != nil {
			c.JSON(502, gin.H{"error": "Token exchange request failed: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			c.JSON(400, gin.H{"error": fmt.Sprintf("Token exchange failed (%d): %s", resp.StatusCode, string(body))})
			return
		}

		var tokens struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			IDToken      string `json:"id_token"`
			ExpiresIn    int    `json:"expires_in"`
			TokenType    string `json:"token_type"`
		}
		if err := json.Unmarshal(body, &tokens); err != nil {
			c.JSON(500, gin.H{"error": "Failed to parse token response: " + err.Error()})
			return
		}

		email := auth.ExtractEmailFromJWT(tokens.IDToken)
		if email == "" {
			email = auth.ExtractEmailFromJWT(tokens.AccessToken)
		}

		cfg, _ := store.Load()
		name := email
		if name == "" {
			name = fmt.Sprintf("OpenAI Account %d", len(cfg.ProviderConnections)+1)
		}

		expiresIn := tokens.ExpiresIn
		if expiresIn == 0 {
			expiresIn = 3600
		}
		now := time.Now().UTC().Format(time.RFC3339)

		conn := domain.ProviderConnection{
			ID:           uuid.New().String(),
			Provider:     "openai",
			AuthType:     "oauth",
			Name:         name,
			Priority:     len(cfg.ProviderConnections) + 1,
			IsActive:     true,
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
			ExpiresIn:    expiresIn,
			Email:        email,
			TestStatus:   "active",
			ProviderSpecificData: map[string]interface{}{
				"authMethod": "oauth",
				"idToken":    tokens.IDToken,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
		if err := store.Save(cfg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		openaiSessionsMu.Lock()
		delete(openaiSessions, req.SessionID)
		openaiSessionsMu.Unlock()

		c.JSON(200, gin.H{
			"status": "success",
			"id":     conn.ID,
			"name":   conn.Name,
			"email":  email,
		})
	}
}

