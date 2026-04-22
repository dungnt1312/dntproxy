package backup

import (
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// Export loads config from store and returns a BackupData snapshot.
// Exports everything without masking or filtering.
func Export(store port.CredentialStore) (*BackupData, error) {
	appCfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	// Clone connections
	connections := make([]domain.ProviderConnection, len(appCfg.ProviderConnections))
	copy(connections, appCfg.ProviderConnections)

	// Clone combos
	combos := make([]domain.Combo, len(appCfg.Combos))
	copy(combos, appCfg.Combos)

	// Clone aliases
	aliases := make(domain.AliasMap)
	for k, v := range appCfg.ModelAliases {
		aliases[k] = v
	}

	// Clone API keys
	apiKeys := make([]domain.APIKey, len(appCfg.APIKeys))
	copy(apiKeys, appCfg.APIKeys)

	// Clone settings
	settings := appCfg.Settings

	// Clone model registry
	var registry *domain.ModelRegistry
	if appCfg.ModelRegistry != nil {
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
