package backup

import (
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// ImportResult reports what happened during import.
type ImportResult struct {
	Imported int `json:"imported"`
}

// Import replaces all config with backup data atomically.
// The entire operation happens under a single Save() call — no partial state.
func Import(store port.CredentialStore, backup *BackupData) (*ImportResult, error) {
	// Validate backup data (version, structure, values)
	if err := ValidateBackup(backup); err != nil {
		return nil, err
	}

	var total int
	if err := store.Update(func(cfg *domain.AppConfig) {
		cfg.ProviderConnections = backup.ProviderConnections
		cfg.Combos = backup.Combos
		cfg.ModelAliases = backup.ModelAliases
		cfg.APIKeys = backup.APIKeys
		applyImportedSettings(&cfg.Settings, backup.Settings)
		if backup.ModelRegistry != nil {
			cfg.ModelRegistry = backup.ModelRegistry
		}

		total = len(backup.ProviderConnections) +
			len(backup.Combos) +
			len(backup.ModelAliases) +
			len(backup.APIKeys)
		if backup.ModelRegistry != nil {
			total += len(backup.ModelRegistry.Models)
		}
	}); err != nil {
		return nil, err
	}

	return &ImportResult{Imported: total}, nil
}

// applyImportedSettings copies routing/compression/logging flags and never
// takes listen port, requireApiKey, or tunnel identity from the backup.
func applyImportedSettings(dst *domain.Settings, src domain.Settings) {
	if dst == nil {
		return
	}
	if src.ComboStrategy != "" {
		dst.ComboStrategy = src.ComboStrategy
	}
	if src.ConnectionStrategy != "" {
		dst.ConnectionStrategy = src.ConnectionStrategy
	}
	if src.ConnectionStrategies != nil {
		dst.ConnectionStrategies = src.ConnectionStrategies
	}
	if src.ComboStrategies != nil {
		dst.ComboStrategies = src.ComboStrategies
	}
	dst.Compression = src.Compression
	dst.Compression.Normalize()
	dst.LogBodies = src.LogBodies
	if src.DefaultModels != nil {
		dst.DefaultModels = src.DefaultModels
	}
	dst.DisableImageGeneration = src.DisableImageGeneration
}
