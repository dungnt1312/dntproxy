package http

import (
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
		xaiSessionsMu.Lock()
		if len(xaiSessions) >= maxAuthSessions {
			xaiSessionsMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many auth sessions"})
			return
		}
		xaiSessions[sessionID] = &xaiSession{
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

		conn := domain.ProviderConnection{
			ID:           uuid.New().String(),
			Provider:     "xai",
			AuthType:     "oauth",
			Name:         name,
			Weight:       100,
			IsActive:     true,
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			ExpiresAt:    now.Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339),
			ExpiresIn:    expiresIn,
			Email:        tokens.Email,
			BaseURL:      adapterauth.XAIDefaultAPIBaseURL,
			TestStatus:   "active",
			SupportedModels: []string{
				"grok-4.3",
				"grok-3-mini",
			},
			ProviderSpecificData: map[string]interface{}{
				"authMethod":    "xai-oauth",
				"tokenEndpoint": sess.TokenEndpoint,
				"redirectURI":   sess.RedirectURI,
				"idToken":       tokens.IDToken,
				"subject":       tokens.Subject,
			},
			CreatedAt: now.Format(time.RFC3339),
			UpdatedAt: now.Format(time.RFC3339),
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
