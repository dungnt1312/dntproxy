package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
)

const signUpPausedMessage = "New signups are temporarily paused."

// authServiceHTTP mirrors AuthServiceClient's axios timeout (10s).
var authServiceHTTP = shared.NewSafeHTTPClient(10 * time.Second)

// authServiceClient talks to the Kiro auth service
// (kiroAuthConfig.endpoint: /oauth/token and /refreshToken).
type authServiceClient struct{}

func (authServiceClient) post(ctx context.Context, path string, body interface{}, badRequestUnexpected bool) ([]byte, error) {
	res, err := doJSON(ctx, "POST", authServiceEndpoint()+path, body, map[string]string{
		"User-Agent": kiroUserAgent(),
		"Accept":     "application/json, text/plain, */*",
	}, reqOpts{client: authServiceHTTP, maxAttempts: 4}) // axios-retry: 3 retries, network errors only
	if err != nil {
		return nil, transportError("AuthService client", err)
	}
	if res.Status >= 200 && res.Status < 300 {
		return res.Body, nil
	}
	return nil, translateAuthServiceError(res, badRequestUnexpected)
}

func translateAuthServiceError(res *httpResult, badRequestUnexpected bool) error {
	var e struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(res.Body, &e)
	if e.Message == signUpPausedMessage {
		return newKiroErr(KindSignInBlocked, "AuthService client: Sign-in temporarily not allowed")
	}
	switch {
	case res.Status >= 500:
		return newKiroErr(KindServerIssue, fmt.Sprintf("AuthService client: Bad response: HTTP %d", res.Status))
	case res.Status == 401 && !badRequestUnexpected:
		return newKiroErrUser(KindInvalidAuth, "AuthService client: "+e.Message, e.Message)
	case res.Status >= 400:
		return newKiroErrUser(KindUnexpected, fmt.Sprintf("AuthService client: HTTP %d: %s", res.Status, truncate(e.Message, 200)), e.Message)
	}
	return newKiroErr(KindUnexpected, fmt.Sprintf("AuthService client: HTTP %d: %s", res.Status, truncate(e.Message, 200)))
}

func parseSocialToken(raw []byte) (*TokenResult, error) {
	var r struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ProfileArn   string `json:"profileArn"`
		ExpiresIn    int    `json:"expiresIn"`
	}
	if err := json.Unmarshal(raw, &r); err != nil || r.AccessToken == "" {
		return nil, newKiroErr(KindUnexpected, "AuthService client: malformed token response")
	}
	if r.ExpiresIn == 0 {
		r.ExpiresIn = 3600
	}
	return &TokenResult{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		ProfileArn:   r.ProfileArn,
		ExpiresIn:    r.ExpiresIn,
	}, nil
}

// createToken exchanges a portal authorization code (social login).
func (a authServiceClient) createToken(ctx context.Context, code, codeVerifier, redirectURI string) (*TokenResult, error) {
	raw, err := a.post(ctx, "/oauth/token", map[string]interface{}{
		"code":          code,
		"code_verifier": codeVerifier,
		"redirect_uri":  redirectURI,
	}, true)
	if err != nil {
		return nil, err
	}
	return parseSocialToken(raw)
}

// refreshToken refreshes a social (Google/GitHub) session.
func (a authServiceClient) refreshToken(ctx context.Context, refreshToken string) (*TokenResult, error) {
	raw, err := a.post(ctx, "/refreshToken", map[string]interface{}{"refreshToken": refreshToken}, false)
	if err != nil {
		return nil, err
	}
	res, err := parseSocialToken(raw)
	if err != nil {
		return nil, err
	}
	if res.RefreshToken == "" {
		res.RefreshToken = refreshToken
	}
	return res, nil
}

// RefreshTokenSocial refreshes a token using the Kiro auth service.
func RefreshTokenSocial(refreshToken string) (*TokenResult, error) {
	return authServiceClient{}.refreshToken(context.Background(), refreshToken)
}
