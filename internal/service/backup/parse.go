package backup

import (
	"bytes"
	"encoding/json"

	"github.com/dungnt/dntproxy/internal/domain"
)

// ParseBackup decodes a backup payload. Settings are decoded leniently so
// backups produced by other tools (e.g. 9router object-shaped
// comboStrategies) can be imported; every other field keeps strict decoding.
func ParseBackup(body []byte) (*BackupData, error) {
	var raw struct {
		Version             string                      `json:"version"`
		ExportedAt          string                      `json:"exportedAt"`
		ProviderConnections []domain.ProviderConnection `json:"providerConnections"`
		Combos              []domain.Combo              `json:"combos"`
		ModelAliases        domain.AliasMap             `json:"modelAliases"`
		APIKeys             []domain.APIKey             `json:"apiKeys"`
		Settings            json.RawMessage             `json:"settings"`
		ModelRegistry       *domain.ModelRegistry       `json:"modelRegistry,omitempty"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	settings := domain.Settings{}
	if len(raw.Settings) > 0 && !bytes.Equal(bytes.TrimSpace(raw.Settings), []byte("null")) {
		if err := unmarshalSettingsTolerant(raw.Settings, &settings); err != nil {
			return nil, err
		}
	}

	return &BackupData{
		Version:             raw.Version,
		ExportedAt:          raw.ExportedAt,
		ProviderConnections: raw.ProviderConnections,
		Combos:              raw.Combos,
		ModelAliases:        raw.ModelAliases,
		APIKeys:             raw.APIKeys,
		Settings:            settings,
		ModelRegistry:       raw.ModelRegistry,
	}, nil
}

// unmarshalSettingsTolerant decodes settings but accepts object-shaped
// comboStrategies entries ({"strategy": ...} / {"type": ...}) and drops
// unrecognized values instead of failing the whole backup.
func unmarshalSettingsTolerant(data []byte, out *domain.Settings) error {
	type plain domain.Settings
	var raw struct {
		plain
		ComboStrategies json.RawMessage `json:"comboStrategies"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*out = domain.Settings(raw.plain)

	if len(raw.ComboStrategies) == 0 || bytes.Equal(bytes.TrimSpace(raw.ComboStrategies), []byte("null")) {
		return nil
	}
	entries := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw.ComboStrategies, &entries); err != nil {
		return err
	}

	normalized := make(map[string]string, len(entries))
	for name, value := range entries {
		var strategy string
		if err := json.Unmarshal(value, &strategy); err == nil {
			normalized[name] = strategy
			continue
		}
		var legacy struct {
			Strategy string `json:"strategy"`
			Type     string `json:"type"`
		}
		if err := json.Unmarshal(value, &legacy); err == nil {
			switch {
			case legacy.Strategy != "":
				normalized[name] = legacy.Strategy
			case legacy.Type != "":
				normalized[name] = legacy.Type
			}
		}
	}
	out.ComboStrategies = normalized
	return nil
}
