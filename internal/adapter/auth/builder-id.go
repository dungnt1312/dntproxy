package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Device-code login (RFC 8628) against AWS SSO OIDC.
//
// Kiro IDE itself signs in with authorization-code + PKCE (see kiro-idc-login.go).
// Device code is kept as the fallback for browsers that cannot reach a loopback
// listener on the dntproxy host (dntproxy on another machine, behind a proxy or
// inside a container). Both flows yield the same clientId/clientSecret pair and
// are refreshed through the same CreateToken(refresh_token) call.

// KiroConfig holds the constants of the legacy device-code registration.
var KiroConfig = struct {
	StartURL   string
	ClientName string
	ClientType string
	Scopes     []string
	GrantTypes []string
	IssuerURL  string
}{
	StartURL:   BuilderIDStartURL,
	ClientName: "kiro-oauth-client",
	ClientType: "public",
	Scopes:     []string{"codewhisperer:completions", "codewhisperer:analysis", "codewhisperer:conversations"},
	GrantTypes: []string{"urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
	IssuerURL:  "https://identitycenter.amazonaws.com/ssoins-722374e8c3c8e6c6",
}

// RegisterClientResult holds the result of a device-code OIDC client registration.
type RegisterClientResult struct {
	ClientID              string `json:"clientId"`
	ClientSecret          string `json:"clientSecret"`
	ClientSecretExpiresAt int64  `json:"clientSecretExpiresAt"`
}

// registerDeviceClient registers an OIDC client for the device-code grant.
func registerDeviceClient(region string) (*RegisterClientResult, error) {
	normalized, err := NormalizeAWSRegion(region)
	if err != nil {
		return nil, err
	}
	endpoint := oidcBaseURL(normalized) + "/client/register"

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

// RegisterClient registers a device-code OIDC client with AWS SSO.
func RegisterClient(region string) (*RegisterClientResult, error) {
	return registerDeviceClient(region)
}

// StartDeviceAuthorization begins the device code flow.
func StartDeviceAuthorization(clientID, clientSecret, startURL, region string) (*DeviceAuthResult, error) {
	normalized, err := NormalizeAWSRegion(region)
	if err != nil {
		return nil, err
	}
	endpoint := oidcBaseURL(normalized) + "/device_authorization"

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
	normalized, err := NormalizeAWSRegion(region)
	if err != nil {
		return nil, err
	}
	endpoint := oidcBaseURL(normalized) + "/token"

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
