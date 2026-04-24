package service

import (
	"fmt"

	"github.com/dungnt/dntproxy/internal/domain"
)

func renderToolPreview(toolID domain.CLIToolID, endpoint string, apiKey string, models map[string]string) (string, error) {
	endpoint = normalizeBaseURL(endpoint)
	switch toolID {
	case domain.CLIToolClaudeCode:
		return renderClaudePreview(endpoint, apiKey, models), nil
	case domain.CLIToolOpenCode:
		return renderOpenCodePreview(endpoint, apiKey, models), nil
	case domain.CLIToolCodex:
		return renderCodexPreview(endpoint, apiKey, models), nil
	default:
		return "", fmt.Errorf("unsupported tool: %s", toolID)
	}
}

func renderClaudePreview(endpoint, apiKey string, models map[string]string) string {
	sonnet := models["sonnet"]
	opus := models["opus"]
	haiku := models["haiku"]
	return fmt.Sprintf(`// ~/.claude/settings.json
// Env keys merged into existing settings.
{
  "env": {
    "ANTHROPIC_BASE_URL": %q,
    "ANTHROPIC_AUTH_TOKEN": %q,
    "ANTHROPIC_DEFAULT_SONNET_MODEL": %q,
    "ANTHROPIC_DEFAULT_OPUS_MODEL": %q,
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": %q,
    "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1"
  }
}
`, endpoint, apiKey, sonnet, opus, haiku)
}

func renderOpenCodePreview(endpoint, apiKey string, models map[string]string) string {
	mainModel := models["model"]
	return fmt.Sprintf(`// ~/.config/opencode/opencode.json
// Provider "dntproxy" with @ai-sdk/openai-compatible.
{
  "model": %q,
  "provider": {
    "dntproxy": {
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "baseURL": %q,
        "apiKey": %q
      }
    }
  }
}
`, "dntproxy/"+mainModel, endpoint, apiKey)
}

func renderCodexPreview(endpoint, apiKey string, models map[string]string) string {
	model := models["model"]
	return fmt.Sprintf(`# ~/.codex/config.toml
# Only model/model_provider and model_providers.dntproxy will be merged.
model = %q
model_provider = "dntproxy"

[model_providers.dntproxy]
name = "dntproxy"
base_url = %q
env_key = "DNTPROXY_API_KEY"
wire_api = "chat"

# Manual shell env:
# export DNTPROXY_API_KEY=%q
`, model, endpoint, apiKey)
}
