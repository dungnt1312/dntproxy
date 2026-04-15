package backup

import (
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// ImportResult reports what happened during import.
type ImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// Import merges or replaces backup data into the current config atomically.
// The entire operation happens under a single Save() call — no partial state.
func Import(store port.CredentialStore, backup *BackupData, mode string) (*ImportResult, error) {
	if err := ValidateImportMode(mode); err != nil {
		return nil, err
	}

	// Validate backup data (version, structure, values)
	if err := ValidateBackup(backup); err != nil {
		return nil, err
	}

	// Load current config
	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	result := &ImportResult{}

	// In replace mode, clear existing data
	if mode == "replace" {
		cfg.ProviderConnections = nil
		cfg.Combos = nil
		cfg.ModelAliases = nil
		cfg.APIKeys = nil
		// Keep settings as defaults, will be overwritten below
	}

	// Import connections
	importConnections(cfg, backup.ProviderConnections, mode, result)

	// Import combos
	importCombos(cfg, backup.Combos, mode, result)

	// Import aliases
	importAliases(cfg, backup.ModelAliases, mode, result)

	// Import API keys (skip masked ones)
	importAPIKeys(cfg, backup.APIKeys, mode, result)

	// Import settings (only non-zero/non-empty values)
	importSettings(cfg, backup.Settings)

	// Import model registry (if present in backup)
	if backup.ModelRegistry != nil && len(backup.ModelRegistry.Models) > 0 {
		if mode == "replace" {
			cfg.ModelRegistry = backup.ModelRegistry
		} else {
			// Merge: add/update models from backup
			if cfg.ModelRegistry == nil {
				cfg.ModelRegistry = &domain.ModelRegistry{
					Models: make(map[string]*domain.ModelDefinition),
				}
			}
			for key, model := range backup.ModelRegistry.Models {
				cfg.ModelRegistry.Models[key] = model
				result.Imported++
			}
		}
	}

	// Atomic save — single write operation
	if err := store.Save(cfg); err != nil {
		return nil, err
	}

	return result, nil
}

func importConnections(cfg *domain.AppConfig, conns []domain.ProviderConnection, mode string, result *ImportResult) {
	for _, conn := range conns {
		if conn.ID == "" {
			result.Skipped++
			continue
		}

		dc := conn
		dc.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		found := false
		for i, existing := range cfg.ProviderConnections {
			if existing.ID == conn.ID {
				cfg.ProviderConnections[i] = dc
				result.Imported++
				found = true
				break
			}
		}

		if !found {
			cfg.ProviderConnections = append(cfg.ProviderConnections, dc)
			result.Imported++
		}
	}
}

func importCombos(cfg *domain.AppConfig, combos []domain.Combo, mode string, result *ImportResult) {
	// Track names already in config to avoid duplicates
	existingNames := make(map[string]bool)
	for _, c := range cfg.Combos {
		existingNames[c.Name] = true
	}

	// Track names in current batch to catch intra-batch duplicates
	seenInBatch := make(map[string]bool)

	for _, combo := range combos {
		if combo.ID == "" || combo.Name == "" {
			result.Skipped++
			continue
		}

		// Skip if duplicate name within the backup batch itself
		if seenInBatch[combo.Name] {
			result.Skipped++
			continue
		}
		seenInBatch[combo.Name] = true

		dc := combo
		dc.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		found := false
		for i, existing := range cfg.Combos {
			if existing.ID == combo.ID || existing.Name == combo.Name {
				cfg.Combos[i] = dc
				result.Imported++
				found = true
				break
			}
		}

		if !found {
			cfg.Combos = append(cfg.Combos, dc)
			result.Imported++
		}
	}
}

func importAliases(cfg *domain.AppConfig, aliases domain.AliasMap, mode string, result *ImportResult) {
	if cfg.ModelAliases == nil {
		cfg.ModelAliases = make(domain.AliasMap)
	}

	if mode == "replace" {
		cfg.ModelAliases = make(domain.AliasMap)
	}

	for alias, model := range aliases {
		cfg.ModelAliases[alias] = model
		result.Imported++
	}
}

func importAPIKeys(cfg *domain.AppConfig, keys []domain.APIKey, mode string, result *ImportResult) {
	// Build lookup maps for existing keys
	existingByID := make(map[string]int)
	existingByKey := make(map[string]int)
	existingByName := make(map[string]int)

	for i, k := range cfg.APIKeys {
		existingByID[k.ID] = i
		existingByKey[k.Key] = i
		existingByName[k.Name] = i
	}

	for _, k := range keys {
		if k.ID == "" || k.Key == "" || IsMasked(k.Key) {
			result.Skipped++
			continue
		}

		dk := k

		// Check by ID first
		if idx, ok := existingByID[k.ID]; ok {
			cfg.APIKeys[idx] = dk
			result.Imported++
			continue
		}

		// Check by key value (catches regenerated keys with same value)
		if idx, ok := existingByKey[k.Key]; ok {
			cfg.APIKeys[idx] = dk
			result.Imported++
			continue
		}

		// Check by name (catches renamed keys)
		if idx, ok := existingByName[k.Name]; ok {
			cfg.APIKeys[idx] = dk
			result.Imported++
			continue
		}

		// New key
		cfg.APIKeys = append(cfg.APIKeys, dk)
		result.Imported++
	}
}

func importSettings(cfg *domain.AppConfig, src domain.Settings) {
	if src.Port > 0 {
		cfg.Settings.Port = src.Port
	}
	if src.ComboStrategy != "" {
		cfg.Settings.ComboStrategy = src.ComboStrategy
	}
	if src.RequireAPIKey {
		cfg.Settings.RequireAPIKey = true
	}
	if len(src.ComboStrategies) > 0 {
		cfg.Settings.ComboStrategies = src.ComboStrategies
	}
}
