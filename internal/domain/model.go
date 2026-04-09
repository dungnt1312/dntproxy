package domain

// ModelInfo represents a resolved provider + model pair.
type ModelInfo struct {
	Provider      string
	Model         string
	ProviderAlias string
}

// Combo represents a named group of models with fallback/round-robin.
type Combo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Models        []string `json:"models"`
	ConnectionIDs []string `json:"connectionIds,omitempty"` // optional: restrict to specific connections
	CreatedAt     string   `json:"createdAt,omitempty"`
	UpdatedAt     string   `json:"updatedAt,omitempty"`
}

// AliasMap is model alias → "provider/model" string.
type AliasMap map[string]string

// ProviderAliasToID maps short alias to full provider ID.
var ProviderAliasToID = map[string]string{
	"kr":   "kiro",
	"oai":  "openai",
	"oaic": "openai-compatible",
	"cc":   "claude",
	"cx":   "codex",
	"gc":   "gemini-cli",
	"qw":   "qwen",
	"if":   "iflow",
	"ag":   "antigravity",
	"gh":   "github",
	"cu":   "cursor",
	"kc":   "kilocode",
	"cl":   "cline",
	"oc":   "opencode",
	"ds":   "deepseek",
	"nb":   "nanobanana",
	"ch":   "chutes",
	"vx":   "vertex",
	"vxp":  "vertex-partner",
	"hyp":  "hyperbolic",
	"pplx": "perplexity",
	// Full names map to themselves
	"openai":     "openai",
	"anthropic":  "anthropic",
	"gemini":     "gemini",
	"openrouter": "openrouter",
	"glm":        "glm",
	"kimi":       "kimi",
	"minimax":    "minimax",
	"deepseek":   "deepseek",
	"groq":       "groq",
	"xai":        "xai",
	"mistral":    "mistral",
	"perplexity": "perplexity",
	"together":   "together",
	"fireworks":  "fireworks",
	"cerebras":   "cerebras",
	"cohere":     "cohere",
	"nvidia":     "nvidia",
	"nebius":     "nebius",
	"kiro":       "kiro",
}
