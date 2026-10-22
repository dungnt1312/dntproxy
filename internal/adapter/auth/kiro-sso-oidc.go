package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ssoUserAgent approximates the AWS SDK user agent Kiro sends
// (SSOOIDC client created with customUserAgent "KiroIDE").
const ssoUserAgent = "aws-sdk-js/3.0.0 KiroIDE"

const ssoUserMsg = "Authentication with IAM Identity Center failed. Please verify you have an active subscription. " +
	"For more information: https://kiro.dev/docs/troubleshooting/#aws-iam-identity-center-issues"

// oidcBaseURL builds the SSO OIDC endpoint for a region. A var so tests can
// point it at a local server.
var oidcBaseURL = func(region string) string {
	return "https://oidc." + region + ".amazonaws.com"
}

// OverrideOIDCBaseURL redirects every SSO OIDC call to base and returns a
// restore func. It exists so handler tests in other packages can run the login
// flows against a local fake; production code never calls it.
func OverrideOIDCBaseURL(base string) (restore func()) {
	old := oidcBaseURL
	oidcBaseURL = func(string) string { return base }
	return func() { oidcBaseURL = old }
}

// ClientRegistration is the result of SSO OIDC RegisterClient.
type ClientRegistration struct {
	ClientID     string
	ClientSecret string
	ExpiresAt    string // RFC3339, derived from clientSecretExpiresAt
}

type ssoClient struct{ region string }

// ssoTokenInput mirrors the CreateToken request fields Kiro uses.
type ssoTokenInput struct {
	ClientID     string
	ClientSecret string
	GrantType    string
	Code         string
	CodeVerifier string
	RedirectURI  string
	RefreshToken string
}

func newSSOClient(region string) (*ssoClient, error) {
	normalized, err := NormalizeAWSRegion(region)
	if err != nil {
		return nil, wrapKiroErr(KindInvalidUserInput, "SSOOIDC client: invalid region", err)
	}
	return &ssoClient{region: normalized}, nil
}

func retryableOIDCStatus(status int) bool {
	return status == 429 || status == 500 || status == 502 || status == 503 || status == 504
}

func (c *ssoClient) post(ctx context.Context, path string, body interface{}, userInput bool) ([]byte, error) {
	res, err := doJSON(ctx, "POST", oidcBaseURL(c.region)+path, body, map[string]string{
		"Accept":           "application/json",
		"User-Agent":       ssoUserAgent,
		"X-Amz-User-Agent": ssoUserAgent,
	}, reqOpts{client: httpClient, maxAttempts: 4, retryTimeouts: true, retryStatus: retryableOIDCStatus})
	if err != nil {
		return nil, transportError("SSOOIDC client", err)
	}
	if res.Status >= 200 && res.Status < 300 {
		return res.Body, nil
	}
	return nil, translateSSOError(res, userInput)
}

