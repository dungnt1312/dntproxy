package service

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
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

		// Skip connections that don't support this model.
		// Exception: OpenAI OAuth connections (ChatGPT tokens) use the Codex Responses API
		// which accepts any ChatGPT model slug — the upstream validates the model.
		// Their stored supportedModels list contains ChatGPT slugs (e.g. "gpt-4o", "o4-mini")
		// that don't match standard API IDs, so skipping this check here is correct.
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

		return shared.ConnectionToCredentials(&conn), nil
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
	var result *domain.FallbackResult
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

		fb := domain.CheckFallbackError(status, errorText, conn.BackoffLevel)
		if !fb.ShouldFallback {
			return
		}
		result = &fb

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
	if result == nil {
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
		}
	})
}
