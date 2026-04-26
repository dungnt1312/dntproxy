package service

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// AccountSelector manages multi-account selection with weighted random and cooldown.
type AccountSelector struct {
	store        port.CredentialStore
	tokenRefresh *auth.TokenRefreshService
}

type AccountSelectionErrorKind string

const (
	SelectionErrNoActiveCredentials AccountSelectionErrorKind = "no_active_credentials"
	SelectionErrUnsupportedModel    AccountSelectionErrorKind = "unsupported_model"
	SelectionErrRateLimited         AccountSelectionErrorKind = "rate_limited"
	SelectionErrModelLocked         AccountSelectionErrorKind = "model_locked"
	SelectionErrUnavailable         AccountSelectionErrorKind = "unavailable"

	// DefaultWeight is the weight assigned to connections with zero/unset weight.
	DefaultWeight = 100
)

type AccountSelectionError struct {
	Kind     AccountSelectionErrorKind
	Provider string
	Model    string
}

func (e *AccountSelectionError) Error() string {
	switch e.Kind {
	case SelectionErrNoActiveCredentials:
		return fmt.Sprintf("no active credentials for provider: %s", e.Provider)
	case SelectionErrUnsupportedModel:
		return fmt.Sprintf("no accounts support model %q for provider: %s", e.Model, e.Provider)
	case SelectionErrRateLimited:
		return fmt.Sprintf("all accounts rate limited for provider: %s", e.Provider)
	case SelectionErrModelLocked:
		return fmt.Sprintf("all accounts locked for model %q on provider: %s", e.Model, e.Provider)
	default:
		return fmt.Sprintf("no available accounts for provider: %s", e.Provider)
	}
}

func IsAccountSelectionError(err error, kind AccountSelectionErrorKind) bool {
	var selErr *AccountSelectionError
	if !errors.As(err, &selErr) {
		return false
	}
	return selErr.Kind == kind
}

// NewAccountSelector creates a new AccountSelector.
func NewAccountSelector(store port.CredentialStore) *AccountSelector {
	return &AccountSelector{
		store:        store,
		tokenRefresh: auth.NewTokenRefreshService(store),
	}
}

// SelectCredentialsForModel selects account based on model string (with optional @connectionId).
// If model string contains @connectionId, it will pin to that specific connection.
// Otherwise, it uses the existing weighted random selection logic.
func (s *AccountSelector) SelectCredentialsForModel(
	modelStr string,
	excludeIDs map[string]bool,
	allowedConnectionIDs []string,
) (*domain.Credentials, error) {
	parsed, err := ParseModelString(modelStr)
	if err != nil {
		return nil, fmt.Errorf("parse model: %w", err)
	}

	// If pinned to specific connection
	if parsed.ConnectionID != "" {
		// Pass only the model name (without @connectionId) to selectPinnedConnection
		return s.selectPinnedConnection(parsed.Provider, parsed.Model, parsed.ConnectionID, excludeIDs)
	}

	// Otherwise use existing weighted random logic
	return s.SelectCredentials(parsed.Provider, excludeIDs, parsed.Model, allowedConnectionIDs)
}

// selectPinnedConnection returns a specific connection by ID.
// It still respects rate limits and model locks, but bypasses weighted random selection.
func (s *AccountSelector) selectPinnedConnection(
	provider string,
	model string,
	connectionID string,
	excludeIDs map[string]bool,
) (*domain.Credentials, error) {
	// Skip if already excluded (failed in this request)
	if excludeIDs != nil && excludeIDs[connectionID] {
		return nil, &AccountSelectionError{
			Kind:     SelectionErrUnavailable,
			Provider: provider,
			Model:    model,
		}
	}

	connections, err := s.store.GetActiveConnections(provider)
	if err != nil {
		return nil, fmt.Errorf("get connections: %w", err)
	}

	for _, conn := range connections {
		if conn.ID != connectionID {
			continue
		}

		// Check if supports model (model should NOT contain @connectionId here)
		// Exception: OpenAI OAuth connections accept any ChatGPT model slug
		isOpenAIOAuth := conn.Provider == "openai" && conn.AuthType == "oauth"
		if !isOpenAIOAuth && !conn.SupportsModel(model) {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrUnsupportedModel,
				Provider: provider,
				Model:    model,
			}
		}

		// Check if rate limited
		if domain.IsAccountUnavailable(conn.RateLimitedUntil) {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrRateLimited,
				Provider: provider,
				Model:    model,
			}
		}

		// Check if model locked
		if domain.IsModelLockActive(conn.ModelLocks, model) {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrModelLocked,
				Provider: provider,
				Model:    model,
			}
		}

		// Auto-refresh token if needed
		if s.tokenRefresh.NeedsRefresh(&conn) {
			refreshed, err := s.tokenRefresh.CheckAndRefresh(&conn)
			if err == nil {
				conn = *refreshed
			}
		}

		return shared.ConnectionToCredentials(&conn), nil
	}

	return nil, &AccountSelectionError{
		Kind:     SelectionErrNoActiveCredentials,
		Provider: provider,
		Model:    model,
	}
}

