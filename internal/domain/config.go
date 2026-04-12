package domain

// AppConfig represents the full application configuration / database.
type AppConfig struct {
	ProviderConnections []ProviderConnection `json:"providerConnections"`
	Combos              []Combo              `json:"combos"`
	ModelAliases        AliasMap             `json:"modelAliases"`
	APIKeys             []APIKey             `json:"apiKeys"`
	Settings            Settings             `json:"settings"`
	ModelRegistry       *ModelRegistry       `json:"modelRegistry,omitempty"`
}

// Settings holds app-level settings.
type Settings struct {
	StickyRoundRobinLimit int               `json:"stickyRoundRobinLimit"`
	ComboStrategy         string            `json:"comboStrategy"`
	ComboStrategies       map[string]string `json:"comboStrategies,omitempty"`
	RequireAPIKey         bool              `json:"requireApiKey"`
	Port                  int               `json:"port,omitempty"`
	// Tunnel settings
	TunnelEnabled  bool   `json:"tunnelEnabled,omitempty"`
	TunnelURL      string `json:"tunnelUrl,omitempty"`
	TunnelProvider string `json:"tunnelProvider,omitempty"` // "cloudflare"
	TunnelShortID  string `json:"tunnelShortId,omitempty"`
	TunnelRunning  bool   `json:"tunnelRunning,omitempty"`
}

// APIKey represents a generated API key.
type APIKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	IsActive  bool   `json:"isActive"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// DefaultConfig returns a new AppConfig with sensible defaults.
func DefaultConfig() AppConfig {
	return AppConfig{
		ProviderConnections: []ProviderConnection{},
		Combos:              []Combo{},
		ModelAliases:        AliasMap{},
		APIKeys:             []APIKey{},
		Settings: Settings{
			StickyRoundRobinLimit: 3,
			ComboStrategy:         "fallback",
			RequireAPIKey:         false,
			Port:                  20128,
		},
		ModelRegistry: DefaultModelRegistry(),
	}
}
