package http

import (
	"context"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// === Social Login (Google/GitHub via the Kiro auth portal) ===
//
// Kiro sends every social sign-in through https://app.kiro.dev/signin; the
// portal lets the user pick Google or GitHub and redirects the browser to
// http://localhost:<port>/oauth/callback?login_option=…&code=…&state=….
// The dashboard keeps its "open page, paste callback URL" panel: the redirect
// target is not served by dntproxy, so the pasted address-bar URL carries the
// code. The provider the user chose on the portal comes back as login_option.

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

		login, err := auth.StartPortalLogin()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to start portal login: " + auth.UserMessage(err)})
			return
		}

		sessionID := uuid.New().String()
		starterTenant, starterKey := authCallerIDs(c)
		authSessionsMu.Lock()
		if len(authSessions) >= maxAuthSessions {
			authSessionsMu.Unlock()
			c.JSON(429, gin.H{"error": "too many pending auth sessions"})
			return
		}
		authSessions[sessionID] = &authSession{
			PortalLogin:  login,
			CodeVerifier: login.CodeVerifier,
			State:        login.State,
			Provider:     req.Provider,
			AuthMethod:   req.Provider,
			TenantID:     starterTenant,
			APIKeyID:     starterKey,
			CreatedAt:    time.Now(),
		}
		authSessionsMu.Unlock()

		c.JSON(200, gin.H{
			"sessionId": sessionID,
			"loginUrl":  login.URL,
			"state":     login.State,
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

		if !ok || session.PortalLogin == nil {
			c.JSON(404, gin.H{"error": "Session not found or expired"})
			return
		}
		if !authSessionAllowed(c, session.TenantID, session.APIKeyID) {
			c.JSON(403, gin.H{"error": "Access denied"})
			return
		}

		var cb *auth.PortalCallback
		switch {
		case req.CallbackURL != "":
			parsed, err := auth.ParsePortalCallback(req.CallbackURL, session.State)
			if err != nil {
				c.JSON(400, gin.H{"error": auth.UserMessage(err)})
				return
			}
			cb = parsed
		case req.Code != "":
			// Bare code: the state cannot be checked, but the code is bound to
			// this session's PKCE verifier at the auth service.
			cb = &auth.PortalCallback{Code: req.Code}
		default:
			c.JSON(400, gin.H{"error": "No authorization code found. Paste the full callback URL or the code parameter."})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		cred, err := auth.ExchangePortalCallback(ctx, cb, session.PortalLogin, session.Provider)
		if err != nil {
			c.JSON(400, gin.H{"error": "Failed to exchange code: " + auth.UserMessage(err)})
			return
		}

		conn, email, err := saveKiroLogin(c, store, cred)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}
		dropAuthSession(req.SessionID)

		c.JSON(200, gin.H{
			"id":    conn.ID,
			"name":  conn.Name,
			"email": email,
		})
	}
}
