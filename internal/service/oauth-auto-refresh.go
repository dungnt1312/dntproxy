package service

import (
	"log"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

const oauthAutoRefreshInterval = 60 * time.Second

// RunOAuthAutoRefresh periodically refreshes OAuth connections before access tokens expire.
func RunOAuthAutoRefresh(store port.CredentialStore, done <-chan struct{}) {
	refreshSvc := auth.NewTokenRefreshService(store)
	ticker := time.NewTicker(oauthAutoRefreshInterval)
	defer ticker.Stop()

	refreshAll := func() {
		cfg, err := store.Load()
		if err != nil || cfg == nil {
			return
		}
		for i := range cfg.ProviderConnections {
			conn := cfg.ProviderConnections[i]
			if !shouldAutoRefreshOAuth(&conn) {
				continue
			}
			if !refreshSvc.ShouldProactivelyRefresh(&conn) {
				continue
			}
			// Skip connections the router already sidelined (rate-limited /
			// revoked). Their cooldown clears on its own; hammering them here
			// just burns upstream token-refresh calls.
			if domain.IsAccountUnavailable(conn.RateLimitedUntil) {
				continue
			}
			updated, err := refreshSvc.CheckAndRefresh(&conn)
			if err != nil {
				log.Printf("[TOKEN] Background refresh failed for %s (%s): %s", conn.Name, conn.Provider, err)
				if domain.IsAuthRevokedErrorText(err.Error()) {
					// The refresh token is dead — the account needs re-auth.
					// Sideline it for the long auth-revoked window so the
					// background loop (and the router) stop retrying it every
					// minute; the operator must re-enroll the account.
					sidelineDeadToken(store, &conn)
				}
				continue
			}
			if updated != nil && updated.ID != "" {
				log.Printf("[TOKEN] Background refresh ok for %s (%s)", updated.Name, updated.Provider)
			}
		}
	}

	refreshAll()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			refreshAll()
		}
	}
}

// sidelineDeadToken marks a connection whose refresh token the upstream no
// longer accepts as rate-limited for the auth-revoked window. It records the
// failure reason so the dashboard shows why the account is unavailable.
func sidelineDeadToken(store port.CredentialStore, conn *domain.ProviderConnection) {
	_ = store.Update(func(cfg *domain.AppConfig) {
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID != conn.ID {
				continue
			}
			cfg.ProviderConnections[i].RateLimitedUntil = domain.CooldownUntil(domain.CooldownAuthRevoked)
			cfg.ProviderConnections[i].LastError = "token refresh failed: invalid refresh token (re-auth required)"
			cfg.ProviderConnections[i].LastErrorAt = time.Now().UTC().Format(time.RFC3339)
			return
		}
	})
}

func shouldAutoRefreshOAuth(conn *domain.ProviderConnection) bool {
	if conn == nil || !conn.IsActive {
		return false
	}
	if conn.AuthType != "oauth" || strings.TrimSpace(conn.RefreshToken) == "" {
		return false
	}
	switch conn.Provider {
	case "xai", "openai", "qwen", "kiro":
		return true
	default:
		return false
	}
}