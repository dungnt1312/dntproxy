package domain

// RequestFormat describes the API request/response format for a provider.
type RequestFormat string

const (
	FormatOpenAIChat   RequestFormat = "openai-chat"        // POST /v1/chat/completions, SSE stream
	FormatAnthropicMsg RequestFormat = "anthropic-msg"      // POST /v1/messages, SSE + event types
	FormatAWSKiro      RequestFormat = "aws-eventstream"    // AWS EventStream binary
	FormatImageAPI     RequestFormat = "image-api"          // Image-only provider; no chat executor
	FormatCommandCode  RequestFormat = "commandcode-ndjson" // POST /alpha/generate, NDJSON stream
)

// ProviderConfig defines all static configuration for a provider.
type ProviderConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	// PublicPrefix is the short model-id prefix shown in playground, /v1/models, and combos.
	// Empty means the provider ID is used as-is.
	PublicPrefix      string        `json:"publicPrefix,omitempty"`
	AuthMethods       []string      `json:"authMethods"`
	DefaultBaseURL    string        `json:"defaultBaseUrl"`
	ChatPath          string        `json:"chatPath,omitempty"`
	RecommendedModels []string      `json:"recommendedModels"`
	DefaultModels     []string      `json:"-"`
	Format            RequestFormat `json:"format"`
	SupportsQuota     bool          `json:"supportsQuota"`
	OAuth             *OAuthConfig  `json:"oauth,omitempty"`
	UI                ProviderUI    `json:"ui"`
}

// OAuthConfig holds OAuth/OIDC endpoints and client info for a provider.
type OAuthConfig struct {
	ClientID     string `json:"clientId,omitempty"`
	AuthorizeURL string `json:"authorizeUrl,omitempty"`
	TokenURL     string `json:"tokenUrl,omitempty"`
	Scopes       string `json:"scopes,omitempty"`
	CallbackPort int    `json:"callbackPort,omitempty"`
	// Device code flow (set for providers using device authorization grant)
	DeviceCodeURL string `json:"deviceCodeUrl,omitempty"`
	// Social login variants — func cannot be serialized, excluded from JSON
	SocialAuthorizeURL func(provider, codeChallenge, state string) string `json:"-"`
}

// FormFieldType defines supported input types for dynamic connection forms.
type FormFieldType string

const (
	FieldTypeText     FormFieldType = "text"
	FieldTypePassword FormFieldType = "password"
	FieldTypeTextarea FormFieldType = "textarea"
	FieldTypeSelect   FormFieldType = "select"
	FieldTypeNumber   FormFieldType = "number"
	FieldTypeURL      FormFieldType = "url"
)

// FormField defines a dynamic form field for the UI.
type FormField struct {
	Name         string        `json:"name"`
	Label        string        `json:"label"`
	Type         FormFieldType `json:"type"`
	Required     bool          `json:"required"`
	Placeholder  string        `json:"placeholder,omitempty"`
	DefaultValue string        `json:"defaultValue,omitempty"`
	Secret       bool          `json:"secret,omitempty"` // hide in list, treat as password
	HelpText     string        `json:"helpText,omitempty"`
	Options      []string      `json:"options,omitempty"` // for select fields
}

// ProviderUI contains UI/UX metadata for the "Add Connection" flow.
type ProviderUI struct {
	Category            string      `json:"category"` // "cloud", "self-hosted", "enterprise"
	Description         string      `json:"description"`
	DocsURL             string      `json:"docsUrl,omitempty"`
	ShowBaseURLField    bool        `json:"showBaseUrlField"`
	BaseURLLabel        string      `json:"baseUrlLabel,omitempty"`
	BaseURLPlaceholder  string      `json:"baseUrlPlaceholder,omitempty"`
	PreferredAuthMethod string      `json:"preferredAuthMethod,omitempty"`
	AuthFlows           []string    `json:"authFlows"` // ["apikey"], ["oauth-device"], ["social"], ["import"]
	FormFields          []FormField `json:"formFields"`
	SupportsModelSelect bool        `json:"supportsModelSelect"`
	DefaultTestModel    string      `json:"defaultTestModel,omitempty"`
}

