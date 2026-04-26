package http

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}
		if name == "" {
			name = providerLabel + " Account"
			name += fmt.Sprintf(" %d", len(cfg.ProviderConnections)+1)
		}

		now := time.Now().UTC().Format(time.RFC3339)
		expiresIn := tokens.ExpiresIn
		if expiresIn == 0 {
			expiresIn = 3600
		}

		conn := domain.ProviderConnection{
			ID:              uuid.New().String(),
			Provider:        "kiro",
			AuthType:        "oauth",
			Name:            name,
			Weight:          100,
			IsActive:        true,
			AccessToken:     tokens.AccessToken,
			RefreshToken:    tokens.RefreshToken,
			ExpiresAt:       time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
			ExpiresIn:       expiresIn,
			Email:           email,
			TestStatus:      "active",
			SupportedModels: domain.GetProviderConfig("kiro").DefaultModels,
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
