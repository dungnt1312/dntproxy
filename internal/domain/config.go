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

// TelegramSettings controls the embedded Telegram bot for alerts and commands.
type TelegramSettings struct {
	Enabled    bool   `json:"enabled"`
	BotToken   string `json:"botToken,omitempty"`
	OwnerID    int64  `json:"ownerId,omitempty"`
	MutedUntil string `json:"mutedUntil,omitempty"` // RFC3339, suppress alerts until this time
}

// Settings holds app-level settings.
type Settings struct {
	ComboStrategy   string            `json:"comboStrategy"`
	ComboStrategies map[string]string `json:"comboStrategies,omitempty"`
	// ConnectionStrategy controls how an account is selected after model routing.
	// Supported: weighted-random, priority-fallback, round-robin.
	ConnectionStrategy string `json:"connectionStrategy,omitempty"`
	RequireAPIKey      bool   `json:"requireApiKey"`
	Port               int    `json:"port,omitempty"`
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
	// Logging settings
	LogBodies bool `json:"logBodies"` // persist request/response bodies in SQLite (default: false)
	// Telegram bot settings
	Telegram TelegramSettings `json:"telegram,omitempty"`
	// Migration flags
		DashboardAccessMigrated bool `json:"dashboardAccessMigrated,omitempty"`
		// Disable image generation endpoints (default: false)
		DisableImageGeneration bool `json:"disableImageGeneration,omitempty"`
}

// APIKey represents a generated API key.
type APIKey struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Key                  string   `json:"key"`
	IsActive             bool     `json:"isActive"`
	DashboardAccess      bool     `json:"dashboardAccess"` // true = can access /api/* dashboard
	CreatedAt            string   `json:"createdAt,omitempty"`
	AllowedConnectionIDs []string `json:"allowedConnectionIds,omitempty"` // nil/empty = unrestricted
	AllowedModels        []string `json:"allowedModels,omitempty"`        // nil/empty = unrestricted
}

// DefaultConfig returns a new AppConfig with sensible defaults.
func DefaultConfig() AppConfig {
	return AppConfig{
		ProviderConnections: []ProviderConnection{},
		Combos:              []Combo{},
		ModelAliases:        AliasMap{},
		APIKeys:             []APIKey{},
		Settings: Settings{
			ComboStrategy:      "fallback",
			ConnectionStrategy: "weighted-random",
			RequireAPIKey:      false,
			Port:               20199,
			Compression: CompressionSettings{
				Enabled:          false,
				MinContentLength: 500,
				LogSavings:       true,
			},
		},
		ModelRegistry: DefaultModelRegistry(),
	}
}
