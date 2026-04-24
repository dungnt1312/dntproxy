package domain

// CLIToolID identifies a supported coding CLI tool.
type CLIToolID string

const (
	CLIToolClaudeCode CLIToolID = "claude"
	CLIToolOpenCode   CLIToolID = "opencode"
	CLIToolCodex      CLIToolID = "codex"
)

// CLIToolStatus describes the current user-level config state for a tool.
type CLIToolStatus struct {
	ID         CLIToolID `json:"id"`
	Name       string    `json:"name"`
	ConfigPath string    `json:"configPath"`
	Exists     bool      `json:"exists"`
	Writable   bool      `json:"writable"`
	Configured bool      `json:"configured"`
	LastBackup string    `json:"lastBackup,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// CLIToolsConfigRequest is used by preview and apply endpoints.
// Models is a map of role→model, e.g.:
//   - Claude: {"sonnet": "kr/...", "opus": "kr/...", "haiku": "kr/..."}
//   - OpenCode: {"model": "kr/..."}
//   - Codex: {"model": "kr/..."}
type CLIToolsConfigRequest struct {
	Endpoint string            `json:"endpoint"`
	APIKey   string            `json:"apiKey"`
	Model    string            `json:"model"`
	Models   map[string]string `json:"models,omitempty"`
	Tools    []CLIToolID       `json:"tools"`
}

// EffectiveModels returns Models if set, otherwise wraps Model as {"model": Model}.
func (r CLIToolsConfigRequest) EffectiveModels() map[string]string {
	if len(r.Models) > 0 {
		return r.Models
	}
	if r.Model != "" {
		return map[string]string{"model": r.Model}
	}
	return map[string]string{}
}

// CLIToolConfigPreview contains the file content that would be written.
type CLIToolConfigPreview struct {
	ToolID     CLIToolID         `json:"toolId"`
	ConfigPath string            `json:"configPath"`
	Content    string            `json:"content"`
	Aliases    map[string]string `json:"aliases,omitempty"`
}

// CLIToolsPreviewResponse is returned before any filesystem mutation.
type CLIToolsPreviewResponse struct {
	Endpoint string                 `json:"endpoint"`
	Models   map[string]string      `json:"models"`
	Aliases  map[string]string      `json:"aliases,omitempty"`
	Previews []CLIToolConfigPreview `json:"previews"`
}

// CLIToolApplyResult describes one tool write result.
type CLIToolApplyResult struct {
	ToolID     CLIToolID `json:"toolId"`
	ConfigPath string    `json:"configPath"`
	BackupPath string    `json:"backupPath,omitempty"`
	Applied    bool      `json:"applied"`
	Error      string    `json:"error,omitempty"`
}

// CLIToolsApplyResponse is returned after applying config files.
type CLIToolsApplyResponse struct {
	Endpoint string               `json:"endpoint"`
	Models   map[string]string    `json:"models"`
	Aliases  map[string]string    `json:"aliases,omitempty"`
	Results  []CLIToolApplyResult `json:"results"`
}

// CLIToolsRestoreRequest selects config files to restore from latest backup.
type CLIToolsRestoreRequest struct {
	Tools []CLIToolID `json:"tools"`
}

// CLIToolRestoreResult describes one restore result.
type CLIToolRestoreResult struct {
	ToolID     CLIToolID `json:"toolId"`
	ConfigPath string    `json:"configPath"`
	BackupPath string    `json:"backupPath,omitempty"`
	Restored   bool      `json:"restored"`
	Error      string    `json:"error,omitempty"`
}