// SelectCredentials returns a weighted-random available connection for a provider+model,
// filtered by excludeIDs and optional allowedConnectionIDs restriction.
func (s *AccountSelector) SelectCredentials(
	provider string,
	excludeIDs map[string]bool,
	model string,
	allowedConnectionIDs []string,
) (*domain.Credentials, error) {
	connections, err := s.store.GetActiveConnections(provider)
	if err != nil {
		return nil, fmt.Errorf("get connections: %w", err)
	}

	if len(connections) == 0 {
		return nil, &AccountSelectionError{
			Kind:     SelectionErrNoActiveCredentials,
			Provider: provider,
			Model:    model,
		}
	}

	// Filter to only available connections
	var available []domain.ProviderConnection
	supportedCount := 0
	rateLimitedSupportedCount := 0
	lockedSupportedCount := 0
	nonExcludedCount := 0

	for _, conn := range connections {
		// Skip excluded connections (already failed in this request)
		if excludeIDs != nil && excludeIDs[conn.ID] {
			continue
		}
		nonExcludedCount++

		// Skip connections not in allowedConnectionIDs (combo restriction)
		if len(allowedConnectionIDs) > 0 {
			allowed := false
			for _, id := range allowedConnectionIDs {
				if conn.ID == id {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		// Skip connections that don't support this model.
		// Exception: OpenAI OAuth connections (ChatGPT tokens) use the Codex Responses API
		// which accepts any ChatGPT model slug — the upstream validates the model.
		isOpenAIOAuth := conn.Provider == "openai" && conn.AuthType == "oauth"
		if !isOpenAIOAuth && !conn.SupportsModel(model) {
			continue
		}
		supportedCount++

		// Skip rate-limited connections
		if domain.IsAccountUnavailable(conn.RateLimitedUntil) {
			rateLimitedSupportedCount++
			continue
		}

		// Skip model-locked connections
		if domain.IsModelLockActive(conn.ModelLocks, model) {
			lockedSupportedCount++
			continue
		}

		available = append(available, conn)
	}

	if len(available) == 0 {
		// Return appropriate error based on what filtered them out
		if nonExcludedCount == 0 {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrUnavailable,
				Provider: provider,
				Model:    model,
			}
		}
		if supportedCount == 0 {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrUnsupportedModel,
				Provider: provider,
				Model:    model,
			}
		}
		if rateLimitedSupportedCount == supportedCount {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrRateLimited,
				Provider: provider,
				Model:    model,
			}
		}
		if lockedSupportedCount == supportedCount {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrModelLocked,
				Provider: provider,
				Model:    model,
			}
		}
		return nil, &AccountSelectionError{
			Kind:     SelectionErrUnavailable,
			Provider: provider,
			Model:    model,
		}
	}

	// Weighted random selection
	selected := weightedRandomSelect(available)

	// Auto-refresh token if expiring soon
	if s.tokenRefresh.NeedsRefresh(selected) {
		log.Printf("[AUTH] Token expiring soon for %s, refreshing...", selected.Name)
		refreshed, err := s.tokenRefresh.CheckAndRefresh(selected)
		if err != nil {
			log.Printf("[AUTH] Token refresh failed for %s: %s", selected.Name, err)
			// Still try with current token
		} else {
			selected = refreshed
		}
	}

	return shared.ConnectionToCredentials(selected), nil
}

// weightedRandomSelect picks a connection from the available list using weighted random.
// Weight represents probability: higher weight = more likely to be selected.
// Connections with weight <= 0 are treated as DefaultWeight (100).
func weightedRandomSelect(available []domain.ProviderConnection) *domain.ProviderConnection {
	if len(available) == 1 {
		return &available[0]
	}

	totalWeight := 0
	for _, conn := range available {
		w := conn.Weight
		if w <= 0 {
			w = DefaultWeight
		}
		totalWeight += w
	}

	r := rand.Intn(totalWeight)
	cumulative := 0
	for i := range available {
		w := available[i].Weight
		if w <= 0 {
			w = DefaultWeight
		}
		cumulative += w
		if r < cumulative {
			return &available[i]
		}
	}

	// Fallback (should never reach here)
	return &available[0]
}

// MarkUnavailable marks a connection as unavailable with cooldown.
func (s *AccountSelector) MarkUnavailable(connectionID string, status int, errorText string, model string) error {
	connFound := false
	err := s.store.Update(func(cfg *domain.AppConfig) {
		var conn *domain.ProviderConnection
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == connectionID {
				conn = &cfg.ProviderConnections[i]
				break
			}
		}
		if conn == nil {
			return
		}
		connFound = true

		fb := domain.CheckFallbackError(status, errorText, conn.BackoffLevel)
		if !fb.ShouldFallback {
			return
		}

		if fb.CooldownMs > 0 {
			conn.RateLimitedUntil = domain.CooldownUntil(fb.CooldownMs)
			conn.BackoffLevel = fb.NewBackoffLevel
			conn.LastError = errorText
			conn.LastErrorAt = time.Now().UTC().Format(time.RFC3339)

			if model != "" {
				if conn.ModelLocks == nil {
					conn.ModelLocks = make(map[string]string)
				}
				conn.ModelLocks[model] = domain.CooldownUntil(fb.CooldownMs)
			}
		}
	})
	if err != nil {
		return err
	}
	if !connFound {
		return fmt.Errorf("connection not found: %s", connectionID)
	}
	return nil
}

// ClearError resets a connection's error state after a successful request.
func (s *AccountSelector) ClearError(connectionID string, model string) error {
	return s.store.Update(func(cfg *domain.AppConfig) {
		var conn *domain.ProviderConnection
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == connectionID {
				conn = &cfg.ProviderConnections[i]
				break
			}
		}
		if conn == nil {
			return
		}

		conn.RateLimitedUntil = ""
		conn.BackoffLevel = 0
		conn.LastError = ""
		conn.LastErrorAt = ""

		if model != "" && conn.ModelLocks != nil {
			delete(conn.ModelLocks, model)

			// Clean up any expired model locks while we're here
			now := time.Now()
			for k, expiry := range conn.ModelLocks {
				if t, err := time.Parse(time.RFC3339, expiry); err == nil && t.Before(now) {
					delete(conn.ModelLocks, k)
				}
			}
		}
	})
}