// translateSSOError maps SSO OIDC failures to Kiro's error classes.
func translateSSOError(res *httpResult, userInput bool) error {
	var e struct {
		Error   string `json:"error"`
		Desc    string `json:"error_description"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(res.Body, &e)
	code := e.Error
	if code == "" && res.Header != nil {
		code = res.Header.Get("X-Amzn-Errortype")
	}
	if i := strings.IndexAny(code, ":#"); i >= 0 {
		code = code[:i]
	}
	norm := strings.NewReplacer("_", "", "exception", "").Replace(strings.ToLower(code))
	detail := firstNonEmpty(e.Desc, e.Message)
	suffix := ""
	if code != "" {
		suffix = " (" + code + ")"
	}

	switch norm {
	case "invalidgrant":
		return newKiroErrUser(KindInvalidSSOAuth, "SSOOIDC client: Invalid grant"+suffix, ssoUserMsg)
	case "accessdenied":
		return newKiroErrUser(KindInvalidSSOAuth, "SSOOIDC client: Access denied"+suffix, ssoUserMsg)
	case "expiredtoken":
		return newKiroErrUser(KindInvalidSSOAuth, "SSOOIDC client: Expired token"+suffix, ssoUserMsg)
	case "invalidclient":
		return newKiroErrUser(KindInvalidSSOAuth, "SSOOIDC client: Invalid client"+suffix, ssoUserMsg)
	case "unauthorizedclient":
		return newKiroErrUser(KindInvalidSSOAuth, "SSOOIDC client: Unauthorized client"+suffix, ssoUserMsg)
	case "internalserver", "internalservererror":
		return newKiroErr(KindServerIssue, "SSOOIDC client: Internal server error")
	case "slowdown":
		return newKiroErr(KindServerIssue, "SSOOIDC client: Throttling")
	case "invalidrequest":
		if userInput && strings.Contains(strings.ToLower(detail), "invalid start url provided") {
			return newKiroErrUser(KindInvalidUserInput, "Invalid start URL provided",
				"The start URL you entered is invalid. Please verify the URL format and try again.")
		}
		return newKiroErr(KindUnexpected, "SSOOIDC client: Unexpected InvalidRequestException: "+truncate(detail, 200))
	}
	if res.Status >= 500 {
		return newKiroErr(KindServerIssue, fmt.Sprintf("SSOOIDC client: Internal server error (HTTP %d)", res.Status))
	}
	return newKiroErr(KindUnexpected, fmt.Sprintf("SSOOIDC client: Unexpected issue (HTTP %d%s): %s",
		res.Status, suffix, truncate(detail, 200)))
}

func (c *ssoClient) registerClient(ctx context.Context, startURL string, userInput bool) (*ClientRegistration, error) {
	raw, err := c.post(ctx, "/client/register", map[string]interface{}{
		"clientName":   idcClientName,
		"clientType":   "public",
		"scopes":       IdCScopes,
		"grantTypes":   []string{"authorization_code", "refresh_token"},
		"redirectUris": []string{idcRedirectURI},
		"issuerUrl":    startURL,
	}, userInput)
	if err != nil {
		return nil, err
	}
	var r struct {
		ClientID              string `json:"clientId"`
		ClientSecret          string `json:"clientSecret"`
		ClientSecretExpiresAt int64  `json:"clientSecretExpiresAt"`
	}
	if err := json.Unmarshal(raw, &r); err != nil || r.ClientID == "" || r.ClientSecret == "" {
		return nil, newKiroErr(KindUnexpected, "SSOOIDC client: malformed RegisterClient response")
	}
	reg := &ClientRegistration{ClientID: r.ClientID, ClientSecret: r.ClientSecret}
	if r.ClientSecretExpiresAt > 0 {
		reg.ExpiresAt = time.Unix(r.ClientSecretExpiresAt, 0).UTC().Format(time.RFC3339)
	}
	return reg, nil
}

func (c *ssoClient) createToken(ctx context.Context, in ssoTokenInput) (*TokenResult, error) {
	body := map[string]interface{}{
		"clientId":     in.ClientID,
		"clientSecret": in.ClientSecret,
		"grantType":    in.GrantType,
	}
	if in.Code != "" {
		body["code"] = in.Code
	}
	if in.CodeVerifier != "" {
		body["codeVerifier"] = in.CodeVerifier
	}
	if in.RedirectURI != "" {
		body["redirectUri"] = in.RedirectURI
	}
	if in.RefreshToken != "" {
		body["refreshToken"] = in.RefreshToken
	}
	raw, err := c.post(ctx, "/token", body, false)
	if err != nil {
		return nil, err
	}
	var r struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
		TokenType    string `json:"tokenType"`
	}
	if err := json.Unmarshal(raw, &r); err != nil || r.AccessToken == "" {
		return nil, newKiroErr(KindUnexpected, "SSOOIDC client: malformed CreateToken response")
	}
	return &TokenResult{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		ExpiresIn:    r.ExpiresIn,
		TokenType:    r.TokenType,
	}, nil
}

// refreshTokenSSO runs CreateToken(grant=refresh_token). The stored refresh
// token is kept when the response omits a new one.
func refreshTokenSSO(ctx context.Context, refreshToken, clientID, clientSecret, region string) (*TokenResult, error) {
	client, err := newSSOClient(region)
	if err != nil {
		return nil, err
	}
	res, err := client.createToken(ctx, ssoTokenInput{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
	})
	if err != nil {
		return nil, err
	}
	if res.RefreshToken == "" {
		res.RefreshToken = refreshToken
	}
	return res, nil
}

// RefreshTokenSSO refreshes a token using AWS SSO OIDC (Builder ID / IDC).
func RefreshTokenSSO(refreshToken, clientID, clientSecret, region string) (*TokenResult, error) {
	return refreshTokenSSO(context.Background(), refreshToken, clientID, clientSecret, region)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
