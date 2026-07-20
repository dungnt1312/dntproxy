package port

import "github.com/dungnt/dntproxy/internal/domain"

// AccountSelector manages multi-account selection with weighted random and cooldown.
type AccountSelector interface {
	// SelectCredentials returns the best available credentials for a provider,
	// using weighted random selection among available connections.
	// excludeIDs: connections to skip (already failed in this request).
	// allowedConnectionIDs: optional restriction to specific connections (from combo config).
	// tenantID: filter connections by tenant (empty = legacy single-tenant mode).
	SelectCredentials(provider string, excludeIDs map[string]bool, model string, allowedConnectionIDs []string, tenantID string) (*domain.Credentials, error)

	// MarkUnavailable marks a connection as unavailable with cooldown.
	MarkUnavailable(connectionID string, status int, errorText string, model string) error

	// ClearError resets a connection's error state after a successful request.
	ClearError(connectionID string, model string) error
}
