package domain

import "strings"

// ProviderConnection represents a saved provider account.
type ProviderConnection struct {
	ID                   string                 `json:"id"`
	Provider             string                 `json:"provider"`
	AuthType             string                 `json:"authType"` // "oauth" or "apikey"
	Name                 string                 `json:"name"`
	Priority             int                    `json:"priority"`
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
	CreatedAt            string                 `json:"createdAt,omitempty"`
	UpdatedAt            string                 `json:"updatedAt,omitempty"`
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
