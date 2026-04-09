package auth

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// BuildSocialLoginURL builds a Google/GitHub social login URL via AWS Cognito.
func BuildSocialLoginURL(provider, codeChallenge, state string) (string, error) {
	idp := "Google"
	if provider == "github" {
		idp = "Github"
	}

	params := url.Values{
		"idp":                   {idp},
		"redirect_uri":          {KiroConfig.SocialRedirectURI},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
		"prompt":                {"select_account"},
	}

	return fmt.Sprintf("%s/login?%s", KiroConfig.SocialAuthURL, params.Encode()), nil
}

// ExchangeSocialCode exchanges an authorization code for tokens (social login).
func ExchangeSocialCode(code, codeVerifier string) (*TokenResult, error) {
	body := map[string]interface{}{
		"code":          code,
		"code_verifier": codeVerifier,
		"redirect_uri":  KiroConfig.SocialRedirectURI,
	}

	resp, err := postJSONStrict(KiroConfig.SocialTokenURL, body, nil)
	if err != nil {
		return nil, fmt.Errorf("exchange social code: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("parse social token response: %w", err)
	}

	accessToken, _ := data["accessToken"].(string)
	refreshToken, _ := data["refreshToken"].(string)
	profileArn, _ := data["profileArn"].(string)
	expiresIn := intFromJSON(data, "expiresIn")
	if expiresIn == 0 {
		expiresIn = 3600
	}

	return &TokenResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ProfileArn:   profileArn,
		ExpiresIn:    expiresIn,
	}, nil
}
