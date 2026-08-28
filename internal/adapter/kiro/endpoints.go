package kiro

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
)

// Kiro exposes GenerateAssistantResponse on three different surfaces. They are
// not interchangeable: each authentication method is only accepted by a subset,
// so the executor walks them in an order that depends on the credential type.
const (
	kiroRuntimeHost       = "https://runtime.us-east-1.kiro.dev"
	kiroCodeWhispererHost = "https://codewhisperer.us-east-1.amazonaws.com"
	kiroQHost             = "https://q.us-east-1.amazonaws.com"

	generateAssistantResponsePath = "/generateAssistantResponse"

	// AuthMethodAPIKey marks a connection authenticated with a long-lived Kiro
	// API key sent verbatim as a bearer token.
	AuthMethodAPIKey = "api_key"
	// AuthMethodExternalIDP marks a connection whose access token was minted by
	// an external identity provider (e.g. Microsoft Entra).
	AuthMethodExternalIDP = "external_idp"
	// AuthMethodIDC marks an AWS IAM Identity Center connection.
	AuthMethodIDC = "idc"

	// DefaultRegion is used whenever a connection stores no explicit region.
	DefaultRegion = "us-east-1"
)

// awsRegionPattern guards the region against being used to build arbitrary
// hostnames from user input.
var awsRegionPattern = regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d{1,2}$`)

// ValidateRegion rejects regions that would let a caller point requests at an
// arbitrary host through hostname interpolation.
func ValidateRegion(region string) error {
	region = strings.TrimSpace(region)
	if region == "" {
		return nil
	}
	if !awsRegionPattern.MatchString(region) {
		return fmt.Errorf("invalid AWS region: %q", region)
	}
	return nil
}

// NormalizeRegion returns a validated region, falling back to DefaultRegion.
func NormalizeRegion(region string) string {
	region = strings.TrimSpace(region)
	if region == "" || ValidateRegion(region) != nil {
		return DefaultRegion
	}
	return region
}

// IsAPIKeyAuth reports whether the credentials are a raw Kiro API key.
func IsAPIKeyAuth(creds *domain.Credentials) bool {
	if creds == nil {
		return false
	}
	return creds.GetAuthMethod() == AuthMethodAPIKey
}

// APIKeyValue returns the bearer value for an API-key connection. The key may
// live in either field depending on how the connection was created or imported.
func APIKeyValue(creds *domain.Credentials) string {
	if creds == nil {
		return ""
	}
	if key := strings.TrimSpace(creds.APIKey); key != "" {
		return key
	}
	return strings.TrimSpace(creds.AccessToken)
}

// ConnectionUsesAPIKey reports whether a stored connection authenticates with a
// raw Kiro API key rather than an OAuth token.
func ConnectionUsesAPIKey(conn *domain.ProviderConnection) bool {
	if conn == nil || conn.ProviderSpecificData == nil {
		return false
	}
	method, _ := conn.ProviderSpecificData["authMethod"].(string)
	return method == AuthMethodAPIKey
}

// ConnectionBearer returns the value to send in the Authorization header. API
// key connections store the key itself; OAuth connections store an access token.
func ConnectionBearer(conn *domain.ProviderConnection) string {
	if conn == nil {
		return ""
	}
	if ConnectionUsesAPIKey(conn) {
		if key := strings.TrimSpace(conn.APIKey); key != "" {
			return key
		}
	}
	if token := strings.TrimSpace(conn.AccessToken); token != "" {
		return token
	}
	return strings.TrimSpace(conn.APIKey)
}

// ApplyConnectionAuth sets the Authorization header plus the TokenType header
// that AWS requires for non-OAuth credentials. GetUsageLimits and
// ListAvailableModels reject API keys with 401/403 when TokenType is missing.
func ApplyConnectionAuth(header interface{ Set(string, string) }, conn *domain.ProviderConnection) {
	bearer := ConnectionBearer(conn)
	if bearer == "" {
		return
	}
	header.Set("Authorization", "Bearer "+bearer)
	if ConnectionUsesAPIKey(conn) {
		header.Set("TokenType", "API_KEY")
	}
}

// credentialRegion reads the region recorded on the connection.
func credentialRegion(creds *domain.Credentials) string {
	if creds == nil || creds.ProviderSpecificData == nil {
		return DefaultRegion
	}
	if v, ok := creds.ProviderSpecificData["region"].(string); ok {
		return NormalizeRegion(v)
	}
	return DefaultRegion
}

// regionalize rewrites the region segment of an amazonaws.com host. The
// kiro.dev surface is single-region and is never rewritten.
func regionalize(url, region string) string {
	if region == "" || region == DefaultRegion || !strings.Contains(url, "amazonaws.com") {
		return url
	}
	return strings.Replace(url, "."+DefaultRegion+".amazonaws.com", "."+region+".amazonaws.com", 1)
}

// OrderedEndpoints returns the GenerateAssistantResponse URLs to try, most
// likely to succeed first.
//
// API keys work on the Amazon Q surface: the legacy CodeWhisperer endpoint
// authenticates the key but then rejects the identical payload with a terminal
// 400 REQUEST_BODY_INVALID. Putting the Q host first is what makes API-key auth
// usable at all. OAuth credentials keep the single CodeWhisperer host they have
// always used.
func OrderedEndpoints(creds *domain.Credentials) []string {
	codeWhisperer := kiroCodeWhispererHost + generateAssistantResponsePath

	authMethod := ""
	if creds != nil {
		authMethod = creds.GetAuthMethod()
	}

	if authMethod != AuthMethodAPIKey && authMethod != AuthMethodExternalIDP {
		// OAuth accounts (Builder ID, IDC, social, imported) have always used the
		// CodeWhisperer surface successfully. Keep them on exactly that one host
		// so a 401 still means "refresh this token" rather than "try elsewhere".
		return []string{codeWhisperer}
	}

	region := credentialRegion(creds)
	q := regionalize(kiroQHost+generateAssistantResponsePath, region)
	cw := regionalize(codeWhisperer, region)

	if authMethod == AuthMethodAPIKey {
		// The legacy CodeWhisperer host authenticates an API key but rejects the
		// same accepted payload with 400 REQUEST_BODY_INVALID, and 400 is
		// terminal — so Q has to be tried first or it is never reached.
		return []string{q, cw}
	}
	return []string{cw, q}
}

// shouldTryNextEndpoint reports whether an upstream status means "wrong surface
// for this credential" rather than a real failure. 400/422 are payload-level
// verdicts and must stay terminal, otherwise a malformed request would be
// replayed against every host.
func shouldTryNextEndpoint(status int) bool {
	return status == 401 || status == 403 || status == 404
}
