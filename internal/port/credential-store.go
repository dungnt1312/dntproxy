package port

import "github.com/dungnt/dntproxy/internal/domain"

// CredentialStore manages provider connections and app config persistence.
type CredentialStore interface {
	// Load reads the full config from storage.
	Load() (*domain.AppConfig, error)
	// Save writes the full config to storage.
	Save(cfg *domain.AppConfig) error

	// GetActiveConnections returns active connections for a provider, sorted by priority.
	GetActiveConnections(provider string) ([]domain.ProviderConnection, error)
	// GetConnectionByID returns a single connection.
	GetConnectionByID(id string) (*domain.ProviderConnection, error)
	// UpdateConnection persists changes to a connection.
	UpdateConnection(conn *domain.ProviderConnection) error

	// GetCombos returns all combos.
	GetCombos() ([]domain.Combo, error)
	// GetComboByName returns a combo by name.
	GetComboByName(name string) (*domain.Combo, error)

	// GetModelAliases returns all model aliases.
	GetModelAliases() (domain.AliasMap, error)

	// GetAPIKeys returns all API keys.
	GetAPIKeys() ([]domain.APIKey, error)
	// ValidateAPIKey checks if a key string is valid and active.
	ValidateAPIKey(key string) bool

	// GetSettings returns app settings.
	GetSettings() (*domain.Settings, error)
}
