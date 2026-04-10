package service

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// AccountSelector manages multi-account selection with fallback and cooldown.
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

// SelectCredentials returns the best available credentials for a provider,
// excluding connections in the excludeIDs set.
func (s *AccountSelector) SelectCredentials(provider string, excludeIDs map[string]bool, model string) (*domain.Credentials, error) {
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

	supportedCount := 0
	rateLimitedSupportedCount := 0
	lockedSupportedCount := 0
	nonExcludedCount := 0

	for _, conn := range connections {
		// Skip excluded connections
		if excludeIDs != nil && excludeIDs[conn.ID] {
			continue
		}
		nonExcludedCount++

		// Skip connections that don't support this model
		if !conn.SupportsModel(model) {
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

		// Auto-refresh token if expiring soon
		if s.tokenRefresh.NeedsRefresh(&conn) {
			log.Printf("[AUTH] Token expiring soon for %s, refreshing...", conn.Name)
			refreshed, err := s.tokenRefresh.CheckAndRefresh(&conn)
			if err != nil {
				log.Printf("[AUTH] Token refresh failed for %s: %s", conn.Name, err)
				// Still try with current token
			} else {
				conn = *refreshed
			}
		}

		return connectionToCredentials(&conn), nil
	}

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

// MarkUnavailable marks a connection as unavailable with cooldown.
func (s *AccountSelector) MarkUnavailable(connectionID string, status int, errorText string, model string) error {
	conn, err := s.store.GetConnectionByID(connectionID)
	if err != nil || conn == nil {
		return fmt.Errorf("connection not found: %s", connectionID)
	}

	result := domain.CheckFallbackError(status, errorText, conn.BackoffLevel)
	if !result.ShouldFallback {
		return nil
	}

	if result.CooldownMs > 0 {
		conn.RateLimitedUntil = domain.CooldownUntil(result.CooldownMs)
		conn.BackoffLevel = result.NewBackoffLevel
		conn.LastError = errorText
		conn.LastErrorAt = time.Now().UTC().Format(time.RFC3339)

		// Set model lock if applicable
		if model != "" {
			if conn.ModelLocks == nil {
				conn.ModelLocks = make(map[string]string)
			}
			conn.ModelLocks[model] = domain.CooldownUntil(result.CooldownMs)
		}
	}

	return s.store.UpdateConnection(conn)
}

// ClearError resets a connection's error state after a successful request.
func (s *AccountSelector) ClearError(connectionID string, model string) error {
	conn, err := s.store.GetConnectionByID(connectionID)
	if err != nil || conn == nil {
		return nil
	}

	conn.RateLimitedUntil = ""
	conn.BackoffLevel = 0
	conn.LastError = ""
	conn.LastErrorAt = ""

	// Clear model lock
	if model != "" && conn.ModelLocks != nil {
		delete(conn.ModelLocks, model)
	}

	return s.store.UpdateConnection(conn)
}

func connectionToCredentials(conn *domain.ProviderConnection) *domain.Credentials {
	creds := &domain.Credentials{
		ConnectionID:         conn.ID,
		ConnectionName:       conn.Name,
		AccessToken:          conn.AccessToken,
		RefreshToken:         conn.RefreshToken,
		APIKey:               conn.APIKey,
		BaseURL:              conn.BaseURL,
		ProviderSpecificData: conn.ProviderSpecificData,
	}

	// Extract profileArn from providerSpecificData
	if conn.ProviderSpecificData != nil {
		if v, ok := conn.ProviderSpecificData["profileArn"]; ok {
			if s, ok := v.(string); ok {
				creds.ProfileArn = s
			}
		}
	}

	return creds
}
