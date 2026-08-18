package domain

// BuiltinPresets contains pre-configured profiles for common use-cases.
// Users can create profiles from these presets via CLI or API.
var BuiltinPresets = map[string]Profile{
	"claude-kiro": {
		Name:        "claude-kiro",
		Description: "Route Claude models through Kiro (AWS CodeWhisperer) — free tier",
		Aliases: AliasMap{
			// Standard Claude model names used by Claude CLI
			"claude-sonnet": "kr/claude-sonnet-5",
			"claude-opus":   "kr/claude-opus-5",
			"claude-haiku":  "kr/claude-haiku-4.5",
			// Versioned model names that Claude CLI may send
			"claude-sonnet-4-5-20250514": "kr/claude-sonnet-4.5",
			"claude-opus-4-5-20250514":   "kr/claude-opus-4.5",
			"claude-haiku-4-5-20251001":  "kr/claude-haiku-4.5",
		},
	},
	"claude-anthropic": {
		Name:        "claude-anthropic",
		Description: "Route Claude models directly to Anthropic API",
		Aliases: AliasMap{
			"claude-sonnet": "anthropic/claude-sonnet-5",
			"claude-opus":   "anthropic/claude-opus-5",
			"claude-haiku":  "anthropic/claude-haiku-4-5",
		},
	},
	"claude-openai": {
		Name:        "claude-openai",
		Description: "Route Claude model names to OpenAI equivalents",
		Aliases: AliasMap{
			"claude-sonnet": "oai/gpt-5.6-terra",
			"claude-opus":   "oai/gpt-5.6-sol",
			"claude-haiku":  "oai/gpt-5.6-luna",
		},
	},
}

// ListPresetNames returns all available preset names.
func ListPresetNames() []string {
	names := make([]string, 0, len(BuiltinPresets))
	for name := range BuiltinPresets {
		names = append(names, name)
	}
	// Simple sort
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
}
