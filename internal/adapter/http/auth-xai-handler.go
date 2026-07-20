package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	adapterauth "github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type xaiSession struct {
	TenantID      string
	APIKeyID      string
	CodeVerifier  string
	State         string
	Nonce         string
	RedirectURI   string
	TokenEndpoint string
	CreatedAt     time.Time
}

func authXAIStart() gin.HandlerFunc {
	return func(c *gin.Context) {
		codeVerifier, codeChallenge, state, err := adapterauth.GeneratePKCE()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PKCE: " + err.Error()})
			return
		}
		nonce, err := adapterauth.GenerateState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate nonce: " + err.Error()})
			return
		}

		discovery, err := adapterauth.DiscoverXAI()
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		authURL, err := adapterauth.BuildXAIAuthURL(adapterauth.XAIAuthorizeURLParams{
			AuthorizationEndpoint: discovery.AuthorizationEndpoint,
			RedirectURI:           adapterauth.XAIRedirectURI,
			CodeChallenge:         codeChallenge,
			State:                 state,
			Nonce:                 nonce,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		sessionID := uuid.New().String()
		starterTenant, starterKey := authCallerIDs(c)
		xaiSessionsMu.Lock()
		if len(xaiSessions) >= maxAuthSessions {
			xaiSessionsMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many pending auth sessions"})
			return
		}
		xaiSessions[sessionID] = &xaiSession{
			TenantID:      starterTenant,
			APIKeyID:      starterKey,
			CodeVerifier:  codeVerifier,
			State:         state,
			Nonce:         nonce,
			RedirectURI:   adapterauth.XAIRedirectURI,
			TokenEndpoint: discovery.TokenEndpoint,
			CreatedAt:     time.Now(),
		}
		xaiSessionsMu.Unlock()

		c.JSON(http.StatusOK, gin.H{
			"sessionId":   sessionID,
			"authUrl":     authURL,
			"state":       state,
			"redirectUri": adapterauth.XAIRedirectURI,
		})
	}
}

func authXAIExchange(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SessionID   string `json:"sessionId"`
			Code        string `json:"code"`
			State       string `json:"state"`
			CallbackURL string `json:"callbackUrl"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
			return
		}

		xaiSessionsMu.Lock()
		sess, ok := xaiSessions[req.SessionID]
		if ok {
			delete(xaiSessions, req.SessionID)
		}
		xaiSessionsMu.Unlock()
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found or expired"})
			return
		}

		if !authSessionAllowed(c, sess.TenantID, sess.APIKeyID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		code, state, err := parseXAICallback(req.Code, req.State, req.CallbackURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "authorization code is required"})
			return
		}
		if state != "" && state != sess.State {
			c.JSON(http.StatusBadRequest, gin.H{"error": "state mismatch"})
			return
		}

		tokens, err := adapterauth.ExchangeXAICode(code, sess.RedirectURI, sess.CodeVerifier, sess.TokenEndpoint)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		expiresIn := tokens.ExpiresIn
		if expiresIn == 0 {
			expiresIn = 3600
		}
		now := time.Now().UTC()
		name := tokens.Email
		if name == "" {
			name = tokens.Subject
		}
		if name == "" {
			name = "Grok Account"
		}

		cfg, _ := store.Load()
		var settings *domain.Settings
		if cfg != nil {
			settings = &cfg.Settings
		}

		conn := domain.ProviderConnection{
			ID:              uuid.New().String(),
			Provider:        "xai",
			AuthType:        "oauth",
			Name:            name,
			Weight:          100,
			IsActive:        true,
			AccessToken:     tokens.AccessToken,
			RefreshToken:    tokens.RefreshToken,
			ExpiresAt:       now.Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339),
			ExpiresIn:       expiresIn,
			Email:           tokens.Email,
			BaseURL:         adapterauth.XAIDefaultAPIBaseURL,
			TestStatus:      "active",
			SupportedModels: domain.GetDefaultConnectionModels(settings, "xai"),
			ProviderSpecificData: map[string]interface{}{
				"authMethod":    "xai-oauth",
				"tokenEndpoint": sess.TokenEndpoint,
				"redirectURI":   sess.RedirectURI,
				"idToken":       tokens.IDToken,
				"subject":       tokens.Subject,
			},
			CreatedAt: now.Format(time.RFC3339),
			UpdatedAt: now.Format(time.RFC3339),
			TenantID:  GetTenantID(c),
		}

		if err := store.Update(func(appCfg *domain.AppConfig) {
			if conn.Name == "Grok Account" {
				count := 0
				for _, existing := range appCfg.ProviderConnections {
					if existing.Provider == "xai" {
						count++
					}
				}
				if count > 0 {
					conn.Name = fmt.Sprintf("Grok Account %d", count+1)
				}
			}
			appCfg.ProviderConnections = append(appCfg.ProviderConnections, conn)
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"connection": conn})
	}
}

func parseXAICallback(code, state, callbackURL string) (string, string, error) {
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	callbackURL = strings.TrimSpace(callbackURL)
	if callbackURL == "" {
		return code, state, nil
	}

	parsed, err := url.Parse(callbackURL)
	if err != nil {
		return "", "", fmt.Errorf("Invalid callback URL: %w", err)
	}
	if parsed.RawQuery == "" && parsed.Scheme == "" && parsed.Host == "" {
		return callbackURL, state, nil
	}
	if parsedCode := parsed.Query().Get("code"); parsedCode != "" {
		code = strings.TrimSpace(parsedCode)
	}
	if parsedState := parsed.Query().Get("state"); parsedState != "" {
		state = strings.TrimSpace(parsedState)
	}
	return code, state, nil
}

func cleanupXAISessions() {
	xaiSessionsMu.Lock()
	defer xaiSessionsMu.Unlock()
	now := time.Now()
	for id, s := range xaiSessions {
		if now.Sub(s.CreatedAt) > 15*time.Minute {
			delete(xaiSessions, id)
		}
	}
}

// xaiAuthFileJSON represents the raw xAI/Grok auth file from a third-party CLI.
type xaiAuthFileJSON struct {
	AccessToken  string `json:"access_token"`
	AuthKind     string `json:"auth_kind"`
	BaseURL      string `json:"base_url"`
	Disabled     bool   `json:"disabled"`
	Email        string `json:"email"`
	Expired      string `json:"expired"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	LastRefresh  string `json:"last_refresh"`
	RedirectURI  string `json:"redirect_uri"`
	RefreshToken string `json:"refresh_token"`
	Sub          string `json:"sub"`
	TokenEp      string `json:"token_endpoint"`
	TokenType    string `json:"token_type"`
	Type         string `json:"type"`
}

