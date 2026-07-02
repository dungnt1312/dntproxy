package domain

// RequestFormat describes the API request/response format for a provider.
type RequestFormat string

const (
	FormatOpenAIChat   RequestFormat = "openai-chat"     // POST /v1/chat/completions, SSE stream
	FormatAnthropicMsg RequestFormat = "anthropic-msg"   // POST /v1/messages, SSE + event types
	FormatAWSKiro      RequestFormat = "aws-eventstream" // AWS EventStream binary
)

// ProviderConfig defines all static configuration for a provider.
// This is the single source of truth for provider-specific defaults.
type ProviderConfig struct {
	ID             string
	Name           string
	Icon           string
	AuthMethods    []string
	DefaultBaseURL string
	ChatPath       string
		// RecommendedModels is the curated list of models shown by default when creating a connection.
		// This is the new preferred field. DefaultModels exists for backward compatibility.
		RecommendedModels []string
		DefaultModels     []string `json:"-"` // deprecated, populated from RecommendedModels

		// Format specifies the API protocol. Defaults to OpenAI-compatible.
		// Set to "anthropic-msg" for Anthropic, "aws-eventstream" for Kiro.
		Format RequestFormat
	// SupportsQuota indicates whether this provider has a quota/usage API.
	// When false, the UI hides the quota fetch button entirely.
	SupportsQuota bool
	// OAuth config (only set for providers that support OAuth)
	OAuth *OAuthConfig
}

// OAuthConfig holds OAuth/OIDC endpoints and client info for a provider.
type OAuthConfig struct {
	ClientID     string
	AuthorizeURL string
	TokenURL     string
	Scopes       string
	CallbackPort int
	// Device code flow (set for providers using device authorization grant)
	DeviceCodeURL string
	// Social login variants
	SocialAuthorizeURL func(provider, codeChallenge, state string) string
}

