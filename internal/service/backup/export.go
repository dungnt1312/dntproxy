package backup

import (
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// ExportOption configures export behavior.
type ExportOption func(*exportConfig)

type exportConfig struct {
	maskTokens   bool
	skipRegistry bool
}

// WithMask enables token/key masking.
func WithMask(enabled bool) ExportOption {
	return func(c *exportConfig) {
		c.maskTokens = enabled
	}
}

// WithSkipRegistry excludes ModelRegistry from backup.
func WithSkipRegistry(enabled bool) ExportOption {
	return func(c *exportConfig) {
		c.skipRegistry = enabled
	}
}

// Export loads config from store and returns a BackupData snapshot.
func Export(store port.CredentialStore, opts ...ExportOption) (*BackupData, error) {
	cfg := &exportConfig{}
	for _, o := range opts {
		o(cfg)
	}

	appCfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	// Clone connections with optional masking
	connections := make([]domain.ProviderConnection, len(appCfg.ProviderConnections))
	for i, conn := range appCfg.ProviderConnections {
		connections[i] = conn
		if cfg.maskTokens {
			connections[i].AccessToken = maskString(conn.AccessToken, 4, 4)
			connections[i].RefreshToken = maskString(conn.RefreshToken, 4, 4)
			connections[i].APIKey = maskString(conn.APIKey, 4, 4)
		}
	}

	// Clone combos
	combos := make([]domain.Combo, len(appCfg.Combos))
	copy(combos, appCfg.Combos)

	// Clone aliases
	aliases := make(domain.AliasMap)
	for k, v := range appCfg.ModelAliases {
		aliases[k] = v
	}

	// Clone API keys with optional masking
	apiKeys := make([]domain.APIKey, len(appCfg.APIKeys))
	for i, k := range appCfg.APIKeys {
		apiKeys[i] = k
		if cfg.maskTokens {
			apiKeys[i].Key = maskString(k.Key, 10, 4)
		}
	}

	// Clone settings
	settings := appCfg.Settings

	// Model registry (optional)
	var registry *domain.ModelRegistry
	if !cfg.skipRegistry && appCfg.ModelRegistry != nil {
		registry = appCfg.ModelRegistry
	}

	return &BackupData{
		Version:             CurrentBackupVersion,
		ExportedAt:          time.Now().UTC().Format(time.RFC3339),
		ProviderConnections: connections,
		Combos:              combos,
		ModelAliases:        aliases,
		APIKeys:             apiKeys,
		Settings:            settings,
		ModelRegistry:       registry,
	}, nil
}

// maskString truncates a string showing only first/last N chars.
func maskString(s string, first, last int) string {
	if len(s) <= first+last {
		return "***"
	}
	return s[:first] + "..." + s[len(s)-last:]
}

// IsMasked checks if a string looks like a masked value.
func IsMasked(s string) bool {
	return s == "" || strings.HasSuffix(s, "...") || s == "***"
}