func authXAIImportFile(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		// Support both {"data": "{...}"} wrapper and raw auth file
		var wrapper struct {
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(raw, &wrapper) == nil && len(wrapper.Data) > 0 {
			raw = wrapper.Data
		}

		var f xaiAuthFileJSON
		if err := json.Unmarshal(raw, &f); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid xAI auth file format: " + err.Error()})
			return
		}

		if f.AccessToken == "" && f.RefreshToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Auth file must contain access_token or refresh_token"})
			return
		}

		conn, dupe, err := createXAIConnectionFromAuthFile(store, c, f)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":        conn.ID,
			"name":      conn.Name,
			"email":     conn.Email,
			"duplicate": dupe,
		})
	}
}

func createXAIConnectionFromAuthFile(store port.CredentialStore, c *gin.Context, f xaiAuthFileJSON) (*domain.ProviderConnection, bool, error) {
	baseURL := strings.TrimSpace(f.BaseURL)
	if baseURL == "" {
		baseURL = adapterauth.XAIDefaultAPIBaseURL
	}

	expiresIn := f.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}
	now := time.Now().UTC()

	expiresAt := f.Expired
	if norm, ok := adapterauth.NormalizeExpiresAtRFC3339(f.Expired); ok {
		expiresAt = norm
	} else if expiresAt == "" {
		expiresAt = now.Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339)
	}

	tokenEndpoint := strings.TrimSpace(f.TokenEp)
	if tokenEndpoint == "" {
		tokenEndpoint = adapterauth.XAIIssuer + "/oauth2/token"
	}

	redirectURI := strings.TrimSpace(f.RedirectURI)
	if redirectURI == "" {
		redirectURI = adapterauth.XAIRedirectURI
	}

	// Check for duplicate by email or subject
	existingDup := false
	cfg, _ := store.Load()
	if cfg != nil {
		for _, x := range cfg.ProviderConnections {
			if x.Provider != "xai" {
				continue
			}
			if f.Email != "" && strings.EqualFold(x.Email, f.Email) {
				existingDup = true
				break
			}
			if x.ProviderSpecificData != nil {
				if sub, ok := x.ProviderSpecificData["subject"].(string); ok && f.Sub != "" && sub == f.Sub {
					existingDup = true
					break
				}
			}
		}
	}

	name := strings.TrimSpace(f.Email)
	if name == "" {
		name = f.Sub
	}
	if name == "" {
		name = "Grok Account"
	}

	cfg, _ = store.Load()
	var settings *domain.Settings
	if cfg != nil {
		settings = &cfg.Settings
	}

	conn := domain.ProviderConnection{
		ID:              uuid.New().String(),
		Provider:        "xai",
		AuthType:        "oauth",
		Name:            name,
		Weight:          100,
		IsActive:        !f.Disabled,
		AccessToken:     strings.TrimSpace(f.AccessToken),
		RefreshToken:    strings.TrimSpace(f.RefreshToken),
		ExpiresAt:       expiresAt,
		ExpiresIn:       expiresIn,
		Email:           strings.TrimSpace(f.Email),
		BaseURL:         baseURL,
		TestStatus:      "active",
		SupportedModels: domain.GetDefaultConnectionModels(settings, "xai"),
		ProviderSpecificData: map[string]interface{}{
			"authMethod":    "xai-oauth",
			"tokenEndpoint": tokenEndpoint,
			"redirectURI":   redirectURI,
			"idToken":       strings.TrimSpace(f.IDToken),
			"subject":       f.Sub,
		},
		CreatedAt: now.Format(time.RFC3339),
		UpdatedAt: now.Format(time.RFC3339),
		TenantID:  GetTenantID(c),
	}

	if err := store.Update(func(appCfg *domain.AppConfig) {
		if conn.Name == "Grok Account" {
			count := 0
			for _, x := range appCfg.ProviderConnections {
				if x.Provider == "xai" {
					count++
				}
			}
			if count > 0 {
				conn.Name = fmt.Sprintf("Grok Account %d", count+1)
			}
		}
		appCfg.ProviderConnections = append(appCfg.ProviderConnections, conn)
	}); err != nil {
		return nil, false, fmt.Errorf("Failed to save: %w", err)
	}

	if strings.TrimSpace(conn.RefreshToken) != "" {
		refreshSvc := adapterauth.NewTokenRefreshService(store)
		if refreshSvc.ShouldProactivelyRefresh(&conn) {
			if updated, refErr := refreshSvc.ForceRefresh(&conn); refErr == nil && updated != nil {
				conn = *updated
			}
		}
	}

	return &conn, existingDup, nil
}
