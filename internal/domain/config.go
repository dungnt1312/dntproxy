package domain

// AppConfig represents the full application configuration / database.
type AppConfig struct {
	ProviderConnections []ProviderConnection `json:"providerConnections"`
	Combos              []Combo              `json:"combos"`
	ModelAliases        AliasMap             `json:"modelAliases"`
	APIKeys             []APIKey             `json:"apiKeys"`
	Settings            Settings             `json:"settings"`
	ModelRegistry       *ModelRegistry       `json:"modelRegistry,omitempty"`
	Profiles            []Profile            `json:"profiles,omitempty"`
}

// CompressionSettings controls request body compression middleware.
type CompressionSettings struct {
	Enabled          bool `json:"enabled"`
	MinContentLength int  `json:"minContentLength,omitempty"` // default 500
	LogSavings       bool `json:"logSavings"`
}

// Normalize sets safe defaults for zero values loaded from JSON.
func (c *CompressionSettings) Normalize() {
	if c.MinContentLength <= 0 {
		c.MinContentLength = 500
	}
}

// Settings holds app-level settings.
type Settings struct {
	ComboStrategy   string            `json:"comboStrategy"`
	ComboStrategies map[string]string `json:"comboStrategies,omitempty"`
	RequireAPIKey   bool              `json:"requireApiKey"`
	Port            int               `json:"port,omitempty"`
	// Profile settings
	ActiveProfile string `json:"activeProfile,omitempty"`
	// Tunnel settings
	TunnelEnabled  bool   `json:"tunnelEnabled,omitempty"`
	TunnelURL      string `json:"tunnelUrl,omitempty"`
	TunnelProvider string `json:"tunnelProvider,omitempty"` // "cloudflare"
	TunnelShortID  string `json:"tunnelShortId,omitempty"`
	TunnelRunning  bool   `json:"tunnelRunning,omitempty"`
	// Compression settings
	Compression CompressionSettings `json:"compression,omitempty"`
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
			ComboStrategy: "fallback",
			RequireAPIKey: false,
			Port:          20199,
			Compression: CompressionSettings{
				Enabled:          false,
				MinContentLength: 500,
				LogSavings:       true,
			},
		},
		ModelRegistry: DefaultModelRegistry(),
	}
}
