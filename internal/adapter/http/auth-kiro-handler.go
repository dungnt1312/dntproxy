package http

import (
	"fmt"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// === Builder ID Device Code Flow ===

func authStartBuilderID() gin.HandlerFunc {
	return func(c *gin.Context) {
		client, deviceAuth, err := auth.StartBuilderIDDeviceAuth()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to start Builder ID auth: " + err.Error()})
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
			ClientID:     client.ClientID,
			ClientSecret: client.ClientSecret,
			DeviceCode:   deviceAuth.DeviceCode,
			Region:       "us-east-1",
			Interval:     deviceAuth.Interval,
			AuthMethod:   "builder-id",
			TenantID:     starterTenant,
			APIKeyID:     starterKey,
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
		starterTenant, starterKey := authCallerIDs(c)
		authSessionsMu.Lock()
		if len(authSessions) >= maxAuthSessions {
			authSessionsMu.Unlock()
			c.JSON(429, gin.H{"error": "too many pending auth sessions"})
			return
		}
		authSessions[sessionID] = &authSession{
			ClientID:     client.ClientID,
			ClientSecret: client.ClientSecret,
			DeviceCode:   deviceAuth.DeviceCode,
			Region:       req.Region,
			StartURL:     req.StartURL,
			Interval:     deviceAuth.Interval,
			AuthMethod:   "idc",
			TenantID:     starterTenant,
			APIKeyID:     starterKey,
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

		if !authSessionAllowed(c, session.TenantID, session.APIKeyID) {
			c.JSON(403, gin.H{"error": "Access denied"})
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
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}
		settings := &cfg.Settings
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
			SupportedModels: domain.GetDefaultConnectionModels(settings, "kiro"),
			ProviderSpecificData: map[string]interface{}{
				"authMethod":   session.AuthMethod,
				"provider":     providerLabel,
				"clientId":     session.ClientID,
				"clientSecret": session.ClientSecret,
				"region":       session.Region,
			},
			CreatedAt: now,
			UpdatedAt: now,
			TenantID:  GetTenantID(c),
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
