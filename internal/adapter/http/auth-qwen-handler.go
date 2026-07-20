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

// === Qwen OAuth (Device Code Flow) ===

type qwenSession struct {
	TenantID     string
	APIKeyID     string
	DeviceCode   string
	CodeVerifier string
	Interval     int
	CreatedAt    time.Time
}

func cleanupQwenSessions() {
	qwenSessionsMu.Lock()
	defer qwenSessionsMu.Unlock()
	now := time.Now()
	for id, s := range qwenSessions {
		if now.Sub(s.CreatedAt) > 15*time.Minute {
			delete(qwenSessions, id)
		}
	}
}

func authQwenStart() gin.HandlerFunc {
	return func(c *gin.Context) {
		codeVerifier, codeChallenge, _, err := auth.GeneratePKCE()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to generate PKCE: " + err.Error()})
			return
		}

		deviceAuth, err := auth.StartQwenDeviceAuth(codeChallenge)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to start Qwen auth: " + err.Error()})
			return
		}

		sessionID := uuid.New().String()
		starterTenant, starterKey := authCallerIDs(c)
		qwenSessionsMu.Lock()
		if len(qwenSessions) >= maxAuthSessions {
			qwenSessionsMu.Unlock()
			c.JSON(429, gin.H{"error": "too many pending auth sessions"})
			return
		}
		qwenSessions[sessionID] = &qwenSession{
			TenantID:     starterTenant,
			APIKeyID:     starterKey,
			DeviceCode:   deviceAuth.DeviceCode,
			CodeVerifier: codeVerifier,
			Interval:     deviceAuth.Interval,
			CreatedAt:    time.Now(),
		}
		qwenSessionsMu.Unlock()

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

func authQwenPoll(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SessionID string `json:"sessionId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
			c.JSON(400, gin.H{"error": "sessionId is required"})
			return
		}

		qwenSessionsMu.Lock()
		session, ok := qwenSessions[req.SessionID]
		qwenSessionsMu.Unlock()

		if !ok {
			c.JSON(404, gin.H{"error": "Session not found or expired"})
			return
		}

		if !authSessionAllowed(c, session.TenantID, session.APIKeyID) {
			c.JSON(403, gin.H{"error": "Access denied"})
			return
		}

		result, err := auth.PollQwenDeviceToken(session.DeviceCode, session.CodeVerifier)
		if err != nil {
			c.JSON(500, gin.H{"error": "Poll failed: " + err.Error()})
			return
		}

		if result.Pending {
			c.JSON(200, gin.H{"status": "pending"})
			return
		}

		if !result.Success {
			qwenSessionsMu.Lock()
			delete(qwenSessions, req.SessionID)
			qwenSessionsMu.Unlock()
			c.JSON(200, gin.H{"status": "error", "error": result.Error, "errorDescription": result.ErrorDescription})
			return
		}

		// Success — create Qwen connection
		tokens := result.Tokens
		email := auth.ExtractEmailFromJWT(tokens.AccessToken)

		name := email
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}
		settings := &cfg.Settings
		if name == "" {
			name = "Qwen Account"
			count := 0
			for _, conn := range cfg.ProviderConnections {
				if conn.Provider == "qwen" {
					count++
				}
			}
			if count > 0 {
				name = fmt.Sprintf("Qwen Account %d", count+1)
			}
		}

		now := time.Now().UTC().Format(time.RFC3339)
		expiresIn := tokens.ExpiresIn
		if expiresIn == 0 {
			expiresIn = 3600
		}

		conn := domain.ProviderConnection{
			ID:              uuid.New().String(),
			Provider:        "qwen",
			AuthType:        "oauth",
			Name:            name,
			Weight:          100,
			IsActive:        true,
			AccessToken:     tokens.AccessToken,
			RefreshToken:    tokens.RefreshToken,
			BaseURL:         auth.QwenConfig.APIBaseURL,
			ExpiresAt:       time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
			ExpiresIn:       expiresIn,
			Email:           email,
			TestStatus:      "active",
			SupportedModels: domain.GetDefaultConnectionModels(settings, "qwen"),
			ProviderSpecificData: map[string]interface{}{
				"authMethod": "qwen-oauth",
				"provider":   "Qwen (Alibaba)",
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

		qwenSessionsMu.Lock()
		delete(qwenSessions, req.SessionID)
		qwenSessionsMu.Unlock()

		c.JSON(200, gin.H{
			"status": "success",
			"id":     conn.ID,
			"name":   conn.Name,
			"email":  email,
		})
	}
}