// Registry of all known providers.
// To add a new provider: add one entry here (with full UI metadata), register executor in main.go.
// The UI metadata makes the "Add Connection" form fully dynamic.
var ProviderConfigs = map[string]ProviderConfig{

	"kiro": {
		ID:             "kiro",
		Name:           "Kiro (AWS CodeWhisperer)",
		Icon:           "kr",
		AuthMethods:    []string{"oauth", "apikey"},
		DefaultBaseURL: "https://codewhisperer.us-east-1.amazonaws.com",
		ChatPath:       "", // Uses AWS EventStream, not HTTP
		RecommendedModels: []string{
			"claude-sonnet-5", "claude-opus-5", "claude-haiku-4.5",
			"gpt-5.6-terra", "glm-5", "qwen3-coder-next",
		},
		Format:        FormatAWSKiro,
		SupportsQuota: true,
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "Amazon CodeWhisperer / Kiro with OAuth (Builder ID, IDC, Social) or a long-lived API key",
			DocsURL:             "https://docs.aws.amazon.com/codewhisperer/latest/userguide/",
			ShowBaseURLField:    false,
			PreferredAuthMethod: "builder-id",
			// These ids must match the panels the Kiro setup form renders, and
			// PreferredAuthMethod must be one of them. A coarser list silently
			// strips panels from the UI (IDC and token import were unreachable),
			// and an unlisted preference leaves every method chip unselected.
			AuthFlows:           []string{"builder-id", "social", "idc", "apikey", "import"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "claude-sonnet-5",
		},
	},

	"openai": {
		ID:             "openai",
		Name:           "OpenAI",
		Icon:           "oai",
		AuthMethods:    []string{"apikey", "oauth"},
		DefaultBaseURL: "https://api.openai.com",
		ChatPath:       "/v1/chat/completions",
		// Chat models only; image models (gpt-image-2, gpt-image-1.5) stay in registry for detect
		RecommendedModels: []string{
			"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna",
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
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "Official OpenAI API with API key and OAuth support",
			ShowBaseURLField:    false,
			PreferredAuthMethod: "apikey",
			AuthFlows:           []string{"apikey", "oauth"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
				{Name: "apiKey", Label: "API Key", Type: FieldTypePassword, Required: true, Secret: true},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "gpt-5.6-terra",
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
		UI: ProviderUI{
			Category:            "self-hosted",
			Description:         "Any OpenAI-compatible endpoint (Groq, Together, Fireworks, local servers, etc.)",
			ShowBaseURLField:    true,
			BaseURLLabel:        "Base URL",
			BaseURLPlaceholder:  "https://api.example.com",
			PreferredAuthMethod: "apikey",
			AuthFlows:           []string{"apikey"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: true},
				{Name: "apiKey", Label: "API Key", Type: FieldTypePassword, Required: true, Secret: true},
				{Name: "baseUrl", Label: "Base URL", Type: FieldTypeURL, Required: true},
				{Name: "routePrefix", Label: "Route Prefix", Type: FieldTypeText, Required: false, Placeholder: "e.g. groq, together"},
				{Name: "modelPrefix", Label: "Model Prefix (optional)", Type: FieldTypeText, Required: false},
			},
			SupportsModelSelect: true,
		},
	},

	"glm": {
		ID:                "glm",
		Name:              "GLM (Zhipu AI)",
		Icon:              "glm",
		AuthMethods:       []string{"apikey"},
		DefaultBaseURL:    "https://api.z.ai",
		ChatPath:          "/api/coding/paas/v4/chat/completions",
		RecommendedModels: []string{"glm-5.2", "glm-5.1", "glm-5", "glm-4.7-flash"},
		Format:            FormatOpenAIChat,
		SupportsQuota:     true,
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "Zhipu AI GLM models",
			ShowBaseURLField:    true,
			BaseURLLabel:        "Base URL",
			BaseURLPlaceholder:  "https://api.z.ai",
			PreferredAuthMethod: "apikey",
			AuthFlows:           []string{"apikey"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
				{Name: "apiKey", Label: "API Key", Type: FieldTypePassword, Required: true, Secret: true},
				{Name: "baseUrl", Label: "Base URL", Type: FieldTypeURL, Required: false},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "glm-4.7-flash",
		},
	},

	"minimax": {
		ID:                "minimax",
		Name:              "MiniMax",
		Icon:              "mm",
		AuthMethods:       []string{"apikey"},
		DefaultBaseURL:    "https://api.minimax.io",
		ChatPath:          "/v1/chat/completions",
		RecommendedModels: []string{"MiniMax-M3", "MiniMax-M2.7", "MiniMax-M2.7-highspeed", "image-01"},
		Format:            FormatOpenAIChat,
		SupportsQuota:     true,
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "MiniMax M3 / M2 series models",
			ShowBaseURLField:    true,
			BaseURLLabel:        "Base URL",
			BaseURLPlaceholder:  "https://api.minimax.io",
			PreferredAuthMethod: "apikey",
			AuthFlows:           []string{"apikey"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
				{Name: "apiKey", Label: "API Key", Type: FieldTypePassword, Required: true, Secret: true},
				{Name: "baseUrl", Label: "Base URL", Type: FieldTypeURL, Required: false},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "MiniMax-M3",
		},
	},

	"qwen": {
		ID:                "qwen",
		Name:              "Qwen (Alibaba)",
		Icon:              "qw",
		AuthMethods:       []string{"apikey", "oauth"},
		DefaultBaseURL:    "https://portal.qwen.ai",
		ChatPath:          "/v1/chat/completions",
		RecommendedModels: []string{"qwen3.8-max", "qwen3.7-plus", "qwen3-coder-plus"},
		Format:            FormatOpenAIChat,
		SupportsQuota:     false,
		OAuth: &OAuthConfig{
			ClientID:      "f0304373b74a44d2b584a3fb70ca9e56",
			DeviceCodeURL: "https://chat.qwen.ai/api/v1/oauth2/device/code",
			TokenURL:      "https://chat.qwen.ai/api/v1/oauth2/token",
			Scopes:        "openid profile email model.completion",
		},
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "Alibaba Qwen models with API key and device code OAuth",
			ShowBaseURLField:    true,
			BaseURLLabel:        "Base URL",
			BaseURLPlaceholder:  "https://portal.qwen.ai",
			PreferredAuthMethod: "oauth",
			// "oauth", not "oauth-device": the Qwen form renders its device-code
			// panel behind mode === "oauth", so advertising the finer id made the
			// OAuth chip select a panel that does not exist.
			AuthFlows:           []string{"apikey", "oauth"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
				{Name: "apiKey", Label: "API Key", Type: FieldTypePassword, Required: true, Secret: true},
				{Name: "baseUrl", Label: "Base URL", Type: FieldTypeURL, Required: false},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "qwen3.8-max",
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
		RecommendedModels: []string{"claude-sonnet-5", "claude-opus-5", "claude-haiku-4-5", "claude-fable-5"},
		Format:            FormatAnthropicMsg,
		SupportsQuota:     false,
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "Anthropic Claude models via Messages API",
			ShowBaseURLField:    true,
			BaseURLLabel:        "Base URL",
			BaseURLPlaceholder:  "https://api.anthropic.com",
			PreferredAuthMethod: "apikey",
			AuthFlows:           []string{"apikey"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
				{Name: "apiKey", Label: "API Key (sk-ant-...)", Type: FieldTypePassword, Required: true, Secret: true},
				{Name: "baseUrl", Label: "Base URL", Type: FieldTypeURL, Required: false},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "claude-sonnet-5",
		},
	},

	// ClinePass
	"cline": {
		ID:             "cline",
		Name:           "ClinePass",
		Icon:           "cl",
		AuthMethods:    []string{"apikey"},
		DefaultBaseURL: "https://api.cline.bot",
		ChatPath:       "/api/v1/chat/completions",
		RecommendedModels: []string{
			"cline-pass/glm-5.2", "cline-pass/kimi-k3", "cline-pass/kimi-k2.7-code",
			"cline-pass/deepseek-v4-pro", "cline-pass/deepseek-v4-flash",
			"cline-pass/mimo-v2.5-pro", "cline-pass/minimax-m3",
			"cline-pass/qwen3.8-max", "cline-pass/qwen3.7-plus",
		},
		Format:        FormatOpenAIChat,
		SupportsQuota: false,
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "ClinePass subscription models (multiple providers)",
			ShowBaseURLField:    true,
			BaseURLLabel:        "Base URL",
			BaseURLPlaceholder:  "https://api.cline.bot",
			PreferredAuthMethod: "apikey",
			AuthFlows:           []string{"apikey"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
				{Name: "apiKey", Label: "API Key", Type: FieldTypePassword, Required: true, Secret: true},
				{Name: "baseUrl", Label: "Base URL", Type: FieldTypeURL, Required: false},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "cline-pass/glm-5.2",
		},
	},

	"gemini": {
		ID:             "gemini",
		Name:           "Google Gemini",
		Icon:           "gem",
		AuthMethods:    []string{"apikey"},
		DefaultBaseURL: "https://generativelanguage.googleapis.com",
		ChatPath:       "/v1beta/openai/chat/completions",
		RecommendedModels: []string{
			"gemini-3.7-flash",
			"gemini-3.6-flash",
			"gemini-3.5-flash",
			"gemini-3.1-flash-image",
		},
		Format:        FormatOpenAIChat,
		SupportsQuota: false,
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "Google Gemini via OpenAI-compatible endpoint",
			ShowBaseURLField:    true,
			BaseURLLabel:        "Base URL",
			BaseURLPlaceholder:  "https://generativelanguage.googleapis.com",
			PreferredAuthMethod: "apikey",
			AuthFlows:           []string{"apikey"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
				{Name: "apiKey", Label: "API Key", Type: FieldTypePassword, Required: true, Secret: true},
				{Name: "baseUrl", Label: "Base URL", Type: FieldTypeURL, Required: false},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "gemini-3.7-flash",
		},
	},

	"byteplus": {
		ID:             "byteplus",
		Name:           "BytePlus ModelArk",
		Icon:           "bp",
		AuthMethods:    []string{"apikey"},
		DefaultBaseURL: "https://ark.ap-southeast.bytepluses.com/api/v3",
		ChatPath:       "/models",
		RecommendedModels: []string{
			"dola-seedream-5-0-pro-260628",
			"seedream-5-0-lite-260128",
			"seedream-4-5-251128",
		},
		Format:        FormatImageAPI,
		SupportsQuota: false,
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "BytePlus ModelArk Seedream image generation and reference editing",
			DocsURL:             "https://docs.byteplus.com/en/docs/ModelArk/1541523",
			ShowBaseURLField:    true,
			BaseURLLabel:        "Regional Base URL",
			BaseURLPlaceholder:  "https://ark.ap-southeast.bytepluses.com/api/v3",
			PreferredAuthMethod: "apikey",
			AuthFlows:           []string{"apikey"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
				{Name: "apiKey", Label: "ModelArk API Key", Type: FieldTypePassword, Required: true, Secret: true},
				{Name: "baseUrl", Label: "Regional Base URL", Type: FieldTypeURL, Required: false},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "dola-seedream-5-0-pro-260628",
		},
	},

	"commandcode": {
		ID:             "commandcode",
		Name:           "Command Code",
		Icon:           "cmc",
		PublicPrefix:   "cmc",
		AuthMethods:    []string{"apikey"},
		DefaultBaseURL: "https://api.commandcode.ai",
		ChatPath:       "/alpha/generate",
		RecommendedModels: []string{
			"deepseek/deepseek-v4-pro",
			"deepseek/deepseek-v4-flash",
			"MiniMaxAI/MiniMax-M3",
			"MiniMaxAI/MiniMax-M2.7",
			"zai-org/GLM-5.1",
			"moonshotai/Kimi-K2.6",
			"Qwen/Qwen3.7-Max",
			"Qwen/Qwen3.7-Max-Free",
			"stepfun/Step-3.7-Flash",
			"xiaomi/mimo-v2.5-pro",
		},
		Format:        FormatCommandCode,
		SupportsQuota: true,
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "Command Code models via CLI generate API (works on Go plan)",
			DocsURL:             "https://commandcode.ai/docs/provider",
			ShowBaseURLField:    true,
			BaseURLLabel:        "Base URL",
			BaseURLPlaceholder:  "https://api.commandcode.ai",
			PreferredAuthMethod: "import",
			AuthFlows:           []string{"import", "apikey"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
				{Name: "apiKey", Label: "API Key", Type: FieldTypePassword, Required: true, Secret: true, Placeholder: "user_...", HelpText: "From ~/.commandcode/auth.json or Studio API keys"},
				{Name: "baseUrl", Label: "Base URL", Type: FieldTypeURL, Required: false},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "deepseek/deepseek-v4-pro",
		},
	},

	"xai": {
		ID:             "xai",
		Name:           "Grok Build (xAI)",
		Icon:           "grok",
		AuthMethods:    []string{"oauth"},
		DefaultBaseURL: "https://api.x.ai/v1",
		ChatPath:       "/responses",
		RecommendedModels: []string{
			"grok-4.6", "grok-4.5", "grok-4.3",
		},
		Format:        FormatOpenAIChat,
		SupportsQuota: false,
		UI: ProviderUI{
			Category:            "cloud",
			Description:         "xAI Grok models with OAuth",
			ShowBaseURLField:    false,
			PreferredAuthMethod: "oauth",
			AuthFlows:           []string{"oauth"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
			},
			SupportsModelSelect: true,
			DefaultTestModel:    "grok-4.6",
		},
	},
}

// PublicProviderPrefix returns the short prefix used in model IDs and UI.
// Unknown providers keep their raw ID.
func PublicProviderPrefix(providerID string) string {
	if cfg, ok := ProviderConfigs[providerID]; ok && cfg.PublicPrefix != "" {
		return cfg.PublicPrefix
	}
	return providerID
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
		// Ensure UI is populated for fallback cases
		if len(cfg.UI.AuthFlows) == 0 {
			cfg.UI = ProviderUI{
				Category:            "cloud",
				Description:         cfg.Name,
				ShowBaseURLField:    true,
				PreferredAuthMethod: "apikey",
				AuthFlows:           cfg.AuthMethods,
				FormFields: []FormField{
					{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: false},
					{Name: "apiKey", Label: "API Key", Type: FieldTypePassword, Required: true, Secret: true},
					{Name: "baseUrl", Label: "Base URL", Type: FieldTypeURL, Required: false},
				},
				SupportsModelSelect: true,
			}
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
		UI: ProviderUI{
			Category:            "self-hosted",
			Description:         "Custom " + providerID + " endpoint",
			ShowBaseURLField:    true,
			PreferredAuthMethod: "apikey",
			AuthFlows:           []string{"apikey"},
			FormFields: []FormField{
				{Name: "name", Label: "Connection Name", Type: FieldTypeText, Required: true},
				{Name: "apiKey", Label: "API Key", Type: FieldTypePassword, Required: true, Secret: true},
				{Name: "baseUrl", Label: "Base URL", Type: FieldTypeURL, Required: true},
			},
			SupportsModelSelect: true,
		},
	}
}

// GetAllProviderMetadata returns sanitized provider configs for the frontend.
func GetAllProviderMetadata() []ProviderConfig {
	ids := ListProviders()
	result := make([]ProviderConfig, 0, len(ids))
	for _, id := range ids {
		c := GetProviderConfig(id)
		if c.OAuth != nil {
			c.OAuth = &OAuthConfig{ClientID: c.OAuth.ClientID}
		}
		result = append(result, c)
	}
	return result
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

// GetDefaultConnectionModels returns the default model list for new connections.
// Checks settings.DefaultModels first, falls back to built-in RecommendedModels.
func GetDefaultConnectionModels(settings *Settings, providerID string) []string {
	if settings != nil {
		if models, ok := settings.DefaultModels[providerID]; ok && len(models) > 0 {
			return models
		}
	}
	return GetProviderConfig(providerID).RecommendedModels
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
