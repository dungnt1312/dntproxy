package backup

import "github.com/dungnt/dntproxy/internal/domain"

const CurrentBackupVersion = "1.0"

// BackupData is the unified backup format shared by CLI and API.
type BackupData struct {
	Version             string                      `json:"version"`
	ExportedAt          string                      `json:"exportedAt"`
	ProviderConnections []domain.ProviderConnection `json:"providerConnections"`
	Combos              []domain.Combo              `json:"combos"`
	ModelAliases        domain.AliasMap             `json:"modelAliases"`
	APIKeys             []domain.APIKey             `json:"apiKeys"`
	Settings            domain.Settings             `json:"settings"`
	ModelRegistry       *domain.ModelRegistry       `json:"modelRegistry,omitempty"`
}
