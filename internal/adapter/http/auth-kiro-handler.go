package http

import (
	"context"
	"net/http"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// === Builder ID / IDC sign-in ===
//
// The dashboard drives both through the same three calls — start, then poll —
// and renders a "code + link + waiting" panel. Two flows sit behind them:
//
//   - auth-code (default when the browser reaches dntproxy's loopback): Kiro's
//     own flow. RegisterClient → /authorize with PKCE → 127.0.0.1 callback →
//     CreateToken(authorization_code). The "code" shown is a short session tag
//     and verificationUri(Complete) is the authorize URL.
//   - device-code (fallback for remote/proxied/containerised dntproxy): RFC 8628,
//     where the user types the code on AWS's page.
//
// The response shape is identical, so the dashboard needs no change.

// startLoginInterval is the poll interval reported for auth-code logins.
const startLoginInterval = 3

func authStartBuilderID() gin.HandlerFunc {
	return func(c *gin.Context) {
		startIdCOrDevice(c, auth.IdCLoginOptions{
			Provider: auth.ProviderBuilderID,
			StartURL: auth.BuilderIDStartURL,
			Region:   auth.DefaultIdCRegion,
		}, "builder-id")
	}
}

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
			req.Region = auth.DefaultIdCRegion
		}
		startIdCOrDevice(c, auth.IdCLoginOptions{
			Provider: auth.ProviderEnterprise,
			StartURL: req.StartURL,
			Region:   req.Region,
		}, "idc")
	}
}

func startIdCOrDevice(c *gin.Context, opts auth.IdCLoginOptions, authMethod string) {
	starterTenant, starterKey := authCallerIDs(c)

	if loginFlowMode(c) == "auth-code" {
		login, err := auth.StartIdCLogin(c.Request.Context(), opts)
		if err != nil {
			c.JSON(statusForAuthErr(err), gin.H{"error": "Failed to start " + authMethod + " auth: " + auth.UserMessage(err)})
			return
		}
		sessionID := uuid.New().String()
		authSessionsMu.Lock()
		if len(authSessions) >= maxAuthSessions {
			authSessionsMu.Unlock()
			login.Cancel()
			c.JSON(429, gin.H{"error": "too many pending auth sessions"})
			return
		}
		authSessions[sessionID] = &authSession{
			Login:      login,
			Region:     opts.Region,
			StartURL:   opts.StartURL,
			AuthMethod: authMethod,
			TenantID:   starterTenant,
			APIKeyID:   starterKey,
			CreatedAt:  time.Now(),
		}
		authSessionsMu.Unlock()

		c.JSON(200, gin.H{
			"sessionId":               sessionID,
			"userCode":                login.UserCode,
			"verificationUri":         login.AuthorizeURL,
			"verificationUriComplete": login.AuthorizeURL,
			"expiresIn":               login.ExpiresIn,
			"interval":                startLoginInterval,
		})
		return
	}

	client, deviceAuth, err := auth.StartIDCDeviceAuth(opts.StartURL, opts.Region)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to start " + authMethod + " auth: " + err.Error()})
		return
	}
	sessionID := uuid.New().String()
	authSessionsMu.Lock()
	if len(authSessions) >= maxAuthSessions {
		authSessionsMu.Unlock()
		c.JSON(429, gin.H{"error": "too many pending auth sessions"})
		return
	}
	authSessions[sessionID] = &authSession{
		ClientID:              client.ClientID,
		ClientSecret:          client.ClientSecret,
		ClientSecretExpiresAt: unixToRFC3339(client.ClientSecretExpiresAt),
		DeviceCode:            deviceAuth.DeviceCode,
		Region:                opts.Region,
		StartURL:              opts.StartURL,
		Interval:              deviceAuth.Interval,
		AuthMethod:            authMethod,
		TenantID:              starterTenant,
		APIKeyID:              starterKey,
		CreatedAt:             time.Now(),
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

func unixToRFC3339(sec int64) string { return auth.UnixToRFC3339(sec) }

// statusForAuthErr maps a classified auth error to an HTTP status.
func statusForAuthErr(err error) int {
	switch auth.ErrorKind(err) {
	case auth.KindInvalidUserInput:
		return http.StatusBadRequest
	case auth.KindNetworkIssue, auth.KindServerIssue:
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}

// === Polling (shared by Builder ID and IDC) ===

func authPoll(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SessionID string `json:"sessionId"`
			// CallbackURL is optional: when the browser could not reach the
			// loopback listener, the operator pastes the redirect URL here.
			CallbackURL string `json:"callbackUrl"`
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
		if !authSessionAllowed(c, session.TenantID, session.APIKeyID) {
			c.JSON(403, gin.H{"error": "Access denied"})
			return
		}

		if session.Login != nil {
			pollAuthCode(c, store, req.SessionID, session, req.CallbackURL)
			return
		}
		pollDeviceCode(c, store, req.SessionID, session)
	}
}

func dropAuthSession(id string) {
	authSessionsMu.Lock()
	delete(authSessions, id)
	authSessionsMu.Unlock()
}

func authFailed(c *gin.Context, id string, err error) {
	dropAuthSession(id)
	c.JSON(200, gin.H{
		"status":           "error",
		"error":            "authorization_failed",
		"errorDescription": auth.UserMessage(err),
	})
}

func authSucceeded(c *gin.Context, store port.CredentialStore, id string, cred *auth.KiroCredential) {
	conn, email, err := saveKiroLogin(c, store, cred)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
		return
	}
	dropAuthSession(id)
	c.JSON(200, gin.H{
		"status": "success",
		"id":     conn.ID,
		"name":   conn.Name,
		"email":  email,
	})
}

func pollAuthCode(c *gin.Context, store port.CredentialStore, id string, session *authSession, callbackURL string) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	var (
		cred *auth.KiroCredential
		done bool
		err  error
	)
	if callbackURL != "" {
		cred, err = session.Login.SubmitCallback(ctx, callbackURL)
		done = true
	} else {
		cred, done, err = session.Login.Poll(ctx)
	}
	switch {
	case !done:
		c.JSON(200, gin.H{"status": "pending"})
	case err != nil:
		authFailed(c, id, err)
	default:
		authSucceeded(c, store, id, cred)
	}
}

func pollDeviceCode(c *gin.Context, store port.CredentialStore, id string, session *authSession) {
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
		dropAuthSession(id)
		c.JSON(200, gin.H{"status": "error", "error": result.Error, "errorDescription": result.ErrorDescription})
		return
	}

	tokens := result.Tokens
	cred := &auth.KiroCredential{
		Provider:              auth.ProviderBuilderID,
		AccessToken:           tokens.AccessToken,
		RefreshToken:          tokens.RefreshToken,
		ExpiresIn:             tokens.ExpiresIn,
		Region:                session.Region,
		StartURL:              session.StartURL,
		ClientID:              session.ClientID,
		ClientSecret:          session.ClientSecret,
		ClientSecretExpiresAt: session.ClientSecretExpiresAt,
	}
	if session.AuthMethod == "idc" {
		cred.Provider = auth.ProviderEnterprise
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		cred.ProfileArn, _ = auth.ResolveProfileArn(ctx, cred)
		cancel()
	}
	authSucceeded(c, store, id, cred)
}
