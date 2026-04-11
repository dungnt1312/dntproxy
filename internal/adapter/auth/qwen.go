package auth

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// QwenConfig holds Qwen OAuth constants.
var QwenConfig = struct {
	DeviceCodeURL string
	TokenURL      string
	APIBaseURL    string
	ClientID      string
	Scopes        string
}{
	DeviceCodeURL: "https://chat.qwen.ai/api/v1/oauth2/device/code",
	TokenURL:      "https://chat.qwen.ai/api/v1/oauth2/token",
	APIBaseURL:    "https://portal.qwen.ai/v1",
	ClientID:      "f0304373b74a44d2b584a3fb70ca9e56",
	Scopes:        "openid profile email model.completion",
}

// StartQwenDeviceAuth initiates the Qwen device code authorization flow.
// Returns a device code, user code, and verification URI for the user.
func StartQwenDeviceAuth(codeChallenge string) (*DeviceAuthResult, error) {
	// Qwen uses standard OAuth2 form-encoded POST for device code
	formData := url.Values{
		"client_id":             {QwenConfig.ClientID},
		"scope":                 {QwenConfig.Scopes},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}

	resp, status, err := postFormURL(QwenConfig.DeviceCodeURL, formData)
	if err != nil {
		return nil, fmt.Errorf("qwen device code request: %w", err)
	}
	if status >= 400 {
		return nil, fmt.Errorf("qwen device code HTTP %d: %s", status, string(resp))
	}

	// Qwen returns standard OAuth2 snake_case fields
	var rawResp struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	}
	if err := json.Unmarshal(resp, &rawResp); err != nil {
		return nil, fmt.Errorf("parse qwen device code response: %w", err)
	}

	if rawResp.Interval == 0 {
		rawResp.Interval = 5
	}

	return &DeviceAuthResult{
		DeviceCode:              rawResp.DeviceCode,
		UserCode:                rawResp.UserCode,
		VerificationURI:         rawResp.VerificationURI,
		VerificationURIComplete: rawResp.VerificationURIComplete,
		ExpiresIn:               rawResp.ExpiresIn,
		Interval:                rawResp.Interval,
	}, nil
}

// PollQwenDeviceToken polls the Qwen token endpoint with the device code.
// Returns pending=true while user hasn't authorized yet.
func PollQwenDeviceToken(deviceCode, codeVerifier string) (*PollResult, error) {
	formData := url.Values{
		"grant_type":    {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":     {QwenConfig.ClientID},
		"device_code":   {deviceCode},
		"code_verifier": {codeVerifier},
	}

	respBytes, _, err := postFormURL(QwenConfig.TokenURL, formData)
	if err != nil {
		return nil, fmt.Errorf("qwen poll token: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(respBytes, &data); err != nil {
		return nil, fmt.Errorf("parse qwen poll response: %w", err)
	}

	// Check for error (standard OAuth2 error response)
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

	// Success — extract tokens (OAuth2 snake_case)
	accessToken, _ := data["access_token"].(string)
	refreshToken, _ := data["refresh_token"].(string)
	expiresIn := intFromJSON(data, "expires_in")
	tokenType, _ := data["token_type"].(string)

	return &PollResult{
		Success: true,
		Tokens: &TokenResult{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    expiresIn,
			TokenType:    tokenType,
			AuthMethod:   "qwen-oauth",
		},
	}, nil
}

// RefreshQwenToken refreshes a Qwen OAuth token.
func RefreshQwenToken(refreshToken string) (*TokenResult, error) {
	formData := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {QwenConfig.ClientID},
		"refresh_token": {refreshToken},
	}

	respBytes, status, err := postFormURL(QwenConfig.TokenURL, formData)
	if err != nil {
		return nil, fmt.Errorf("qwen refresh token: %w", err)
	}
	if status >= 400 {
		return nil, fmt.Errorf("qwen refresh HTTP %d: %s", status, string(respBytes))
	}

	var data map[string]interface{}
	if err := json.Unmarshal(respBytes, &data); err != nil {
		return nil, fmt.Errorf("parse qwen refresh response: %w", err)
	}

	accessToken, _ := data["access_token"].(string)
	newRefresh, _ := data["refresh_token"].(string)
	if newRefresh == "" {
		newRefresh = refreshToken
	}
	expiresIn := intFromJSON(data, "expires_in")
	if expiresIn == 0 {
		expiresIn = 3600
	}

	return &TokenResult{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		ExpiresIn:    expiresIn,
		AuthMethod:   "qwen-oauth",
	}, nil
}

// postFormURL sends a POST request with form-encoded body.
func postFormURL(targetURL string, formData url.Values) ([]byte, int, error) {
	body := strings.NewReader(formData.Encode())

	resp, err := httpClient.Post(targetURL, "application/x-www-form-urlencoded", body)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody := make([]byte, 0)
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			respBody = append(respBody, buf[:n]...)
		}
		if readErr != nil {
			break
		}
	}

	return respBody, resp.StatusCode, nil
}
