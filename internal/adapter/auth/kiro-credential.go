package auth

import "net/url"

// Kiro provider names as written in kiro-auth-token.json.
const (
	ProviderBuilderID   = "BuilderId"
	ProviderEnterprise  = "Enterprise"
	ProviderInternal    = "Internal"
	ProviderGoogle      = "Google"
	ProviderGithub      = "Github"
	ProviderExternalIdP = "ExternalIdp"
)

// KiroCredential is the outcome of a Kiro login, before it is stored as a
// provider connection.
type KiroCredential struct {
	Provider     string // Kiro provider name (ProviderBuilderID, ...)
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	ProfileArn   string

	// IdC
	Region                string
	StartURL              string
	ClientID              string
	ClientSecret          string
	ClientSecretExpiresAt string
}

// AuthMethod returns the dntproxy authMethod value for the credential.
func (k *KiroCredential) AuthMethod() string {
	switch k.Provider {
	case ProviderBuilderID:
		return "builder-id"
	case ProviderEnterprise, ProviderInternal:
		return "idc"
	case ProviderGoogle:
		return "google"
	case ProviderGithub:
		return "github"
	}
	return "imported"
}

// ProviderLabel is the human label shown in the dashboard.
func (k *KiroCredential) ProviderLabel() string {
	switch k.Provider {
	case ProviderBuilderID:
		return "AWS Builder ID"
	case ProviderEnterprise:
		return "AWS IAM Identity Center"
	case ProviderInternal:
		return "AWS IAM Identity Center (Internal)"
	case ProviderGoogle:
		return "Google"
	case ProviderGithub:
		return "GitHub"
	}
	return "Kiro"
}

// ProviderData builds the connection's providerSpecificData.
func (k *KiroCredential) ProviderData() map[string]interface{} {
	m := map[string]interface{}{
		"authMethod":   k.AuthMethod(),
		"provider":     k.ProviderLabel(),
		"kiroProvider": k.Provider,
	}
	set := func(key, val string) {
		if val != "" {
			m[key] = val
		}
	}
	set("profileArn", k.ProfileArn)
	set("region", k.Region)
	set("startUrl", k.StartURL)
	set("clientId", k.ClientID)
	set("clientSecret", k.ClientSecret)
	set("clientSecretExpiresAt", k.ClientSecretExpiresAt)
	if k.ClientID != "" && k.StartURL != "" {
		m["clientIdHash"] = ClientIDHash(k.StartURL)
	}
	return m
}

// orderedQuery encodes key/value pairs in the given order, like URLSearchParams.
func orderedQuery(pairs ...string) string {
	var b []byte
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			b = append(b, '&')
		}
		b = append(b, url.QueryEscape(pairs[i])...)
		b = append(b, '=')
		b = append(b, url.QueryEscape(pairs[i+1])...)
	}
	return string(b)
}
