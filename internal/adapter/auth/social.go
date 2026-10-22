package auth

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Social login (Google / GitHub) goes through Kiro's unified auth portal
// (https://app.kiro.dev/signin). The portal authenticates the user and
// redirects the browser to http://localhost:<port>/oauth/callback with a
// one-time code, which is then exchanged at the Kiro auth service.
//
// Kiro IDE catches that redirect with a loopback listener. dntproxy's dashboard
// completes the flow from the pasted callback URL instead, so no listener is
// opened here: the browser lands on a "connection refused" page whose address
// bar holds the URL to paste. That also works when dntproxy runs on another
// machine than the browser.

// PortalLogin is one pending portal sign-in.
type PortalLogin struct {
	URL          string
	State        string
	CodeVerifier string
	RedirectURI  string // http://localhost:<port>, no path
}

// StartPortalLogin builds the portal sign-in URL (PortalAuthProvider.buildPortalUrl).
func StartPortalLogin() (*PortalLogin, error) {
	verifier, challenge, err := pkcePair()
	if err != nil {
		return nil, wrapKiroErr(KindUnexpected, "PortalAuth: failed to generate PKCE", err)
	}
	state := uuid.New().String()
	redirect := "http://localhost:" + strconv.Itoa(PortalCallbackPorts[0])
	query := orderedQuery(
		"state", state,
		"code_challenge", challenge,
		"code_challenge_method", "S256",
		"redirect_uri", redirect,
		"redirect_from", "KiroIDE",
	)
	return &PortalLogin{
		URL:          PortalURL() + "/signin?" + query,
		State:        state,
		CodeVerifier: verifier,
		RedirectURI:  redirect,
	}, nil
}

// PortalCallback is a parsed portal redirect (PortalAuthServer.handleCallback).
type PortalCallback struct {
	LoginOption string
	Code        string
	State       string
	Path        string
	IssuerURL   string
	IdCRegion   string
}

// ParsePortalCallback parses and validates a pasted callback URL against the
// expected state. Both the portal's http://localhost:<port>/oauth/callback form
// and the legacy kiro://kiro.kiroAgent/authenticate-success form are accepted.
func ParsePortalCallback(raw, expectedState string) (*PortalCallback, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" {
		return nil, newKiroErrUser(KindInvalidUserInput, "invalid callback URL",
			"The callback URL is not valid. Paste the full URL from the browser address bar.")
	}
	q := u.Query()
	if e := q.Get("error"); e != "" {
		desc := firstNonEmpty(q.Get("error_description"), e)
		if e == "access_denied" {
			return nil, newKiroErrUser(KindInvalidAuth, "Access denied through identity provider",
				"Authentication failed: The identity provider denied access to Kiro. Please ensure you grant all required permissions.")
		}
		return nil, newKiroErrUser(KindInvalidAuth, "Failure from identity provider: "+desc,
			"Authentication failed: The identity provider reported an error: "+desc)
	}
	state := q.Get("state")
	if state == "" {
		return nil, newKiroErrUser(KindInvalidAuth, "AuthSSOServer: missing state",
			"Authentication failed: Please try signing in again.")
	}
	if state != expectedState {
		return nil, newKiroErrUser(KindInvalidAuth, "AuthSSOServer: invalid state",
			"Authentication failed: Please try signing in again.")
	}
	return &PortalCallback{
		LoginOption: q.Get("login_option"),
		Code:        q.Get("code"),
		State:       state,
		Path:        u.Path,
		IssuerURL:   q.Get("issuer_url"),
		IdCRegion:   q.Get("idc_region"),
	}, nil
}

// socialProvider maps a login_option to a Kiro social provider.
func socialProvider(loginOption string) string {
	switch loginOption {
	case "google":
		return ProviderGoogle
	case "github":
		return ProviderGithub
	}
	return ""
}

// ExchangePortalCallback turns a validated callback into credentials. Only
// social logins finish here; the IdC options need a second browser step, which
// the dashboard's Builder ID / IAM Identity Center tabs perform.
func ExchangePortalCallback(ctx context.Context, cb *PortalCallback, login *PortalLogin, fallbackOption string) (*KiroCredential, error) {
	option := firstNonEmpty(cb.LoginOption, fallbackOption)
	switch option {
	case "google", "github":
	case "builderid", "awsidc", "internal", "external_idp":
		return nil, newKiroErrUser(KindInvalidUserInput, "portal returned login_option="+option,
			"This sign-in is an AWS IAM Identity Center / Builder ID login. Use the matching tab to connect it.")
	default:
		return nil, newKiroErr(KindUnexpected, "Unknown login_option: "+option)
	}
	if cb.Code == "" {
		return nil, newKiroErrUser(KindInvalidAuth, "AuthSSOServer: missing code",
			"Authentication failed: Please try signing in again.")
	}
	path := cb.Path
	if path != "/oauth/callback" && path != "/signin/callback" {
		path = "/oauth/callback"
	}
	fullRedirect := login.RedirectURI + path + "?login_option=" + option

	tok, err := authServiceClient{}.createToken(ctx, cb.Code, login.CodeVerifier, fullRedirect)
	if err != nil {
		return nil, err
	}
	return &KiroCredential{
		Provider:     socialProvider(option),
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresIn:    tok.ExpiresIn,
		ProfileArn:   tok.ProfileArn,
	}, nil
}
