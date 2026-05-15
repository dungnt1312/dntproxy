package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/logger"
	"github.com/dungnt/dntproxy/internal/port"
)

// Alerter monitors the log stream and sends alerts to the bot owner.
type Alerter struct {
	bot   *Bot
	store port.CredentialStore
	ch    chan *domain.LogEntry
	cancel context.CancelFunc
}

// NewAlerter creates an alerter that subscribes to the logger broadcast.
func NewAlerter(bot *Bot, store port.CredentialStore) *Alerter {
	return &Alerter{
		bot:   bot,
		store: store,
	}
}

// Start begins monitoring the log stream for alertable events.
func (a *Alerter) Start() {
	a.ch = logger.Get().Subscribe()
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	go a.loop(ctx)
	log.Printf("[TELEGRAM] Alerter started")
}

// Stop unsubscribes from the log stream and stops the alerter.
func (a *Alerter) Stop() {
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	if a.ch != nil {
		logger.Get().Unsubscribe(a.ch)
		a.ch = nil
	}
	log.Printf("[TELEGRAM] Alerter stopped")
}

func (a *Alerter) loop(ctx context.Context) {
	// Periodic check for all-down condition
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-a.ch:
			if !ok {
				return
			}
			a.processEntry(entry)
		case <-ticker.C:
			a.checkAllDown()
		}
	}
}

func (a *Alerter) processEntry(entry *domain.LogEntry) {
	if entry == nil {
		return
	}

	// Only process request-level errors (not event logs)
	if entry.Direction != "response" && entry.Direction != "event" {
		return
	}

	// Check for recovery: successful response for a previously-alerted connection
	if entry.Level == "INFO" && entry.Direction == "response" && entry.StatusCode >= 200 && entry.StatusCode < 300 {
		if entry.ConnectionID != "" && a.bot.dedup.HasEntries(entry.ConnectionID) {
			a.bot.NotifyRecovered(entry.ConnectionID, entry.ConnectionName, entry.Provider)
		}
		return
	}

	// Only process errors
	if entry.Level != "ERROR" {
		return
	}

	alert := a.classifyError(entry)
	if alert == nil {
		return
	}

	_ = a.bot.SendAlert(*alert)
}

func (a *Alerter) classifyError(entry *domain.LogEntry) *port.Alert {
	alert := &port.Alert{
		ConnectionID: entry.ConnectionID,
		Connection:   entry.ConnectionName,
		Provider:     entry.Provider,
		Model:        entry.Model,
		Message:      entry.Message,
	}

	// Classify by status code
	switch {
	case entry.StatusCode == 402 || entry.StatusCode == 403:
		alert.Type = port.AlertQuotaExhausted
	case entry.StatusCode == 429:
		alert.Type = port.AlertRateLimited
	case entry.StatusCode == 401:
		// Check if it's a token issue
		if isTokenError(entry.Message) || isTokenError(entry.Error) {
			alert.Type = port.AlertTokenExpired
		} else {
			return nil // generic 401, not worth alerting
		}
	default:
		// Check for token refresh failures in event logs
		if entry.Direction == "event" && isTokenRefreshFailure(entry.Message) {
			alert.Type = port.AlertTokenExpired
			return alert
		}

		// Check for combo exhaustion
		if isComboExhausted(entry.Message) {
			alert.Type = port.AlertComboExhausted
			alert.Combo = extractComboName(entry.Message)
			return alert
		}

		// Check connection backoff level for connection_down
		if entry.ConnectionID != "" {
			conn, err := a.store.GetConnectionByID(entry.ConnectionID)
			if err == nil && conn != nil && conn.BackoffLevel >= 4 {
				alert.Type = port.AlertConnectionDown
				return alert
			}
		}

		return nil // not an alertable error
	}

	return alert
}

func (a *Alerter) checkAllDown() {
	cfg, err := a.store.Load()
	if err != nil {
		return
	}

	// Group connections by provider
	providerConns := make(map[string][]domain.ProviderConnection)
	for _, conn := range cfg.ProviderConnections {
		if conn.IsActive {
			providerConns[conn.Provider] = append(providerConns[conn.Provider], conn)
		}
	}

	now := time.Now()
	for provider, conns := range providerConns {
		allDown := true
		for _, conn := range conns {
			isDown := false

			// Check rate-limited
			if conn.RateLimitedUntil != "" {
				if t, err := time.Parse(time.RFC3339, conn.RateLimitedUntil); err == nil && t.After(now) {
					isDown = true
				}
			}

			// Check token expired
			if !isDown && conn.ExpiresAt != "" {
				if t, err := time.Parse(time.RFC3339, conn.ExpiresAt); err == nil && t.Before(now) {
					isDown = true
				}
			}

			// Check high backoff
			if !isDown && conn.BackoffLevel >= 4 {
				isDown = true
			}

			if !isDown {
				allDown = false
				break
			}
		}

		if allDown && len(conns) > 0 {
			_ = a.bot.SendAlert(port.Alert{
				Type:     port.AlertAllDown,
				Provider: provider,
				Message:  fmt.Sprintf("All %d connections for %s are unavailable", len(conns), provider),
			})
		}
	}
}

func isTokenError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "token expired") ||
		strings.Contains(lower, "token invalid") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "refresh token")
}

func isTokenRefreshFailure(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "token refresh failed") ||
		strings.Contains(lower, "failed to refresh")
}

func isComboExhausted(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "combo exhausted") ||
		strings.Contains(lower, "all models in combo failed")
}

func extractComboName(msg string) string {
	// Try to extract combo name from message like "combo exhausted: my-combo"
	if idx := strings.Index(msg, ":"); idx > 0 && idx < len(msg)-1 {
		return strings.TrimSpace(msg[idx+1:])
	}
	return ""
}
