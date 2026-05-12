package port

import "github.com/dungnt/dntproxy/internal/domain"

// CredentialStore manages provider connections and app config persistence.
type CredentialStore interface {
	// Load reads the full config from storage.
	Load() (*domain.AppConfig, error)
	// Save writes the full config to storage.
	Save(cfg *domain.AppConfig) error
	// Update loads config, applies fn, and saves atomically (single lock).
	Update(fn func(cfg *domain.AppConfig)) error

	// GetActiveConnections returns active connections for a provider.
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
	// GetAPIKeyByValue returns the full APIKey object for a given key string.
	// Returns nil, false if not found or inactive.
	GetAPIKeyByValue(key string) (*domain.APIKey, bool)

	// GetSettings returns app settings.
	GetSettings() (*domain.Settings, error)

	// GetModelRegistry returns the model registry.
	GetModelRegistry() (*domain.ModelRegistry, error)

	// GetConnectionIDsForCombo returns ConnectionIDs for a combo name (empty if combo not found or no restriction).
	GetConnectionIDsForCombo(comboName string) ([]string, error)
}
