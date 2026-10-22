package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// IdCLoginOptions selects the IdC provider flavour (Kiro's IDCAuthProvider).
type IdCLoginOptions struct {
	Provider string // ProviderBuilderID | ProviderEnterprise | ProviderInternal
	StartURL string // required for ProviderEnterprise
	Region   string // defaults to us-east-1
}

// IdCLogin is one in-flight browser sign-in: RegisterClient → /authorize
// (PKCE) → loopback callback → CreateToken(authorization_code).
type IdCLogin struct {
	AuthorizeURL string
	UserCode     string // short display code derived from the state (informational)
	ExpiresIn    int    // seconds until the flow is abandoned

	opts         IdCLoginOptions
	startURL     string
	region       string
	reg          *ClientRegistration
	codeVerifier string
	redirectURI  string
	state        string
	server       *callbackServer

	mu    sync.Mutex
	final bool
	cred  *KiroCredential
	err   error
}

// IdCStartURL resolves the start URL for a provider.
func IdCStartURL(opts IdCLoginOptions) string {
	switch opts.Provider {
	case ProviderEnterprise:
		return strings.TrimSpace(opts.StartURL)
	case ProviderBuilderID:
		return BuilderIDStartURL
	default:
		return InternalSSOStartURL
	}
}

func validateStartURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return newKiroErrUser(KindInvalidUserInput, "Invalid start URL provided",
			"The start URL you entered is invalid. Please verify the URL format and try again.")
	}
	return nil
}

// pkcePair mirrors Kiro: verifier = base64url(32 random bytes), S256 challenge.
func pkcePair() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// StartIdCLogin registers the OIDC client, opens the loopback listener and
// builds the authorize URL. The caller sends the user to AuthorizeURL and then
// polls Poll until it completes.
func StartIdCLogin(ctx context.Context, opts IdCLoginOptions) (*IdCLogin, error) {
	startURL := IdCStartURL(opts)
	if err := validateStartURL(startURL); err != nil {
		return nil, err
	}
	region := opts.Region
	if region == "" {
		region = DefaultIdCRegion
	}
	sso, err := newSSOClient(region)
	if err != nil {
		return nil, err
	}
	reg, err := sso.registerClient(ctx, startURL, opts.Provider == ProviderEnterprise)
	if err != nil {
		return nil, err
	}

	state := uuid.New().String()
	srv, err := startCallbackServer(state, 0, loginFlowTimeout)
	if err != nil {
		return nil, err
	}
	verifier, challenge, err := pkcePair()
	if err != nil {
		srv.Close()
		return nil, wrapKiroErr(KindUnexpected, "IdCAuth: failed to generate PKCE", err)
	}
	redirectURI := srv.RedirectURI()

	query := orderedQuery(
		"response_type", "code",
		"client_id", reg.ClientID,
		"redirect_uri", redirectURI,
		"scopes", strings.Join(IdCScopes, ","),
		"state", state,
		"code_challenge", challenge,
		"code_challenge_method", "S256",
	)
	return &IdCLogin{
		AuthorizeURL: oidcBaseURL(sso.region) + "/authorize?" + query,
		UserCode:     shortCode(state),
		ExpiresIn:    int(loginFlowTimeout / time.Second),
		opts:         opts,
		startURL:     startURL,
		region:       sso.region,
		reg:          reg,
		codeVerifier: verifier,
		redirectURI:  redirectURI,
		state:        state,
		server:       srv,
	}, nil
}

func shortCode(state string) string {
	s := strings.ToUpper(strings.ReplaceAll(state, "-", ""))
	if len(s) < 8 {
		return s
	}
	return s[:4] + "-" + s[4:8]
}

// Poll reports whether the sign-in has finished. done=false means still
// waiting for the browser. After done=true the same result is returned again.
func (l *IdCLogin) Poll(ctx context.Context) (cred *KiroCredential, done bool, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.final {
		return l.cred, true, l.err
	}
	select {
	case out := <-l.server.result:
		l.finish(ctx, out)
		return l.cred, true, l.err
	default:
		return nil, false, nil
	}
}

// SubmitCallback completes the flow from a pasted callback URL, for browsers
// that cannot reach the loopback listener.
func (l *IdCLogin) SubmitCallback(ctx context.Context, raw string) (*KiroCredential, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.final {
		return l.cred, l.err
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, newKiroErrUser(KindInvalidUserInput, "invalid callback URL", "The callback URL is not valid.")
	}
	code, vErr := validateIdCCallback(u.Query(), l.state)
	l.server.Close()
	l.finish(ctx, callbackOutcome{code: code, err: vErr})
	return l.cred, l.err
}

// finish exchanges the code (once) and records the final outcome. Caller holds l.mu.
func (l *IdCLogin) finish(ctx context.Context, out callbackOutcome) {
	l.final = true
	if out.err != nil {
		l.err = out.err
		return
	}
	sso, err := newSSOClient(l.region)
	if err != nil {
		l.err = err
		return
	}
	tok, err := sso.createToken(ctx, ssoTokenInput{
		ClientID:     l.reg.ClientID,
		ClientSecret: l.reg.ClientSecret,
		GrantType:    "authorization_code",
		RedirectURI:  l.redirectURI,
		Code:         out.code,
		CodeVerifier: l.codeVerifier,
	})
	if err != nil {
		l.err = err
		return
	}
	cred := &KiroCredential{
		Provider:              l.opts.Provider,
		AccessToken:           tok.AccessToken,
		RefreshToken:          tok.RefreshToken,
		ExpiresIn:             tok.ExpiresIn,
		Region:                l.region,
		StartURL:              l.startURL,
		ClientID:              l.reg.ClientID,
		ClientSecret:          l.reg.ClientSecret,
		ClientSecretExpiresAt: l.reg.ExpiresAt,
	}
	cred.ProfileArn, _ = ResolveProfileArn(ctx, cred)
	l.cred = cred
}

// Cancel aborts a pending sign-in and frees the listener.
func (l *IdCLogin) Cancel() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.final {
		l.final = true
		l.err = newKiroErr(KindCanceled, "user cancellation")
	}
	l.server.Cancel()
	l.server.Close()
}
