package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// KiroConfig holds Kiro OAuth constants.
var KiroConfig = struct {
	SSOOIDCEndpoint string
	StartURL        string
	ClientName      string
	ClientType      string
	Scopes          []string
	GrantTypes      []string
	IssuerURL       string
	SocialAuthURL   string
	SocialTokenURL  string
	SocialRefreshURL string
	SocialRedirectURI string
}{
	SSOOIDCEndpoint:   "https://oidc.us-east-1.amazonaws.com",
	StartURL:          "https://view.awsapps.com/start",
	ClientName:        "kiro-oauth-client",
	ClientType:        "public",
	Scopes:            []string{"codewhisperer:completions", "codewhisperer:analysis", "codewhisperer:conversations"},
	GrantTypes:        []string{"urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
	IssuerURL:         "https://identitycenter.amazonaws.com/ssoins-722374e8c3c8e6c6",
	SocialAuthURL:     "https://prod.us-east-1.auth.desktop.kiro.dev",
	SocialTokenURL:    "https://prod.us-east-1.auth.desktop.kiro.dev/oauth/token",
	SocialRefreshURL:  "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken",
	SocialRedirectURI: "kiro://kiro.kiroAgent/authenticate-success",
}

// RegisterClientResult holds the result of OIDC client registration.
type RegisterClientResult struct {
	ClientID              string `json:"clientId"`
	ClientSecret          string `json:"clientSecret"`
	ClientSecretExpiresAt int64  `json:"clientSecretExpiresAt"`
}

// DeviceAuthResult holds the result of device authorization.
type DeviceAuthResult struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	ExpiresIn               int    `json:"expiresIn"`
	Interval                int    `json:"interval"`
}

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
	Region       string `json:"region,omitempty"`
	AuthMethod   string `json:"authMethod,omitempty"`
}

// PollResult holds the result of device code polling.
type PollResult struct {
	Success          bool
	Tokens           *TokenResult
	Error            string
	ErrorDescription string
	Pending          bool
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// RegisterClient registers an OIDC client with AWS SSO.
func RegisterClient(region string) (*RegisterClientResult, error) {
	if region == "" {
		region = "us-east-1"
	}
	endpoint := fmt.Sprintf("https://oidc.%s.amazonaws.com/client/register", region)

	body := map[string]interface{}{
		"clientName": KiroConfig.ClientName,
		"clientType": KiroConfig.ClientType,
		"scopes":     KiroConfig.Scopes,
		"grantTypes": KiroConfig.GrantTypes,
		"issuerUrl":  KiroConfig.IssuerURL,
	}

	resp, err := postJSONStrict(endpoint, body, nil)
	if err != nil {
		return nil, fmt.Errorf("register client: %w", err)
	}

	var result RegisterClientResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse register response: %w", err)
	}
	return &result, nil
}

// StartDeviceAuthorization begins the device code flow.
func StartDeviceAuthorization(clientID, clientSecret, startURL, region string) (*DeviceAuthResult, error) {
	if region == "" {
		region = "us-east-1"
	}
	endpoint := fmt.Sprintf("https://oidc.%s.amazonaws.com/device_authorization", region)

	body := map[string]interface{}{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"startUrl":     startURL,
	}

	resp, err := postJSONStrict(endpoint, body, nil)
	if err != nil {
		return nil, fmt.Errorf("start device auth: %w", err)
	}

	var result DeviceAuthResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse device auth response: %w", err)
	}
	if result.Interval == 0 {
		result.Interval = 5
	}
	return &result, nil
}

// PollDeviceToken polls for token using device code.
func PollDeviceToken(clientID, clientSecret, deviceCode, region string) (*PollResult, error) {
	if region == "" {
		region = "us-east-1"
	}
	endpoint := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", region)

	body := map[string]interface{}{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"deviceCode":   deviceCode,
		"grantType":    "urn:ietf:params:oauth:grant-type:device_code",
	}

	respBytes, _, err := postJSON(endpoint, body, nil)
	if err != nil {
		return nil, fmt.Errorf("poll device token: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(respBytes, &data); err != nil {
		return nil, fmt.Errorf("parse poll response: %w", err)
	}

	// Check for error
	if errStr, ok := data["error"].(string); ok && errStr != "" {
		pending := errStr == "authorization_pending" || errStr == "slow_down"
		desc, _ := data["error_description"].(string)
		return &PollResult{
			Success:          false,
			Error:            errStr,
			ErrorDescription: desc,
			Pending:          pending,
		}, nil
	}

	accessToken, _ := data["accessToken"].(string)
	refreshToken, _ := data["refreshToken"].(string)
	expiresIn := intFromJSON(data, "expiresIn")
	tokenType, _ := data["tokenType"].(string)

	return &PollResult{
		Success: true,
		Tokens: &TokenResult{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    expiresIn,
			TokenType:    tokenType,
		},
	}, nil
}

// RefreshTokenSSO refreshes a token using AWS SSO OIDC (Builder ID / IDC).
func RefreshTokenSSO(refreshToken, clientID, clientSecret, region string) (*TokenResult, error) {
	if region == "" {
		region = "us-east-1"
	}
	endpoint := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", region)

	body := map[string]interface{}{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"refreshToken": refreshToken,
		"grantType":    "refresh_token",
	}

	resp, err := postJSONStrict(endpoint, body, nil)
	if err != nil {
		return nil, fmt.Errorf("refresh SSO token: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}

	accessToken, _ := data["accessToken"].(string)
	newRefresh, _ := data["refreshToken"].(string)
	if newRefresh == "" {
		newRefresh = refreshToken
	}
	expiresIn := intFromJSON(data, "expiresIn")

	return &TokenResult{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		ExpiresIn:    expiresIn,
	}, nil
}

// RefreshTokenSocial refreshes a token using Kiro social auth endpoint.
func RefreshTokenSocial(refreshToken string) (*TokenResult, error) {
	body := map[string]interface{}{
		"refreshToken": refreshToken,
	}

	headers := map[string]string{
		"User-Agent": "KiroIDE-0.10.32-kiro-proxy",
		"Accept":     "application/json, text/plain, */*",
	}

	resp, err := postJSONStrict(KiroConfig.SocialRefreshURL, body, headers)
	if err != nil {
		return nil, fmt.Errorf("refresh social token: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("parse social refresh response: %w", err)
	}

	accessToken, _ := data["accessToken"].(string)
	newRefresh, _ := data["refreshToken"].(string)
	if newRefresh == "" {
		newRefresh = refreshToken
	}
	profileArn, _ := data["profileArn"].(string)
	expiresIn := intFromJSON(data, "expiresIn")
	if expiresIn == 0 {
		expiresIn = 3600
	}

	return &TokenResult{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		ProfileArn:   profileArn,
		ExpiresIn:    expiresIn,
	}, nil
}

// postJSON sends a POST request with JSON body and returns the response body.
// Returns error only on network/transport failures, NOT on HTTP 4xx/5xx.
func postJSON(url string, body interface{}, extraHeaders map[string]string) ([]byte, int, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

// postJSONStrict is like postJSON but returns an error on HTTP 4xx/5xx.
func postJSONStrict(url string, body interface{}, extraHeaders map[string]string) ([]byte, error) {
	respBody, status, err := postJSON(url, body, extraHeaders)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", status, string(respBody))
	}
	return respBody, nil
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
