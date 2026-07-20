package domain

import (
	"strconv"
	"strings"
	"unicode"
)

// ProviderConnection represents a saved provider account.
type ProviderConnection struct {
	ID                   string                 `json:"id"`
	Provider             string                 `json:"provider"`
	AuthType             string                 `json:"authType"` // "oauth" or "apikey"
	Name                 string                 `json:"name"`
	Priority             int                    `json:"priority,omitempty"`
	Weight               int                    `json:"weight"`
	IsActive             bool                   `json:"isActive"`
	AccessToken          string                 `json:"accessToken,omitempty"`
	RefreshToken         string                 `json:"refreshToken,omitempty"`
	ExpiresAt            string                 `json:"expiresAt,omitempty"`
	ExpiresIn            int                    `json:"expiresIn,omitempty"`
	Email                string                 `json:"email,omitempty"`
	APIKey               string                 `json:"apiKey,omitempty"`
	TestStatus           string                 `json:"testStatus,omitempty"`
	LastError            string                 `json:"lastError,omitempty"`
	LastErrorAt          string                 `json:"lastErrorAt,omitempty"`
	RateLimitedUntil     string                 `json:"rateLimitedUntil,omitempty"`
	BackoffLevel         int                    `json:"backoffLevel,omitempty"`
	ConsecutiveUseCount  int                    `json:"consecutiveUseCount,omitempty"`
	ProviderSpecificData map[string]interface{} `json:"providerSpecificData,omitempty"`
	ModelLocks           map[string]string      `json:"modelLocks,omitempty"`
	SupportedModels      []string               `json:"supportedModels,omitempty"`
	BaseURL              string                 `json:"baseUrl,omitempty"`
	// RoutePrefix is the public provider prefix for openai-compatible connections.
	// Example: routePrefix "windsurf" exposes models as "windsurf/<model>" and pins routing to this connection.
	RoutePrefix string `json:"routePrefix,omitempty"`
	// ModelPrefix is used by openai-compatible connections to strip a prefix
	// from incoming model names before forwarding (e.g. "my-" to strip so "my-gpt-4" becomes "gpt-4").
	ModelPrefix string `json:"modelPrefix,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	TenantID    string `json:"tenantId,omitempty"` // multi-tenancy support. Empty = legacy single-tenant.
}

// SupportsModel checks if this connection supports the given model.
// If SupportedModels is empty, all models are supported.
// Supports exact match and wildcard prefix match (e.g. "claude-*").
func (c *ProviderConnection) SupportsModel(model string) bool {
	if len(c.SupportedModels) == 0 {
		return true
	}
	for _, pattern := range c.SupportedModels {
		if pattern == model {
			return true
		}
		// Wildcard prefix match: "claude-*" matches "claude-sonnet-4.5"
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(model, prefix) {
				return true
			}
		}
	}
	return false
}

// Credentials holds the runtime credentials passed to an executor.
type Credentials struct {
	ConnectionID         string
	ConnectionName       string
	Provider             string
	AccessToken          string
	RefreshToken         string
	APIKey               string
	ProfileArn           string
	BaseURL              string
	ModelPrefix          string
	ProviderSpecificData map[string]interface{}
}

// GetProfileArn extracts profileArn from ProviderSpecificData.
func (c *Credentials) GetProfileArn() string {
	if c.ProfileArn != "" {
		return c.ProfileArn
	}
	if c.ProviderSpecificData == nil {
		return ""
	}
	if v, ok := c.ProviderSpecificData["profileArn"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetAuthMethod extracts authMethod from ProviderSpecificData.
func (c *Credentials) GetAuthMethod() string {
	if c.ProviderSpecificData == nil {
		return ""
	}
	if v, ok := c.ProviderSpecificData["authMethod"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// NormalizeRoutePrefix converts free-form provider labels into a stable route prefix.
func NormalizeRoutePrefix(prefix string) string {
	prefix = strings.TrimSpace(strings.ToLower(prefix))
	var b strings.Builder
	lastDash := false
	for _, r := range prefix {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if (r == '-' || r == '_' || r == '.') && b.Len() > 0 && !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// EnsureOpenAICompatibleRoutePrefixes fills missing custom route prefixes and avoids duplicates.
func EnsureOpenAICompatibleRoutePrefixes(conns []ProviderConnection) {
	used := make(map[string]bool, len(conns))
	for i := range conns {
		if conns[i].Provider != "openai-compatible" {
			continue
		}

		base := NormalizeRoutePrefix(conns[i].RoutePrefix)
		isExplicit := base != ""
		if base == "" {
			base = NormalizeRoutePrefix(conns[i].Name)
		}
		if base == "" {
			base = "custom"
		}
		if isExplicit {
			if _, reserved := ProviderAliasToID[base]; reserved {
				base += "-custom"
			}
		} else if _, reserved := ProviderAliasToID[base]; reserved {
			base += "-custom"
		}

		prefix := base
		for n := 2; used[prefix]; n++ {
			prefix = base + "-" + strconv.Itoa(n)
		}
		conns[i].RoutePrefix = prefix
		used[prefix] = true
	}
}
