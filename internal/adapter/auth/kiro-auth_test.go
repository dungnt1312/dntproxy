package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func init() { retryBaseDelay = 0 }

// withOIDC points the SSO OIDC client at a local server for one test.
func withOIDC(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	old := oidcBaseURL
	oidcBaseURL = func(string) string { return srv.URL }
	t.Cleanup(func() { oidcBaseURL = old; srv.Close() })
	return srv
}

func TestClientIDHashMatchesKiro(t *testing.T) {
	// Values computed with Node: sha1(JSON.stringify({startUrl})).
	cases := map[string]string{
		"https://view.awsapps.com/start":           "e909a0580879b06ece1202964fbe9dda95ea4ce3",
		"https://amzn.awsapps.com/start":           "06df7ec90decadd0c5dd4ed2f245b2a59d1d8826",
		"https://acme.awsapps.com/start?x=<y>&z=1": "53337ac27ee81fa764c5d7ac0905c1b7143e2460", // JSON.stringify leaves <>& unescaped
	}
	for in, want := range cases {
		if got := ClientIDHash(in); got != want {
			t.Errorf("ClientIDHash(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestIdCScopesMatchKiro(t *testing.T) {
	want := []string{
		"codewhisperer:completions", "codewhisperer:analysis", "codewhisperer:conversations",
		"codewhisperer:transformations", "codewhisperer:taskassist",
	}
	if strings.Join(IdCScopes, ",") != strings.Join(want, ",") {
		t.Fatalf("IdCScopes = %v", IdCScopes)
	}
}

func TestValidateIdCCallback(t *testing.T) {
	q := func(s string) url.Values { v, _ := url.ParseQuery(s); return v }
	if code, err := validateIdCCallback(q("code=abc&state=s1"), "s1"); err != nil || code != "abc" {
		t.Fatalf("valid: %q %v", code, err)
	}
	cases := []struct{ name, query, state, wantMsg string }{
		{"missing code", "state=s1", "s1", "missing code"},
		{"missing state", "code=abc", "s1", "missing state"},
		{"wrong state", "code=abc&state=zzz", "s1", "invalid state"},
		{"denied", "error=access_denied&error_description=nope", "s1", "denied access"},
		{"idp failure", "error=server_error&error_description=boom", "s1", "boom"},
	}
	for _, tc := range cases {
		_, err := validateIdCCallback(q(tc.query), tc.state)
		if err == nil || !strings.Contains(strings.ToLower(UserMessage(err)+err.Error()), tc.wantMsg) {
			t.Errorf("%s: err=%v, want containing %q", tc.name, err, tc.wantMsg)
		}
		if err != nil && !IsBadAuthIssue(err) {
			t.Errorf("%s: should be a bad-auth issue", tc.name)
		}
	}
	// An empty expected state skips state validation (Kiro: `if (this.state)`).
	if _, err := validateIdCCallback(q("code=abc"), ""); err != nil {
		t.Fatalf("no expected state: %v", err)
	}
}

func TestTranslateSSOError(t *testing.T) {
	mk := func(status int, body string) *httpResult { return &httpResult{Status: status, Body: []byte(body)} }
	cases := []struct {
		name string
		res  *httpResult
		user bool
		kind ErrKind
	}{
		{"invalid grant", mk(400, `{"error":"invalid_grant"}`), false, KindInvalidSSOAuth},
		{"access denied", mk(400, `{"error":"access_denied"}`), false, KindInvalidSSOAuth},
		{"expired", mk(400, `{"error":"expired_token"}`), false, KindInvalidSSOAuth},
		{"invalid client", mk(401, `{"error":"invalid_client"}`), false, KindInvalidSSOAuth},
		{"unauthorized client", mk(400, `{"error":"unauthorized_client"}`), false, KindInvalidSSOAuth},
		{"server", mk(500, `{"error":"InternalServerException"}`), false, KindServerIssue},
		{"slow down", mk(400, `{"error":"slow_down"}`), false, KindServerIssue},
		{"bad start url", mk(400, `{"error":"invalid_request","error_description":"Invalid start URL provided"}`), true, KindInvalidUserInput},
		{"bad request w/o user input", mk(400, `{"error":"invalid_request","error_description":"Invalid start URL provided"}`), false, KindUnexpected},
		{"unknown 5xx", mk(503, `oops`), false, KindServerIssue},
		{"unknown 4xx", mk(418, `{}`), false, KindUnexpected},
	}
	for _, tc := range cases {
		if got := ErrorKind(translateSSOError(tc.res, tc.user)); got != tc.kind {
			t.Errorf("%s: kind = %s, want %s", tc.name, got, tc.kind)
		}
	}
}

func TestRegisterClientSendsKiroPayload(t *testing.T) {
	var got map[string]interface{}
	withOIDC(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/client/register" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		exp := time.Now().Add(90 * 24 * time.Hour).Unix()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"clientId": "cid", "clientSecret": "sec", "clientSecretExpiresAt": exp,
		})
	}))

	c, _ := newSSOClient("us-west-1")
	reg, err := c.registerClient(context.Background(), "https://acme.awsapps.com/start", true)
	if err != nil {
		t.Fatal(err)
	}
	if reg.ClientID != "cid" || reg.ExpiresAt == "" {
		t.Fatalf("reg = %+v", reg)
	}
	if got["clientName"] != "Kiro IDE" || got["clientType"] != "public" || got["issuerUrl"] != "https://acme.awsapps.com/start" {
		t.Fatalf("payload = %v", got)
	}
	if gt, _ := got["grantTypes"].([]interface{}); len(gt) != 2 || gt[0] != "authorization_code" || gt[1] != "refresh_token" {
		t.Fatalf("grantTypes = %v", got["grantTypes"])
	}
	if ru, _ := got["redirectUris"].([]interface{}); len(ru) != 1 || ru[0] != "http://127.0.0.1/oauth/callback" {
		t.Fatalf("redirectUris = %v", got["redirectUris"])
	}
	if sc, _ := got["scopes"].([]interface{}); len(sc) != 5 {
		t.Fatalf("scopes = %v", got["scopes"])
	}
}

