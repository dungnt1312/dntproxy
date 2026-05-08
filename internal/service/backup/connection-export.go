package backup

import (
	"fmt"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// ConnectionExportData represents a single connection export format.
type ConnectionExportData struct {
	Version    string                    `json:"version"`
	ExportedAt string                    `json:"exportedAt"`
	Connection domain.ProviderConnection `json:"connection"`
}

// ExportConnection exports a single connection by ID.
func ExportConnection(store port.CredentialStore, connectionID string) (*ConnectionExportData, error) {
	appCfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	// Find the connection
	var found *domain.ProviderConnection
	for i := range appCfg.ProviderConnections {
		if appCfg.ProviderConnections[i].ID == connectionID {
			found = &appCfg.ProviderConnections[i]
			break
		}
	}

	if found == nil {
		return nil, fmt.Errorf("connection not found: %s", connectionID)
	}

	// Clone the connection to avoid modifying the original
	connection := *found

	return &ConnectionExportData{
		Version:    CurrentBackupVersion,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Connection: connection,
	}, nil
}

// ExportConnections exports multiple connections by IDs.
// If connectionIDs is empty, exports all connections.
func ExportConnections(store port.CredentialStore, connectionIDs []string) (*BackupData, error) {
	appCfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	var connections []domain.ProviderConnection

	// If no IDs specified, export all
	if len(connectionIDs) == 0 {
		connections = make([]domain.ProviderConnection, len(appCfg.ProviderConnections))
		copy(connections, appCfg.ProviderConnections)
	} else {
		// Export only specified connections
		idMap := make(map[string]bool)
		for _, id := range connectionIDs {
			idMap[id] = true
		}

		for i := range appCfg.ProviderConnections {
			if idMap[appCfg.ProviderConnections[i].ID] {
				connections = append(connections, appCfg.ProviderConnections[i])
			}
		}

		// Check if all requested IDs were found
		if len(connections) != len(connectionIDs) {
			return nil, fmt.Errorf("some connections not found (requested: %d, found: %d)", len(connectionIDs), len(connections))
		}
	}

	return &BackupData{
		Version:             CurrentBackupVersion,
		ExportedAt:          time.Now().UTC().Format(time.RFC3339),
		ProviderConnections: connections,
		Combos:              []domain.Combo{},
		ModelAliases:        domain.AliasMap{},
		APIKeys:             []domain.APIKey{},
		Settings:            domain.Settings{},
		ModelRegistry:       nil,
	}, nil
}
