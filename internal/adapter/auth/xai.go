package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	XAIDefaultAPIBaseURL = "https://api.x.ai/v1"
	XAIIssuer            = "https://auth.x.ai"
	XAIDiscoveryURL      = XAIIssuer + "/.well-known/openid-configuration"
	XAIClientID          = "b1a00492-073a-47ea-816f-4c329264a828"
	XAIScope             = "openid profile email offline_access grok-cli:access api:access"
	XAIRedirectURI       = "http://127.0.0.1:56121/callback"
)

type XAIAuthorizeURLParams struct {
	AuthorizationEndpoint string
	RedirectURI           string
	CodeChallenge         string
	State                 string
	Nonce                 string
}

type XAIDiscovery struct {
	AuthorizationEndpoint string
	TokenEndpoint         string
}

type XAITokenResponse struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	TokenType    string
	ExpiresIn    int
	Email        string
	Subject      string
}

func ValidateXAIEndpoint(rawURL string, field string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("xai discovery %s is empty", field)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("xai discovery %s is invalid: %w", field, err)
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("xai discovery %s must use https: %q", field, rawURL)
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host != "x.ai" && !strings.HasSuffix(host, ".x.ai") {
		return "", fmt.Errorf("xai discovery %s host %q is not on x.ai", field, host)
	}
	return rawURL, nil
}

func DiscoverXAI() (*XAIDiscovery, error) {
	req, err := http.NewRequest(http.MethodGet, XAIDiscoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("xai discovery: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xai discovery: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("xai discovery: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xai discovery failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		AuthorizationEndpoint string `json:"authorization_endpoint"`
		TokenEndpoint         string `json:"token_endpoint"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("xai discovery: parse response: %w", err)
	}

	authEndpoint, err := ValidateXAIEndpoint(payload.AuthorizationEndpoint, "authorization_endpoint")
	if err != nil {
		return nil, err
	}
	tokenEndpoint, err := ValidateXAIEndpoint(payload.TokenEndpoint, "token_endpoint")
	if err != nil {
		return nil, err
	}

	return &XAIDiscovery{AuthorizationEndpoint: authEndpoint, TokenEndpoint: tokenEndpoint}, nil
}

func BuildXAIAuthURL(params XAIAuthorizeURLParams) (string, error) {
	endpoint, err := ValidateXAIEndpoint(params.AuthorizationEndpoint, "authorization_endpoint")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(params.RedirectURI) == "" {
		return "", fmt.Errorf("xai authorize URL: redirect URI is required")
	}
	if strings.TrimSpace(params.CodeChallenge) == "" {
		return "", fmt.Errorf("xai authorize URL: code challenge is required")
	}
	if strings.TrimSpace(params.State) == "" {
		return "", fmt.Errorf("xai authorize URL: state is required")
	}
	if strings.TrimSpace(params.Nonce) == "" {
		return "", fmt.Errorf("xai authorize URL: nonce is required")
	}

	values := url.Values{
		"response_type":         {"code"},
		"client_id":             {XAIClientID},
		"redirect_uri":          {strings.TrimSpace(params.RedirectURI)},
		"scope":                 {XAIScope},
		"code_challenge":        {strings.TrimSpace(params.CodeChallenge)},
		"code_challenge_method": {"S256"},
		"state":                 {strings.TrimSpace(params.State)},
		"nonce":                 {strings.TrimSpace(params.Nonce)},
		"plan":                  {"generic"},
		"referrer":              {"dntproxy"},
	}
	return endpoint + "?" + values.Encode(), nil
}

func ExchangeXAICode(code, redirectURI, codeVerifier, tokenEndpoint string) (*XAITokenResponse, error) {
	if strings.TrimSpace(tokenEndpoint) == "" {
		discovery, err := DiscoverXAI()
		if err != nil {
			return nil, err
		}
		tokenEndpoint = discovery.TokenEndpoint
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {strings.TrimSpace(code)},
		"redirect_uri":  {strings.TrimSpace(redirectURI)},
		"client_id":     {XAIClientID},
		"code_verifier": {strings.TrimSpace(codeVerifier)},
	}
	return postXAITokenForm(tokenEndpoint, form, "")
}

func RefreshXAIToken(refreshToken, tokenEndpoint string) (*XAITokenResponse, error) {
	if strings.TrimSpace(tokenEndpoint) == "" {
		discovery, err := DiscoverXAI()
		if err != nil {
			return nil, err
		}
		tokenEndpoint = discovery.TokenEndpoint
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {XAIClientID},
		"refresh_token": {strings.TrimSpace(refreshToken)},
	}
	return postXAITokenForm(tokenEndpoint, form, refreshToken)
}

func postXAITokenForm(tokenEndpoint string, form url.Values, fallbackRefreshToken string) (*XAITokenResponse, error) {
	req, err := http.NewRequest(http.MethodPost, strings.TrimSpace(tokenEndpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("xai token request: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xai token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("xai token response: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xai token request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("xai token response: parse body: %w", err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return nil, fmt.Errorf("xai token response missing access_token")
	}
	if payload.RefreshToken == "" {
		payload.RefreshToken = fallbackRefreshToken
	}

	email, subject := extractXAIIdentity(payload.IDToken)
	return &XAITokenResponse{
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		IDToken:      strings.TrimSpace(payload.IDToken),
		TokenType:    strings.TrimSpace(payload.TokenType),
		ExpiresIn:    payload.ExpiresIn,
		Email:        email,
		Subject:      subject,
	}, nil
}

func extractXAIIdentity(token string) (email string, subject string) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", ""
	}
	payload := parts[1]
	payload += strings.Repeat("=", (4-len(payload)%4)%4)
	raw, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", ""
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return "", ""
	}
	if v, ok := claims["email"].(string); ok {
		email = strings.TrimSpace(v)
	}
	if v, ok := claims["sub"].(string); ok {
		subject = strings.TrimSpace(v)
	}
	return email, subject
}
