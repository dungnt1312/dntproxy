package backup

import (
	"fmt"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// ImportConnectionMode defines how to handle existing connections during import.
type ImportConnectionMode string

const (
	// ImportModeAdd adds new connections, fails if ID already exists
	ImportModeAdd ImportConnectionMode = "add"
	// ImportModeReplace replaces existing connection with same ID, adds if not exists
	ImportModeReplace ImportConnectionMode = "replace"
	// ImportModeMerge adds only if ID doesn't exist, skips if exists
	ImportModeMerge ImportConnectionMode = "merge"
)

// ConnectionImportResult reports what happened during connection import.
type ConnectionImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Updated  int      `json:"updated"`
	Errors   []string `json:"errors,omitempty"`
}

// ImportConnection imports a single connection.
func ImportConnection(store port.CredentialStore, data *ConnectionExportData, mode ImportConnectionMode) (*ConnectionImportResult, error) {
	// Validate version
	if data.Version != CurrentBackupVersion {
		return nil, fmt.Errorf("unsupported backup version: %s (expected: %s)", data.Version, CurrentBackupVersion)
	}

	// Validate connection data
	if err := validateConnection(&data.Connection); err != nil {
		return nil, fmt.Errorf("invalid connection data: %w", err)
	}

	// If imported connection has no SupportedModels, fill with RecommendedModels for the provider
	if len(data.Connection.SupportedModels) == 0 {
		cfg := domain.GetProviderConfig(data.Connection.Provider)
		if len(cfg.RecommendedModels) > 0 {
			data.Connection.SupportedModels = cfg.RecommendedModels
		}
	}

	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	result := &ConnectionImportResult{}

	// Find existing connection
	existingIdx := -1
	for i := range cfg.ProviderConnections {
		if cfg.ProviderConnections[i].ID == data.Connection.ID {
			existingIdx = i
			break
		}
	}

	switch mode {
	case ImportModeAdd:
		if existingIdx >= 0 {
			return nil, fmt.Errorf("connection already exists: %s", data.Connection.ID)
		}
		cfg.ProviderConnections = append(cfg.ProviderConnections, data.Connection)
		result.Imported = 1

	case ImportModeReplace:
		if existingIdx >= 0 {
			cfg.ProviderConnections[existingIdx] = data.Connection
			result.Updated = 1
		} else {
			cfg.ProviderConnections = append(cfg.ProviderConnections, data.Connection)
			result.Imported = 1
		}

	case ImportModeMerge:
		if existingIdx >= 0 {
			result.Skipped = 1
		} else {
			cfg.ProviderConnections = append(cfg.ProviderConnections, data.Connection)
			result.Imported = 1
		}

	default:
		return nil, fmt.Errorf("unsupported import mode: %s", mode)
	}

	// Save atomically
	if err := store.Save(cfg); err != nil {
		return nil, err
	}

	return result, nil
}

// ImportConnections imports multiple connections from a backup.
func ImportConnections(store port.CredentialStore, data *BackupData, mode ImportConnectionMode) (*ConnectionImportResult, error) {
	// Validate version
	if data.Version != CurrentBackupVersion {
		return nil, fmt.Errorf("unsupported backup version: %s (expected: %s)", data.Version, CurrentBackupVersion)
	}

	if len(data.ProviderConnections) == 0 {
		return nil, fmt.Errorf("no connections to import")
	}

	// Validate all connections first
	for i := range data.ProviderConnections {
		if err := validateConnection(&data.ProviderConnections[i]); err != nil {
			return nil, fmt.Errorf("invalid connection at index %d: %w", i, err)
		}
	}

	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	result := &ConnectionImportResult{}

	// Build existing connection ID map
	existingMap := make(map[string]int)
	for i := range cfg.ProviderConnections {
		existingMap[cfg.ProviderConnections[i].ID] = i
	}

	// Process each connection
	for _, conn := range data.ProviderConnections {
		existingIdx, exists := existingMap[conn.ID]

		switch mode {
		case ImportModeAdd:
			if exists {
				result.Errors = append(result.Errors, fmt.Sprintf("connection already exists: %s", conn.ID))
				result.Skipped++
			} else {
				cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
				existingMap[conn.ID] = len(cfg.ProviderConnections) - 1
				result.Imported++
			}

		case ImportModeReplace:
			if exists {
				cfg.ProviderConnections[existingIdx] = conn
				result.Updated++
			} else {
				cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
				existingMap[conn.ID] = len(cfg.ProviderConnections) - 1
				result.Imported++
			}

		case ImportModeMerge:
			if exists {
				result.Skipped++
			} else {
				cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
				existingMap[conn.ID] = len(cfg.ProviderConnections) - 1
				result.Imported++
			}

		default:
			return nil, fmt.Errorf("unsupported import mode: %s", mode)
		}
	}

	// Save atomically
	if err := store.Save(cfg); err != nil {
		return nil, err
	}

	return result, nil
}

// validateConnection checks if a connection has required fields.
func validateConnection(conn *domain.ProviderConnection) error {
	if conn.ID == "" {
		return fmt.Errorf("connection ID is required")
	}
	if conn.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if conn.AuthType == "" {
		return fmt.Errorf("authType is required")
	}
	if conn.AuthType == "oauth" && conn.RefreshToken == "" {
		return fmt.Errorf("refreshToken is required for oauth connections")
	}
	if conn.AuthType == "apikey" && conn.APIKey == "" {
		return fmt.Errorf("apiKey is required for apikey connections")
	}
	return nil
}
