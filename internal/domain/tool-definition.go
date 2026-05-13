package domain

// ToolID identifies a supported AI coding tool.
type ToolID string

const (
	ToolClaudeCode ToolID = "claude-code"
	ToolCursor     ToolID = "cursor"
	ToolWindsurf   ToolID = "windsurf"
	ToolOpenCode   ToolID = "opencode"
	ToolCline      ToolID = "cline"
	ToolContinue   ToolID = "continue"
	ToolGeminiCLI  ToolID = "gemini-cli"
)

// ToolDefinition describes how to detect and configure an AI coding tool.
type ToolDefinition struct {
	ID          ToolID
	Name        string
	Description string
	// ConfigPaths per OS (key: "windows", "linux", "darwin")
	// Paths may contain ~ for home dir expansion.
	ConfigPaths map[string]string
	// ConfigFormat describes the file format ("json", "env").
	ConfigFormat string
	// DetectPaths are paths/binaries to check for tool presence.
	// If any exist, the tool is considered installed.
	DetectPaths map[string][]string
	// ProxyFields maps the config key to set for base URL and API key.
	ProxyURLField string
	ProxyKeyField string
}

// ToolRegistry holds all supported tool definitions.
var ToolRegistry = map[ToolID]ToolDefinition{
	ToolClaudeCode: {
		ID:          ToolClaudeCode,
		Name:        "Claude Code",
		Description: "Anthropic Claude CLI tool",
		ConfigPaths: map[string]string{
			"windows": `~\.claude.json`,
			"linux":   `~/.claude.json`,
			"darwin":  `~/.claude.json`,
		},
		ConfigFormat:  "json",
		ProxyURLField: "apiUrl",
		ProxyKeyField: "apiKey",
		DetectPaths: map[string][]string{
			"windows": {`~\.claude`, `~\AppData\Roaming\npm\claude.cmd`},
			"linux":   {`~/.claude`, `/usr/local/bin/claude`},
			"darwin":  {`~/.claude`, `/usr/local/bin/claude`},
		},
	},
	ToolCursor: {
		ID:          ToolCursor,
		Name:        "Cursor",
		Description: "Cursor AI code editor",
		ConfigPaths: map[string]string{
			"windows": `~\.cursor\mcp.json`,
			"linux":   `~/.cursor/mcp.json`,
			"darwin":  `~/.cursor/mcp.json`,
		},
		ConfigFormat:  "json",
		ProxyURLField: "openaiBaseUrl",
		ProxyKeyField: "openaiApiKey",
		DetectPaths: map[string][]string{
			"windows": {`~\.cursor`},
			"linux":   {`~/.cursor`},
			"darwin":  {`~/.cursor`},
		},
	},
	ToolWindsurf: {
		ID:          ToolWindsurf,
		Name:        "Windsurf",
		Description: "Codeium Windsurf AI editor",
		ConfigPaths: map[string]string{
			"windows": `~\.codeium\windsurf\config.json`,
			"linux":   `~/.codeium/windsurf/config.json`,
			"darwin":  `~/.codeium/windsurf/config.json`,
		},
		ConfigFormat:  "json",
		ProxyURLField: "apiBaseUrl",
		ProxyKeyField: "apiKey",
		DetectPaths: map[string][]string{
			"windows": {`~\.codeium\windsurf`},
			"linux":   {`~/.codeium/windsurf`},
			"darwin":  {`~/.codeium/windsurf`},
		},
	},
	ToolOpenCode: {
		ID:          ToolOpenCode,
		Name:        "OpenCode",
		Description: "OpenCode AI coding CLI",
		ConfigPaths: map[string]string{
			"windows": `~\.config\opencode\config.json`,
			"linux":   `~/.config/opencode/config.json`,
			"darwin":  `~/.config/opencode/config.json`,
		},
		ConfigFormat:  "json",
		ProxyURLField: "provider.baseURL",
		ProxyKeyField: "provider.apiKey",
		DetectPaths: map[string][]string{
			"windows": {`~\.config\opencode`},
			"linux":   {`~/.config/opencode`},
			"darwin":  {`~/.config/opencode`},
		},
	},
	ToolCline: {
		ID:          ToolCline,
		Name:        "Cline",
		Description: "Cline VS Code extension",
		ConfigPaths: map[string]string{
			"windows": `~\.vscode\extensions\cline\config.json`,
			"linux":   `~/.vscode/extensions/cline/config.json`,
			"darwin":  `~/.vscode/extensions/cline/config.json`,
		},
		ConfigFormat:  "json",
		ProxyURLField: "apiBaseUrl",
		ProxyKeyField: "apiKey",
		DetectPaths: map[string][]string{
			"windows": {`~\.vscode\extensions\cline`},
			"linux":   {`~/.vscode/extensions/cline`},
			"darwin":  {`~/.vscode/extensions/cline`},
		},
	},
	ToolContinue: {
		ID:          ToolContinue,
		Name:        "Continue",
		Description: "Continue VS Code extension",
		ConfigPaths: map[string]string{
			"windows": `~\.continue\config.json`,
			"linux":   `~/.continue/config.json`,
			"darwin":  `~/.continue/config.json`,
		},
		ConfigFormat:  "json",
		ProxyURLField: "models[].apiBase",
		ProxyKeyField: "models[].apiKey",
		DetectPaths: map[string][]string{
			"windows": {`~\.continue`},
			"linux":   {`~/.continue`},
			"darwin":  {`~/.continue`},
		},
	},
	ToolGeminiCLI: {
		ID:          ToolGeminiCLI,
		Name:        "Gemini CLI",
		Description: "Google Gemini CLI tool",
		ConfigPaths: map[string]string{
			"windows": `~\.gemini\settings.json`,
			"linux":   `~/.gemini/settings.json`,
			"darwin":  `~/.gemini/settings.json`,
		},
		ConfigFormat:  "json",
		ProxyURLField: "apiBaseUrl",
		ProxyKeyField: "apiKey",
		DetectPaths: map[string][]string{
			"windows": {`~\.gemini`},
			"linux":   {`~/.gemini`},
			"darwin":  {`~/.gemini`},
		},
	},
}

// ListToolIDs returns all supported tool IDs sorted alphabetically.
func ListToolIDs() []ToolID {
	ids := make([]ToolID, 0, len(ToolRegistry))
	for id := range ToolRegistry {
		ids = append(ids, id)
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}

// GetToolDefinition returns the definition for a tool ID, or nil if not found.
func GetToolDefinition(id ToolID) *ToolDefinition {
	if def, ok := ToolRegistry[id]; ok {
		return &def
	}
	return nil
}
