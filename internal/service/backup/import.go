package backup

import (
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

	// Load current config (to preserve structure)
	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	// Override everything
	cfg.ProviderConnections = backup.ProviderConnections
	cfg.Combos = backup.Combos
	cfg.ModelAliases = backup.ModelAliases
	cfg.APIKeys = backup.APIKeys
	cfg.Settings = backup.Settings
	cfg.ModelRegistry = backup.ModelRegistry

	// Atomic save — single write operation
	if err := store.Save(cfg); err != nil {
		return nil, err
	}

	// Count total items imported
	total := len(backup.ProviderConnections) + 
		len(backup.Combos) + 
		len(backup.ModelAliases) + 
		len(backup.APIKeys)
	
	if backup.ModelRegistry != nil {
		total += len(backup.ModelRegistry.Models)
	}

	return &ImportResult{Imported: total}, nil
}