// Registry of all known providers.
// To add a new provider: add one entry here, register executor in main.go.
var ProviderConfigs = map[string]ProviderConfig{
		"kiro": {
			ID:                "kiro",
			Name:              "Kiro (AWS CodeWhisperer)",
			Icon:              "kr",
			AuthMethods:       []string{"oauth"},
			DefaultBaseURL:    "https://codewhisperer.us-east-1.amazonaws.com",
			ChatPath:          "", // Uses AWS EventStream, not HTTP
			RecommendedModels: []string{"claude-sonnet-4.6", "claude-sonnet-4.5", "claude-opus-4.6"},
			Format:            FormatAWSKiro,
			SupportsQuota:     true,
			// RecommendedModels is now the source of truth. Detect endpoint can still expand the list.
		},

		"openai": {
			ID:                "openai",
			Name:              "OpenAI",
			Icon:              "oai",
			AuthMethods:       []string{"apikey", "oauth"},
			DefaultBaseURL:    "https://api.openai.com",
			ChatPath:          "/v1/chat/completions",
			RecommendedModels: []string{
				"gpt-5.5",
				"gpt-5.4",
				"gpt-5.4-mini",
				"gpt-5.3-codex-spark",
				"codex-auto-review",
				"gpt-image-2",
				"gpt-image-1.5",
			},
			Format:        FormatOpenAIChat,
			SupportsQuota: true,
			OAuth: &OAuthConfig{
				ClientID:     "app_EMoamEEZ73f0CkXaXp7hrann",
				AuthorizeURL: "https://auth.openai.com/oauth/authorize",
				TokenURL:     "https://auth.openai.com/oauth/token",
				Scopes:       "openid profile email offline_access",
				CallbackPort: 1455,
			},
		},

		"openai-compatible": {
			ID:                "openai-compatible",
			Name:              "OpenAI Compatible",
			Icon:              "api",
			AuthMethods:       []string{"apikey"},
			DefaultBaseURL:    "https://api.openai.com",
			ChatPath:          "/v1/chat/completions",
			Format:            FormatOpenAIChat,
			SupportsQuota:     false,
			RecommendedModels: []string{}, // User must provide or use detect
		},

		"glm": {
			ID:                "glm",
			Name:              "GLM (Zhipu AI)",
			Icon:              "glm",
			AuthMethods:       []string{"apikey"},
			DefaultBaseURL:    "https://api.z.ai",
			ChatPath:          "/api/coding/paas/v4/chat/completions",
			RecommendedModels: []string{"glm-5.1", "glm-5", "glm-4.7-flash"},
			Format:            FormatOpenAIChat,
			SupportsQuota:     false,
		},

		"minimax": {
			ID:                "minimax",
			Name:              "MiniMax",
			Icon:              "mm",
			AuthMethods:       []string{"apikey"},
			DefaultBaseURL:    "https://api.minimax.io",
			ChatPath:          "/v1/chat/completions",
			RecommendedModels: []string{"MiniMax-M2.7", "MiniMax-M2.7-highspeed", "MiniMax-M2.5"},
			Format:            FormatOpenAIChat,
			SupportsQuota:     true,
		},

		"qwen": {
			ID:                "qwen",
			Name:              "Qwen (Alibaba)",
			Icon:              "qw",
			AuthMethods:       []string{"apikey", "oauth"},
			DefaultBaseURL:    "https://portal.qwen.ai",
			ChatPath:          "/v1/chat/completions",
			RecommendedModels: []string{"qwen3-coder-plus", "qwen3-coder", "qwen-turbo"},
			Format:            FormatOpenAIChat,
			SupportsQuota:     false,
			OAuth: &OAuthConfig{
				ClientID:      "f0304373b74a44d2b584a3fb70ca9e56",
				DeviceCodeURL: "https://chat.qwen.ai/api/v1/oauth2/device/code",
				TokenURL:      "https://chat.qwen.ai/api/v1/oauth2/token",
				Scopes:        "openid profile email model.completion",
			},
		},

	// Anthropic (Claude API)
		"anthropic": {
			ID:                "anthropic",
			Name:              "Anthropic (Claude API)",
			Icon:              "ANT",
			AuthMethods:       []string{"apikey"},
			DefaultBaseURL:    "https://api.anthropic.com",
			ChatPath:          "/v1/messages",
			RecommendedModels: []string{"claude-sonnet", "claude-opus", "claude-haiku"},
			Format:            FormatAnthropicMsg,
			SupportsQuota:     false,
		},

		"gemini": {
			ID:                "gemini",
			Name:              "Google Gemini",
			Icon:              "gem",
			AuthMethods:       []string{"apikey"},
			DefaultBaseURL:    "https://generativelanguage.googleapis.com",
			ChatPath:          "/v1beta/openai/chat/completions",
			RecommendedModels: []string{"gemini-2.5-flash"},
			Format:            FormatOpenAIChat,
			SupportsQuota:     false,
		},

		"xai": {
			ID:                "xai",
			Name:              "Grok Build (xAI)",
			Icon:              "grok",
			AuthMethods:       []string{"oauth"},
			DefaultBaseURL:    "https://api.x.ai/v1",
			ChatPath:          "/responses",
			RecommendedModels: []string{"grok-4.20-0309-reasoning", "grok-4.3"},
			Format:            FormatOpenAIChat,
			SupportsQuota:     false,
		},
}

	// GetProviderConfig returns the config for a provider ID.
	// RecommendedModels is now the single source of truth for sensible defaults when creating a connection.
	// It is curated per-provider (not auto-filtered from entire registry).
	func GetProviderConfig(providerID string) ProviderConfig {
		if cfg, ok := ProviderConfigs[providerID]; ok {
			// Populate RecommendedModels from curated list if not explicitly overridden
			if len(cfg.RecommendedModels) == 0 && providerID != "openai-compatible" {
				cfg.RecommendedModels = GetRecommendedModelsForProvider(providerID)
			}
			// Backward compatibility for code that still reads .DefaultModels
			if len(cfg.DefaultModels) == 0 {
				cfg.DefaultModels = cfg.RecommendedModels
			}
			return cfg
		}
	// Fallback: treat as OpenAI-compatible
	return ProviderConfig{
		ID:             providerID,
		Name:           providerID,
		Icon:           "api",
		AuthMethods:    []string{"apikey"},
		DefaultBaseURL: "https://api.openai.com",
		ChatPath:       "/v1/chat/completions",
		Format:         FormatOpenAIChat,
		DefaultModels:  GetDefaultModelsForProvider(providerID),
	}
}

// HasProvider checks if a provider is registered.
func HasProvider(providerID string) bool {
	_, ok := ProviderConfigs[providerID]
	return ok
}

// ListProviders returns all registered provider IDs sorted by name.
func ListProviders() []string {
	result := make([]string, 0, len(ProviderConfigs))
	for id := range ProviderConfigs {
		result = append(result, id)
	}
	// Simple sort by name
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if ProviderConfigs[result[j]].Name < ProviderConfigs[result[i]].Name {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// StripVersionSuffix removes trailing /v1, /v2, /v3, /v4 from a base URL.
// This prevents double version paths when the chat path already includes version.
func StripVersionSuffix(baseURL string) string {
	for _, suffix := range []string{"/v1", "/v2", "/v3", "/v4"} {
		if len(baseURL) >= len(suffix) && baseURL[len(baseURL)-len(suffix):] == suffix {
			return baseURL[:len(baseURL)-len(suffix)]
		}
	}
	return baseURL
}