func TestSSOClientRetriesServerErrors(t *testing.T) {
	var hits atomic.Int32
	withOIDC(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) < 3 {
			w.WriteHeader(500)
			_, _ = io.WriteString(w, `{"error":"InternalServerException"}`)
			return
		}
		_, _ = io.WriteString(w, `{"accessToken":"at","refreshToken":"rt2","expiresIn":3600}`)
	}))
	res, err := refreshTokenSSO(context.Background(), "rt", "cid", "sec", "us-east-1")
	if err != nil || res.AccessToken != "at" || hits.Load() != 3 {
		t.Fatalf("res=%+v err=%v hits=%d", res, err, hits.Load())
	}
}

func TestSSOClientDoesNotRetryClientErrors(t *testing.T) {
	var hits atomic.Int32
	withOIDC(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(400)
		_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
	}))
	_, err := refreshTokenSSO(context.Background(), "rt", "cid", "sec", "us-east-1")
	if !IsBadAuthIssue(err) || hits.Load() != 1 {
		t.Fatalf("err=%v hits=%d", err, hits.Load())
	}
}

func TestRefreshKeepsStoredRefreshTokenWhenOmitted(t *testing.T) {
	withOIDC(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"accessToken":"at","expiresIn":1800}`)
	}))
	res, err := refreshTokenSSO(context.Background(), "old-rt", "cid", "sec", "us-east-1")
	if err != nil || res.RefreshToken != "old-rt" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestRefreshTokenSSORejectsHostInjectionTyped(t *testing.T) {
	_, err := refreshTokenSSO(context.Background(), "rt", "cid", "sec", "evil.com/steal")
	if ErrorKind(err) != KindInvalidUserInput {
		t.Fatalf("err = %v", err)
	}
}

func TestErrorTextTriggersRevocationSideline(t *testing.T) {
	err := newKiroErr(KindInvalidSSOAuth, "SSOOIDC client: Invalid grant")
	if !IsDeadRefreshError(err) {
		t.Fatal("bad auth must count as a dead refresh")
	}
	if strings.Contains(UserMessage(newKiroErr(KindInvalidAuth, "x")), "re-authenticate") {
		t.Fatal("UserMessage must not leak the internal marker")
	}
	if IsDeadRefreshError(newKiroErr(KindNetworkIssue, "net")) {
		t.Fatal("network error must not be dead")
	}
	if IsDeadRefreshError(errors.New("timeout")) {
		t.Fatal("foreign error must not be dead")
	}
}

func TestUserAgentFormat(t *testing.T) {
	t.Setenv("KIRO_MACHINE_ID", "")
	t.Setenv("KIRO_IDE_VERSION", "9.9.9")
	ua := kiroUserAgent()
	if !strings.HasPrefix(ua, "KiroIDE-9.9.9-") || len(ua) <= len("KiroIDE-9.9.9-") {
		t.Fatalf("ua = %q", ua)
	}
}

func TestAuthPageURLs(t *testing.T) {
	t.Setenv("KIRO_AUTH_PORTAL_URL", "https://portal.example/")
	if AuthSuccessPageURL() != "https://portal.example/signin?auth_status=success&redirect_from=KiroIDE" {
		t.Fatal(AuthSuccessPageURL())
	}
	if !strings.HasSuffix(AuthErrorPageURL("a b&c"), "&error_message=a%20b%26c") {
		t.Fatal(AuthErrorPageURL("a b&c"))
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:5000": true, "[::1]:80": true, "127.0.0.1": true,
		"10.0.0.5:80": false, "203.0.113.9:1": false, "": false, "localhost:80": false,
	} {
		if got := IsLoopbackAddr(addr); got != want {
			t.Errorf("IsLoopbackAddr(%q) = %v, want %v", addr, got, want)
		}
	}
}
