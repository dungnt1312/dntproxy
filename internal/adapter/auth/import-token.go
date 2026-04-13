package auth

import (
	"fmt"
	"strings"
)

// ValidateAndImportToken validates a refresh token and returns credentials.
// Flow:
// 1. If clientId+clientSecret provided → SSO OIDC directly (throw on fail)
// 2. No clientId/clientSecret → register new client → try SSO OIDC
// 3. If SSO fails → silently fallback to social auth endpoint (works for IDC tokens too)
func ValidateAndImportToken(refreshToken string, clientID, clientSecret, region, authMethod string) (*TokenResult, error) {
	refreshToken = strings.TrimSpace(refreshToken)

	if !strings.HasPrefix(refreshToken, "aorAAAAAG") {
		return nil, fmt.Errorf("invalid token format: should start with aorAAAAAG...")
	}

	if region == "" {
		region = "us-east-1"
	}

	normalizedMethod := strings.ToLower(authMethod)

	// If we have clientId/clientSecret, use AWS SSO OIDC directly — throw on fail
	if clientID != "" && clientSecret != "" {
		result, err := RefreshTokenSSO(refreshToken, clientID, clientSecret, region)
		if err != nil {
			return nil, fmt.Errorf("token validation failed: %w", err)
		}
		if normalizedMethod == "" {
			normalizedMethod = "builder-id"
		}
		result.AuthMethod = normalizedMethod
		result.ClientID = clientID
		result.ClientSecret = clientSecret
		result.Region = region
		return result, nil
	}

	// No clientId/clientSecret — register new client then try SSO OIDC
	regResult, err := RegisterClient(region)
	if err == nil && regResult != nil {
		result, ssoErr := RefreshTokenSSO(refreshToken, regResult.ClientID, regResult.ClientSecret, region)
		if ssoErr == nil {
			if normalizedMethod == "" {
				normalizedMethod = "builder-id"
			}
			result.AuthMethod = normalizedMethod
			result.ClientID = regResult.ClientID
			result.ClientSecret = regResult.ClientSecret
			result.Region = region
			return result, nil
		}
		// SSO failed — silently fall through to social auth
	}

	// Social auth fallback — works for IDC tokens too
	result, err := RefreshTokenSocial(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	result.AuthMethod = "imported"
	return result, nil
}
