package backup

import (
	"fmt"
	"strings"
)

// ValidateBackup checks backup data for correctness before import.
// Returns first validation error found, or nil if valid.
func ValidateBackup(b *BackupData) error {
	if b == nil {
		return fmt.Errorf("backup data is nil")
	}

	// Version check: if missing, assume current version (backward compat)
	ver := b.Version
	if ver == "" {
		ver = CurrentBackupVersion
	}
	if ver != CurrentBackupVersion {
		return fmt.Errorf("unsupported backup version %q, expected %q", ver, CurrentBackupVersion)
	}

	// Validate connections
	for i, conn := range b.ProviderConnections {
		if conn.ID == "" {
			return fmt.Errorf("providerConnections[%d]: missing id", i)
		}
		if conn.Provider == "" {
			return fmt.Errorf("providerConnections[%d]: missing provider", i)
		}
		if conn.AuthType != "" && conn.AuthType != "oauth" && conn.AuthType != "apikey" {
			return fmt.Errorf("providerConnections[%d]: invalid authType %q (must be 'oauth' or 'apikey')", i, conn.AuthType)
		}
		if conn.Weight < 0 {
			return fmt.Errorf("providerConnections[%d]: negative weight", i)
		}
	}

	// Validate combos
	seenComboNames := make(map[string]int) // name → index
	for i, combo := range b.Combos {
		if combo.ID == "" {
			return fmt.Errorf("combos[%d]: missing id", i)
		}
		if combo.Name == "" {
			return fmt.Errorf("combos[%d]: missing name", i)
		}
		if len(combo.Models) == 0 {
			return fmt.Errorf("combos[%d]: no models", i)
		}
		if prev, ok := seenComboNames[combo.Name]; ok {
			return fmt.Errorf("combos[%d]: duplicate name %q (also at index %d)", i, combo.Name, prev)
		}
		seenComboNames[combo.Name] = i
	}

	// Validate API keys
	seenKeyValues := make(map[string]int) // key → index
	seenKeyNames := make(map[string]int)  // name → index
	for i, k := range b.APIKeys {
		if k.ID == "" {
			return fmt.Errorf("apiKeys[%d]: missing id", i)
		}
		if k.Name == "" {
			return fmt.Errorf("apiKeys[%d]: missing name", i)
		}
		// Skip validation for masked keys (they end with "...")
		if k.Key != "" && !strings.HasSuffix(k.Key, "...") {
			if prev, ok := seenKeyValues[k.Key]; ok {
				return fmt.Errorf("apiKeys[%d]: duplicate key value (also at index %d)", i, prev)
			}
			seenKeyValues[k.Key] = i
		}
		if prev, ok := seenKeyNames[k.Name]; ok {
			return fmt.Errorf("apiKeys[%d]: duplicate name %q (also at index %d)", i, k.Name, prev)
		}
		seenKeyNames[k.Name] = i
	}

	// Validate settings
	if b.Settings.Port < 0 || b.Settings.Port > 65535 {
		return fmt.Errorf("settings: invalid port %d", b.Settings.Port)
	}
	if b.Settings.ComboStrategy != "" && b.Settings.ComboStrategy != "fallback" && b.Settings.ComboStrategy != "round-robin" {
		return fmt.Errorf("settings: invalid comboStrategy %q", b.Settings.ComboStrategy)
	}

	// Validate model registry (if present)
	if b.ModelRegistry != nil && b.ModelRegistry.Models != nil {
		for key, model := range b.ModelRegistry.Models {
			if model == nil {
				return fmt.Errorf("modelRegistry[%s]: nil definition", key)
			}
			if model.Provider == "" {
				return fmt.Errorf("modelRegistry[%s]: missing provider", key)
			}
		}
	}

	return nil
}

// ValidateImportMode checks if the import mode is valid.
func ValidateImportMode(mode string) error {
	switch mode {
	case "merge", "replace":
		return nil
	default:
		return fmt.Errorf("invalid import mode %q (must be 'merge' or 'replace')", mode)
	}
}
