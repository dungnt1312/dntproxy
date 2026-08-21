package backup

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
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

	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	normalizeImportedConnection(&data.Connection, &cfg.Settings)

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
			if err := refuseCrossTenantReplace(cfg.ProviderConnections[existingIdx], data.Connection); err != nil {
				return nil, err
			}
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

// NineRouterImportResult carries the converted connections and the entries
// that were skipped because dntproxy has no compatible target for them.
type NineRouterImportResult struct {
	Data    *BackupData
	Skipped []string
}

// supported9RouterMappings maps 9router provider IDs to dntproxy provider IDs.
var supported9RouterMappings = map[string]string{
	"codex":       "openai",
	"glm":         "glm",
	"xai":         "xai",
	"kiro":        "kiro",
	"commandcode": "commandcode",
}

// Convert9RouterBackup converts the supported 9router backup shape into a
// dntproxy connection import. Unsupported entries are skipped with a reason
// instead of failing the whole backup.
func Convert9RouterBackup(data *BackupData) (*NineRouterImportResult, error) {
	if data.Version != "" {
		return nil, fmt.Errorf("9router backup must not include a version")
	}
	if len(data.ProviderConnections) == 0 {
		return nil, fmt.Errorf("no connections to import")
	}

	result := &NineRouterImportResult{
		Data: &BackupData{
			Version:    CurrentBackupVersion,
			ExportedAt: data.ExportedAt,
		},
	}
	for _, source := range data.ProviderConnections {
		target, ok := supported9RouterMappings[source.Provider]
		if !ok {
			result.Skipped = append(result.Skipped, fmt.Sprintf("connection %q skipped: unsupported provider %q", source.Name, source.Provider))
			continue
		}
		conn, err := convert9RouterConnection(target, source)
		if err != nil {
			result.Skipped = append(result.Skipped, fmt.Sprintf("connection %q skipped: %v", source.Name, err))
			continue
		}
		result.Data.ProviderConnections = append(result.Data.ProviderConnections, conn)
	}
	if len(result.Data.ProviderConnections) == 0 {
		return nil, fmt.Errorf("no importable connections in 9router backup")
	}
	return result, nil
}

func convert9RouterConnection(target string, source domain.ProviderConnection) (domain.ProviderConnection, error) {
	conn := domain.ProviderConnection{
		ID:              source.ID,
		Provider:        target,
		AuthType:        source.AuthType,
		Name:            source.Name,
		Priority:        source.Priority,
		Weight:          source.Weight,
		IsActive:        source.IsActive,
		AccessToken:     source.AccessToken,
		RefreshToken:    source.RefreshToken,
		ExpiresAt:       source.ExpiresAt,
		ExpiresIn:       source.ExpiresIn,
		Email:           source.Email,
		SupportedModels: source.SupportedModels,
		CreatedAt:       source.CreatedAt,
	}

	switch target {
	case "openai", "xai":
		if source.AuthType != "oauth" {
			return domain.ProviderConnection{}, fmt.Errorf("%s requires oauth, got %s", source.Provider, source.AuthType)
		}
		providerData := map[string]interface{}{"authMethod": "oauth"}
		if target == "xai" {
			providerData["authMethod"] = "xai-oauth"
		}
		if idToken, ok := source.ProviderSpecificData["idToken"].(string); ok && idToken != "" {
			providerData["idToken"] = idToken
		}
		conn.ProviderSpecificData = providerData

	case "kiro":
		if source.AuthType != "oauth" {
			return domain.ProviderConnection{}, fmt.Errorf("kiro requires oauth, got %s", source.AuthType)
		}
		providerData := map[string]interface{}{}
		for _, key := range []string{"authMethod", "clientId", "clientSecret", "provider", "region", "profileArn"} {
			if value, ok := source.ProviderSpecificData[key]; ok {
				providerData[key] = value
			}
		}
		conn.ProviderSpecificData = providerData

	case "glm", "commandcode":
		if source.AuthType != "apikey" {
			return domain.ProviderConnection{}, fmt.Errorf("%s requires apikey, got %s", source.Provider, source.AuthType)
		}
		conn.APIKey = source.APIKey

	default:
		return domain.ProviderConnection{}, fmt.Errorf("unsupported provider %q", source.Provider)
	}
	return conn, nil
}

// Parse9RouterBackup parses and converts a version-less 9router backup.
func Parse9RouterBackup(body []byte) (*NineRouterImportResult, error) {
	var raw struct {
		ProviderConnections []json.RawMessage `json:"providerConnections"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	data := &BackupData{ProviderConnections: make([]domain.ProviderConnection, 0, len(raw.ProviderConnections))}
	for i, item := range raw.ProviderConnections {
		var conn domain.ProviderConnection
		var metadata struct {
			IDToken string `json:"idToken"`
		}
		if err := json.Unmarshal(item, &conn); err != nil {
			return nil, fmt.Errorf("invalid 9router connection at index %d: %w", i, err)
		}
		if err := json.Unmarshal(item, &metadata); err != nil {
			return nil, fmt.Errorf("invalid 9router connection metadata at index %d: %w", i, err)
		}
		if conn.ProviderSpecificData == nil {
			conn.ProviderSpecificData = map[string]interface{}{}
		}
		if metadata.IDToken != "" {
			conn.ProviderSpecificData["idToken"] = metadata.IDToken
		}
		data.ProviderConnections = append(data.ProviderConnections, conn)
	}
	return Convert9RouterBackup(data)
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

	// Validate all connections first, including duplicate IDs in the uploaded file.
	seenIDs := make(map[string]struct{}, len(data.ProviderConnections))
	for i := range data.ProviderConnections {
		conn := &data.ProviderConnections[i]
		if err := validateConnection(conn); err != nil {
			return nil, fmt.Errorf("invalid connection at index %d: %w", i, err)
		}
		if _, exists := seenIDs[conn.ID]; exists {
			return nil, fmt.Errorf("duplicate connection ID in import file: %s", conn.ID)
		}
		seenIDs[conn.ID] = struct{}{}
	}

	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	for i := range data.ProviderConnections {
		normalizeImportedConnection(&data.ProviderConnections[i], &cfg.Settings)
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
				if err := refuseCrossTenantReplace(cfg.ProviderConnections[existingIdx], conn); err != nil {
					result.Errors = append(result.Errors, err.Error())
					result.Skipped++
					continue
				}
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
	if conn.BaseURL != "" {
		if err := shared.ValidateOutboundURL(conn.BaseURL, shared.AllowPrivateOutbound(conn.TenantID)); err != nil {
			return fmt.Errorf("invalid baseUrl: %w", err)
		}
	}
	return nil
}

func normalizeImportedConnection(conn *domain.ProviderConnection, settings *domain.Settings) {
	if len(conn.SupportedModels) == 0 {
		conn.SupportedModels = domain.GetDefaultConnectionModels(settings, conn.Provider)
	}
	conn.TestStatus = ""
	conn.LastError = ""
	conn.LastErrorAt = ""
	conn.RateLimitedUntil = ""
	conn.BackoffLevel = 0
	conn.ConsecutiveUseCount = 0
	conn.ModelLocks = nil
	conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func refuseCrossTenantReplace(existing, incoming domain.ProviderConnection) error {
	if existing.TenantID != "" && existing.TenantID != incoming.TenantID {
		return fmt.Errorf("cannot replace connection %s owned by another tenant", existing.ID)
	}
	return nil
}
