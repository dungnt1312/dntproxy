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
			updated, err := refreshSvc.CheckAndRefresh(&conn)
			if err != nil {
				log.Printf("[TOKEN] Background refresh failed for %s (%s): %s", conn.Name, conn.Provider, err)
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