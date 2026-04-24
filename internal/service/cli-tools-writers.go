package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/pelletier/go-toml/v2"
)

func renderToolConfig(toolID domain.CLIToolID, current []byte, endpoint string, apiKey string, models map[string]string) (string, error) {
	endpoint = normalizeBaseURL(endpoint)
	switch toolID {
	case domain.CLIToolClaudeCode:
		return renderClaudeConfig(current, endpoint, apiKey, models)
	case domain.CLIToolOpenCode:
		return renderOpenCodeConfig(current, endpoint, apiKey, models)
	case domain.CLIToolCodex:
		return renderCodexConfig(current, endpoint, models)
	default:
		return "", fmt.Errorf("unsupported tool: %s", toolID)
	}
}

// renderClaudeConfig merges dntproxy settings into Claude Code's settings.json.
// Models map: {"sonnet": "...", "opus": "...", "haiku": "..."}
func renderClaudeConfig(current []byte, endpoint string, apiKey string, models map[string]string) (string, error) {
	cfg, err := decodeJSONObject(current)
	if err != nil {
		return "", err
	}
	env := getMap(cfg, "env")
	env["ANTHROPIC_BASE_URL"] = endpoint
	env["ANTHROPIC_AUTH_TOKEN"] = apiKey
	env["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"] = "1"

	if v := models["sonnet"]; v != "" {
		env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = v
	}
	if v := models["opus"]; v != "" {
		env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = v
	}
	if v := models["haiku"]; v != "" {
		env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = v
	}
	// Remove legacy single-model key if present
	delete(env, "ANTHROPIC_MODEL")

	cfg["env"] = env
	return encodeJSON(cfg)
}

// renderOpenCodeConfig merges dntproxy settings into OpenCode's opencode.json.
// Uses "dntproxy" provider with @ai-sdk/openai-compatible.
// Models map: {"model": "kr/claude-sonnet-4.5", "extra_0": "kr/claude-opus-4.6", ...}
func renderOpenCodeConfig(current []byte, endpoint string, apiKey string, models map[string]string) (string, error) {
	cfg, err := decodeJSONObject(current)
	if err != nil {
		return "", err
	}
	cfg["$schema"] = "https://opencode.ai/config.json"

	mainModel := models["model"]
	if mainModel != "" {
		cfg["model"] = "dntproxy/" + mainModel
	}

	provider := getMap(cfg, "provider")
	dntproxy := getMap(provider, "dntproxy")
	dntproxy["npm"] = "@ai-sdk/openai-compatible"
	options := getMap(dntproxy, "options")
	options["baseURL"] = endpoint
	options["apiKey"] = apiKey
	dntproxy["options"] = options

	// Build models map: main model + extras
	modelsMap := getMap(dntproxy, "models")
	for _, modelID := range collectModelIDs(models) {
		if _, exists := modelsMap[modelID]; !exists {
			modelsMap[modelID] = map[string]any{"name": modelID}
		}
	}
	dntproxy["models"] = modelsMap

	provider["dntproxy"] = dntproxy
	cfg["provider"] = provider

	return encodeJSON(cfg)
}

func renderCodexConfig(current []byte, endpoint string, models map[string]string) (string, error) {
	cfg := map[string]any{}
	if len(bytes.TrimSpace(current)) > 0 {
		if err := toml.Unmarshal(current, &cfg); err != nil {
			return "", err
		}
	}
	if v := models["model"]; v != "" {
		cfg["model"] = v
	}
	cfg["model_provider"] = "dntproxy"

	providers := getMap(cfg, "model_providers")
	dntproxy := getMap(providers, "dntproxy")
	dntproxy["name"] = "dntproxy"
	dntproxy["base_url"] = endpoint
	dntproxy["env_key"] = "DNTPROXY_API_KEY"
	dntproxy["wire_api"] = "chat"
	providers["dntproxy"] = dntproxy
	cfg["model_providers"] = providers

	data, err := toml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func isToolConfigured(toolID domain.CLIToolID, path string) (bool, error) {
	data, err := readConfigOrEmpty(path)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return false, err
	}
	switch toolID {
	case domain.CLIToolClaudeCode:
		cfg, err := decodeJSONObject(data)
		if err != nil {
			return false, err
		}
		env, ok := cfg["env"].(map[string]any)
		if !ok {
			return false, nil
		}
		return valueContainsDntproxy(env["ANTHROPIC_BASE_URL"]), nil
	case domain.CLIToolOpenCode:
		cfg, err := decodeJSONObject(data)
		if err != nil {
			return false, err
		}
		provider, _ := cfg["provider"].(map[string]any)
		// Check new dntproxy provider
		if dntproxy, ok := provider["dntproxy"].(map[string]any); ok {
			options, _ := dntproxy["options"].(map[string]any)
			if valueContainsDntproxy(options["baseURL"]) {
				return true, nil
			}
		}
		// Check legacy openai provider
		openai, _ := provider["openai"].(map[string]any)
		options, _ := openai["options"].(map[string]any)
		return valueContainsDntproxy(options["baseURL"]), nil
	case domain.CLIToolCodex:
		cfg := map[string]any{}
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return false, err
		}
		return cfg["model_provider"] == "dntproxy", nil
	default:
		return false, nil
	}
}

func decodeJSONObject(data []byte) (map[string]any, error) {
	cfg := map[string]any{}
	if len(bytes.TrimSpace(data)) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func encodeJSON(cfg map[string]any) (string, error) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

func getMap(parent map[string]any, key string) map[string]any {
	if existing, ok := parent[key].(map[string]any); ok {
		return existing
	}
	next := map[string]any{}
	parent[key] = next
	return next
}

func valueContainsDntproxy(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	text = strings.ToLower(text)
	return strings.Contains(text, "dntproxy") || strings.Contains(text, "127.0.0.1") || strings.Contains(text, "localhost")
}

// collectModelIDs extracts all non-empty model IDs from the models map.
func collectModelIDs(models map[string]string) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, id := range models {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}
