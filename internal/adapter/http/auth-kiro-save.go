package http

import (
	"fmt"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// saveKiroLogin stores a finished Kiro login as a provider connection and
// returns it. The same shape is produced by every login flow so refresh and
// routing treat them alike.
func saveKiroLogin(c *gin.Context, store port.CredentialStore, cred *auth.KiroCredential) (*domain.ProviderConnection, string, error) {
	email := auth.ExtractEmailFromJWT(cred.AccessToken)

	cfg, err := store.Load()
	if err != nil {
		return nil, "", fmt.Errorf("load config: %w", err)
	}
	settings := &cfg.Settings

	expiresIn := cred.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}
	now := time.Now().UTC().Format(time.RFC3339)
	conn := domain.ProviderConnection{
		ID:                   uuid.New().String(),
		Provider:             "kiro",
		AuthType:             "oauth",
		Weight:               100,
		IsActive:             true,
		AccessToken:          cred.AccessToken,
		RefreshToken:         cred.RefreshToken,
		ExpiresAt:            time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339),
		ExpiresIn:            expiresIn,
		Email:                email,
		TestStatus:           "active",
		SupportedModels:      domain.GetDefaultConnectionModels(settings, "kiro"),
		ProviderSpecificData: cred.ProviderData(),
		CreatedAt:            now,
		UpdatedAt:            now,
		TenantID:             GetTenantID(c),
	}

	if err := store.Update(func(cfg *domain.AppConfig) {
		conn.Name = email
		if conn.Name == "" {
			conn.Name = fmt.Sprintf("%s Account %d", cred.ProviderLabel(), len(cfg.ProviderConnections)+1)
		}
		cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
	}); err != nil {
		return nil, "", fmt.Errorf("save: %w", err)
	}
	return &conn, email, nil
}

// loginFlowMode returns "auth-code" or "device-code" for a Builder ID / IDC
// start request.
//
// Kiro signs in with authorization-code + PKCE and a loopback listener, which
// only works when the browser can reach dntproxy's 127.0.0.1. That is true when
// the dashboard is opened directly on the dntproxy host; behind a reverse proxy,
// from another machine, or inside a container it is not, and the device-code
// flow is used instead. KIRO_LOGIN_FLOW=auth-code|device-code overrides this.
func loginFlowMode(c *gin.Context) string {
	switch strings.ToLower(strings.TrimSpace(auth.LoginFlowOverride())) {
	case "auth-code", "authcode", "code":
		return "auth-code"
	case "device-code", "device", "devicecode":
		return "device-code"
	}
	if requestIsLoopback(c) {
		return "auth-code"
	}
	return "device-code"
}

// requestIsLoopback reports whether the request reached dntproxy directly from
// the same machine (not through a proxy that forwards for someone else).
func requestIsLoopback(c *gin.Context) bool {
	r := c.Request
	if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-Ip") != "" ||
		r.Header.Get("Forwarded") != "" {
		return false
	}
	return auth.IsLoopbackAddr(r.RemoteAddr)
}
