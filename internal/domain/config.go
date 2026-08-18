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
	// TenantID supports multi-tenancy (SaaS). Empty string = legacy single-tenant mode.
	TenantID string `json:"tenantId,omitempty"`
	// Tenants is the registry of customer tenants (admin-managed).
	Tenants []Tenant `json:"tenants,omitempty"`
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
	// ConnectionStrategy controls how an account is selected after model routing.
	// Supported: weighted-random, priority-fallback, round-robin, fill-first.
	// Used as fallback when a provider has no entry in ConnectionStrategies.
	ConnectionStrategy string `json:"connectionStrategy,omitempty"`
	// ConnectionStrategies overrides ConnectionStrategy per provider.
	// Key = provider ID (e.g. "kiro", "openai"), value = strategy name.
	ConnectionStrategies map[string]string `json:"connectionStrategies,omitempty"`
	RequireAPIKey        bool              `json:"requireApiKey"`
	Port                 int               `json:"port,omitempty"`
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
	// Migration flags
	DashboardAccessMigrated bool `json:"dashboardAccessMigrated,omitempty"`
	// AdminKeyBootstrapped records that a default admin key was auto-created
	// on first launch. Prevents re-creating one on subsequent loads.
	AdminKeyBootstrapped bool `json:"adminKeyBootstrapped,omitempty"`
	// Disable image generation endpoints (default: false)
	DisableImageGeneration bool `json:"disableImageGeneration,omitempty"`
	// DefaultModels overrides the built-in RecommendedModels when creating connections.
	// Keyed by provider ID (e.g. "xai", "openai"), value is the model list to use.
	DefaultModels map[string][]string `json:"defaultModels,omitempty"`
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
	TenantID             string   `json:"tenantId,omitempty"`             // for multi-tenancy (SaaS). Empty = legacy single-tenant.
}

// DefaultConfig returns a new AppConfig with sensible defaults.
func DefaultConfig() AppConfig {
	return AppConfig{
		ProviderConnections: []ProviderConnection{},
		Combos:              []Combo{},
		ModelAliases:        AliasMap{},
		APIKeys:             []APIKey{},
		Tenants:             []Tenant{},
		Settings: Settings{
			ComboStrategy:      "fallback",
			ConnectionStrategy: "weighted-random",
			RequireAPIKey:      true,
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
