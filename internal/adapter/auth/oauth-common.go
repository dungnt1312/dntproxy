package auth

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
)

// TokenResult holds tokens from any auth flow.
type TokenResult struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ProfileArn   string `json:"profileArn,omitempty"`
	ExpiresIn    int    `json:"expiresIn"`
	TokenType    string `json:"tokenType,omitempty"`
	// For Builder ID/IDC — persist for future refreshes
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	// ClientSecretExpiresAt is the RFC3339 expiry of the OIDC client registration.
	ClientSecretExpiresAt string `json:"clientSecretExpiresAt,omitempty"`
	Region                string `json:"region,omitempty"`
	AuthMethod            string `json:"authMethod,omitempty"`
}

// DeviceAuthResult holds the result of a device authorization request (Qwen).
type DeviceAuthResult struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	ExpiresIn               int    `json:"expiresIn"`
	Interval                int    `json:"interval"`
}

// PollResult holds the result of device code polling (Qwen).
type PollResult struct {
	Success          bool
	Tokens           *TokenResult
	Error            string
	ErrorDescription string
	Pending          bool
}

var httpClient = shared.NewSafeHTTPClient(30 * time.Second)

var awsRegionPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,30}[a-z0-9])?$`)

// NormalizeAWSRegion accepts empty (defaults to us-east-1) or a simple
// AWS-style region label. Dots and slashes are rejected so the value cannot
// change the OIDC hostname.
func NormalizeAWSRegion(region string) (string, error) {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" {
		return "us-east-1", nil
	}
	if strings.ContainsAny(region, "./:@") || !awsRegionPattern.MatchString(region) {
		return "", fmt.Errorf("invalid AWS region")
	}
	return region, nil
}

// UnixToRFC3339 converts an epoch-seconds expiry to RFC3339 ("" when unset).
func UnixToRFC3339(sec int64) string { return unixToRFC3339(sec) }

func unixToRFC3339(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}

func intFromJSON(data map[string]interface{}, key string) int {
	if v, ok := data[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}
