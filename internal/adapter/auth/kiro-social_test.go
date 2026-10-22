package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func contains(s, sub string) bool { return strings.Contains(s, sub) }
func hasPrefix(s, p string) bool  { return strings.HasPrefix(s, p) }

// newAuthService points the Kiro auth service at a local server.
func newAuthService(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Setenv("KIRO_AUTH_ENDPOINT", srv.URL)
	t.Cleanup(srv.Close)
	return srv
}

func TestStartPortalLoginBuildsKiroURL(t *testing.T) {
	t.Setenv("KIRO_AUTH_PORTAL_URL", "")
	login, err := StartPortalLogin()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(login.URL)
	if err != nil || u.Host != "app.kiro.dev" || u.Path != "/signin" {
		t.Fatalf("url = %q", login.URL)
	}
	q := u.Query()
	if q.Get("state") != login.State || q.Get("code_challenge_method") != "S256" || q.Get("redirect_from") != "KiroIDE" {
		t.Fatalf("query = %v", q)
	}
	if q.Get("redirect_uri") != "http://localhost:3128" || login.RedirectURI != "http://localhost:3128" {
		t.Fatalf("redirect = %q / %q", q.Get("redirect_uri"), login.RedirectURI)
	}
	if q.Get("code_challenge") == "" || login.CodeVerifier == "" {
		t.Fatal("PKCE material missing")
	}
}

func TestParsePortalCallback(t *testing.T) {
	ok, err := ParsePortalCallback("http://localhost:3128/oauth/callback?login_option=google&code=C&state=S", "S")
	if err != nil || ok.Code != "C" || ok.LoginOption != "google" || ok.Path != "/oauth/callback" {
		t.Fatalf("ok=%+v err=%v", ok, err)
	}
	bad := map[string]string{
		"state mismatch": "http://localhost:3128/oauth/callback?code=C&state=other",
		"missing state":  "http://localhost:3128/oauth/callback?code=C",
		"idp error":      "http://localhost:3128/oauth/callback?error=access_denied&state=S",
		"not a url":      "garbage",
	}
	for name, raw := range bad {
		if _, err := ParsePortalCallback(raw, "S"); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestExchangePortalCallbackSocial(t *testing.T) {
	var got map[string]string
	newAuthService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"accessToken":"at","refreshToken":"rt","expiresIn":3600,"profileArn":"arn:p"}`))
	})
	login := &PortalLogin{CodeVerifier: "ver", RedirectURI: "http://localhost:3128"}
	cb := &PortalCallback{LoginOption: "github", Code: "C", Path: "/oauth/callback"}

	cred, err := ExchangePortalCallback(context.Background(), cb, login, "")
	if err != nil {
		t.Fatal(err)
	}
	if got["code"] != "C" || got["code_verifier"] != "ver" ||
		got["redirect_uri"] != "http://localhost:3128/oauth/callback?login_option=github" {
		t.Fatalf("token request = %v", got)
	}
	if cred.Provider != ProviderGithub || cred.AuthMethod() != "github" || cred.ProfileArn != "arn:p" || cred.ProviderLabel() != "GitHub" {
		t.Fatalf("cred = %+v", cred)
	}
	// Social connections carry no OIDC client, so refresh takes the auth-service path.
	if pd := cred.ProviderData(); pd["clientId"] != nil || pd["profileArn"] != "arn:p" {
		t.Fatalf("provider data = %v", pd)
	}
}

func TestExchangePortalCallbackFallsBackToChosenProvider(t *testing.T) {
	newAuthService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"accessToken":"at","refreshToken":"rt","expiresIn":3600}`))
	})
	login := &PortalLogin{CodeVerifier: "v", RedirectURI: "http://localhost:3128"}
	// A bare code (no login_option in the URL) uses the provider picked in the UI.
	cred, err := ExchangePortalCallback(context.Background(), &PortalCallback{Code: "C"}, login, "google")
	if err != nil || cred.Provider != ProviderGoogle {
		t.Fatalf("cred=%+v err=%v", cred, err)
	}
}

func TestExchangePortalCallbackRejectsIdCOptions(t *testing.T) {
	login := &PortalLogin{RedirectURI: "http://localhost:3128"}
	for _, opt := range []string{"builderid", "awsidc", "internal", "external_idp"} {
		_, err := ExchangePortalCallback(context.Background(), &PortalCallback{LoginOption: opt, Code: "C"}, login, "")
		if ErrorKind(err) != KindInvalidUserInput {
			t.Errorf("%s: err = %v", opt, err)
		}
	}
	if _, err := ExchangePortalCallback(context.Background(), &PortalCallback{LoginOption: "weird", Code: "C"}, login, ""); err == nil {
		t.Error("unknown login_option must fail")
	}
}

func TestAuthServiceErrorClassification(t *testing.T) {
	mk := func(status int, body string) *httpResult { return &httpResult{Status: status, Body: []byte(body)} }
	if k := ErrorKind(translateAuthServiceError(mk(401, `{"message":"expired"}`), false)); k != KindInvalidAuth {
		t.Errorf("refresh 401 = %s, want INVALID_AUTH", k)
	}
	if k := ErrorKind(translateAuthServiceError(mk(401, `{"message":"bad code"}`), true)); k != KindUnexpected {
		t.Errorf("createToken 401 = %s, want UNEXPECTED_ISSUE", k)
	}
	if k := ErrorKind(translateAuthServiceError(mk(400, `{"message":"New signups are temporarily paused."}`), true)); k != KindSignInBlocked {
		t.Errorf("paused = %s", k)
	}
	if k := ErrorKind(translateAuthServiceError(mk(502, ``), false)); k != KindServerIssue {
		t.Errorf("502 = %s", k)
	}
}

func TestAuthServiceRetriesNetworkErrorsButNotServerErrors(t *testing.T) {
	calls := 0
	newAuthService(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(500)
	})
	if _, err := RefreshTokenSocial("rt"); ErrorKind(err) != KindServerIssue || calls != 1 {
		t.Fatalf("err=%v calls=%d (axios-retry does not retry 5xx)", err, calls)
	}
}
