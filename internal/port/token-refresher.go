package port

import "github.com/dungnt/dntproxy/internal/domain"

// TokenRefresher handles refreshing OAuth tokens for a provider.
type TokenRefresher interface {
	// NeedsRefresh returns true if the credentials are expiring soon.
	NeedsRefresh(conn *domain.ProviderConnection) bool
	// Refresh performs token refresh and returns updated fields.
	Refresh(conn *domain.ProviderConnection) (*domain.ProviderConnection, error)
}
