package service

import (
	"log"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

const tokenRefreshInterval = 5 * time.Minute
const tokenRefreshThreshold = 15 * time.Minute

type TokenRefreshScheduler struct {
	store        port.CredentialStore
	tokenRefresh *auth.TokenRefreshService
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

func NewTokenRefreshScheduler(store port.CredentialStore) *TokenRefreshScheduler {
	return &TokenRefreshScheduler{
		store:        store,
		tokenRefresh: auth.NewTokenRefreshService(store),
		stopCh:       make(chan struct{}),
	}
}

func (s *TokenRefreshScheduler) Start() {
	s.wg.Add(1)
	go s.run()
	log.Printf("[SCHEDULER] Token refresh scheduler started (interval=%v, threshold=%v)", tokenRefreshInterval, tokenRefreshThreshold)
}

func (s *TokenRefreshScheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	log.Printf("[SCHEDULER] Token refresh scheduler stopped")
}

func (s *TokenRefreshScheduler) run() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		case <-time.After(tokenRefreshInterval):
			s.refreshExpiringTokens()
		}
	}
}

func (s *TokenRefreshScheduler) refreshExpiringTokens() {
	cfg, err := s.store.Load()
	if err != nil {
		log.Printf("[SCHEDULER] Failed to load config: %s", err)
		return
	}

	for i := range cfg.ProviderConnections {
		conn := &cfg.ProviderConnections[i]
		if !conn.IsActive {
			continue
		}

		if conn.AccessToken == "" || conn.RefreshToken == "" {
			continue
		}

		if !s.needsRefresh(conn) {
			continue
		}

		log.Printf("[SCHEDULER] Token expiring soon for %s (provider=%s), refreshing...", conn.Name, conn.Provider)

		updated, err := s.tokenRefresh.Refresh(conn)
		if err != nil {
			log.Printf("[SCHEDULER] Refresh failed for %s: %s", conn.Name, err)
			continue
		}

		if err := s.store.UpdateConnection(updated); err != nil {
			log.Printf("[SCHEDULER] Failed to persist refreshed token for %s: %s", conn.Name, err)
			continue
		}

		log.Printf("[SCHEDULER] Token refreshed successfully for %s (provider=%s)", conn.Name, conn.Provider)
	}
}

func (s *TokenRefreshScheduler) needsRefresh(conn *domain.ProviderConnection) bool {
	if conn.ExpiresAt == "" {
		return false
	}

	expiresAt, err := time.Parse(time.RFC3339, conn.ExpiresAt)
	if err != nil {
		return false
	}

	timeUntilExpiry := time.Until(expiresAt)
	return timeUntilExpiry > 0 && timeUntilExpiry < tokenRefreshThreshold
}
