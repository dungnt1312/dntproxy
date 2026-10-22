package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// fakeIdC serves RegisterClient and CreateToken and records the last token request.
type fakeIdC struct {
	lastToken map[string]interface{}
}

func (f *fakeIdC) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/client/register":
			exp := time.Now().Add(90 * 24 * time.Hour).Unix()
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"clientId": "cid-1", "clientSecret": "sec-1", "clientSecretExpiresAt": exp,
			})
		case "/token":
			f.lastToken = map[string]interface{}{}
			_ = json.NewDecoder(r.Body).Decode(&f.lastToken)
			_, _ = io.WriteString(w, `{"accessToken":"at-1","refreshToken":"rt-1","expiresIn":3600}`)
		default:
			http.NotFound(w, r)
		}
	})
}

func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func waitDone(t *testing.T, l *IdCLogin) (*KiroCredential, error) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cred, done, err := l.Poll(context.Background())
		if done {
			return cred, err
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("login did not finish")
	return nil, nil
}

func TestIdCLoginAuthorizationCodeEndToEnd(t *testing.T) {
	f := &fakeIdC{}
	withOIDC(t, f.handler())

	login, err := StartIdCLogin(context.Background(), IdCLoginOptions{Provider: ProviderBuilderID})
	if err != nil {
		t.Fatal(err)
	}
	defer login.Cancel()

	u, err := url.Parse(login.AuthorizeURL)
	if err != nil || u.Path != "/authorize" {
		t.Fatalf("authorize url = %q (%v)", login.AuthorizeURL, err)
	}
	q := u.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != "cid-1" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorize query = %v", q)
	}
	if strings.Count(q.Get("scopes"), ",") != 4 {
		t.Fatalf("scopes = %q", q.Get("scopes"))
	}
	redirect := q.Get("redirect_uri")
	if !strings.HasPrefix(redirect, "http://127.0.0.1:") || !strings.HasSuffix(redirect, "/oauth/callback") {
		t.Fatalf("redirect_uri = %q", redirect)
	}

	// Still pending until the browser comes back.
	if _, done, _ := login.Poll(context.Background()); done {
		t.Fatal("must be pending before the callback")
	}

	resp, err := noRedirectClient().Get(redirect + "?code=the-code&state=" + url.QueryEscape(q.Get("state")))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 302 || resp.Header.Get("Location") != AuthSuccessPageURL() {
		t.Fatalf("callback = %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	cred, err := waitDone(t, login)
	if err != nil {
		t.Fatal(err)
	}
	if cred.AccessToken != "at-1" || cred.RefreshToken != "rt-1" || cred.Provider != ProviderBuilderID {
		t.Fatalf("cred = %+v", cred)
	}
	if cred.ClientID != "cid-1" || cred.ClientSecretExpiresAt == "" || cred.StartURL != BuilderIDStartURL {
		t.Fatalf("cred = %+v", cred)
	}

	// CreateToken got the authorization_code grant, bound to our PKCE challenge.
	tok := f.lastToken
	if tok["grantType"] != "authorization_code" || tok["code"] != "the-code" || tok["redirectUri"] != redirect {
		t.Fatalf("token request = %v", tok)
	}
	sum := sha256.Sum256([]byte(tok["codeVerifier"].(string)))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != q.Get("code_challenge") {
		t.Fatal("code_verifier does not match code_challenge")
	}

	pd := cred.ProviderData()
	if pd["authMethod"] != "builder-id" || pd["clientIdHash"] != ClientIDHash(BuilderIDStartURL) || pd["region"] != "us-east-1" {
		t.Fatalf("provider data = %v", pd)
	}

	// Poll is idempotent after completion.
	if again, done, err := login.Poll(context.Background()); !done || err != nil || again != cred {
		t.Fatal("Poll after completion must return the same result")
	}
}

func TestIdCLoginRejectsWrongState(t *testing.T) {
	f := &fakeIdC{}
	withOIDC(t, f.handler())
	login, err := StartIdCLogin(context.Background(), IdCLoginOptions{Provider: ProviderBuilderID})
	if err != nil {
		t.Fatal(err)
	}
	defer login.Cancel()
	redirect, _ := url.Parse(login.AuthorizeURL)
	redirectURI := redirect.Query().Get("redirect_uri")

	resp, err := noRedirectClient().Get(redirectURI + "?code=x&state=forged")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Location"), "auth_status=error") {
		t.Fatalf("Location = %q", resp.Header.Get("Location"))
	}
	if _, err := waitDone(t, login); !IsBadAuthIssue(err) {
		t.Fatalf("err = %v", err)
	}
	if f.lastToken != nil {
		t.Fatal("a forged callback must never reach CreateToken")
	}
}

func TestIdCLoginSubmitPastedCallback(t *testing.T) {
	f := &fakeIdC{}
	withOIDC(t, f.handler())
	login, err := StartIdCLogin(context.Background(), IdCLoginOptions{Provider: ProviderBuilderID})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(login.AuthorizeURL)
	state := u.Query().Get("state")

	cred, err := login.SubmitCallback(context.Background(), "http://127.0.0.1:1/oauth/callback?code=pasted&state="+state)
	if err != nil || cred.AccessToken != "at-1" || f.lastToken["code"] != "pasted" {
		t.Fatalf("cred=%+v err=%v token=%v", cred, err, f.lastToken)
	}
}

func TestIdCLoginSubmitRejectsBadState(t *testing.T) {
	f := &fakeIdC{}
	withOIDC(t, f.handler())
	login, _ := StartIdCLogin(context.Background(), IdCLoginOptions{Provider: ProviderBuilderID})
	if _, err := login.SubmitCallback(context.Background(), "http://127.0.0.1:1/oauth/callback?code=x&state=nope"); !IsBadAuthIssue(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestIdCLoginCancelFreesListener(t *testing.T) {
	withOIDC(t, (&fakeIdC{}).handler())
	login, err := StartIdCLogin(context.Background(), IdCLoginOptions{Provider: ProviderBuilderID})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(login.AuthorizeURL)
	redirect := u.Query().Get("redirect_uri")
	login.Cancel()
	if _, err := waitDone(t, login); ErrorKind(err) != KindCanceled {
		t.Fatalf("err = %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if resp, err := http.Get(redirect + "?code=x"); err == nil {
		resp.Body.Close()
		t.Fatal("listener should be closed after cancel")
	}
}

func TestStartIdCLoginValidatesStartURL(t *testing.T) {
	withOIDC(t, (&fakeIdC{}).handler())
	for _, bad := range []string{"http://acme.awsapps.com/start", "not a url", "https://user:pw@acme.awsapps.com/start"} {
		_, err := StartIdCLogin(context.Background(), IdCLoginOptions{Provider: ProviderEnterprise, StartURL: bad})
		if ErrorKind(err) != KindInvalidUserInput {
			t.Errorf("start url %q: err = %v", bad, err)
		}
	}
}

func TestEnterpriseLoginResolvesSingleProfile(t *testing.T) {
	oldQ := qEndpoint
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("TokenType") != "SSO_OIDC" || r.Header.Get("Authorization") != "Bearer at-ent" {
			t.Errorf("headers = %v", r.Header)
		}
		switch r.URL.Path {
		case "/us-east-1/ListAvailableProfiles":
			_, _ = io.WriteString(w, `{"profiles":[{"arn":"arn:aws:codewhisperer:us-east-1:111:profile/AAA","profileName":"Team"},{"arn":"","profileName":"broken"}]}`)
		default:
			w.WriteHeader(500) // the other region failing must not matter
		}
	}))
	qEndpoint = func(region string) string { return srv.URL + "/" + region }
	t.Cleanup(func() { qEndpoint = oldQ; srv.Close() })

	cred := &KiroCredential{Provider: ProviderEnterprise, AccessToken: "at-ent", Region: "us-west-1"}
	arn, err := ResolveProfileArn(context.Background(), cred)
	if err != nil || arn != "arn:aws:codewhisperer:us-east-1:111:profile/AAA" {
		t.Fatalf("arn=%q err=%v", arn, err)
	}

	// Builder ID never calls out.
	if arn, err := ResolveProfileArn(context.Background(), &KiroCredential{Provider: ProviderBuilderID}); arn != "" || err != nil {
		t.Fatalf("builder id arn=%q err=%v", arn, err)
	}
}

func TestEnterpriseLoginLeavesProfileEmptyWhenAmbiguous(t *testing.T) {
	oldQ := qEndpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"profiles":[{"arn":"arn:a","profileName":"A"}]}`)
	}))
	qEndpoint = func(string) string { return srv.URL } // both regions answer → 2 profiles
	t.Cleanup(func() { qEndpoint = oldQ; srv.Close() })

	arn, err := ResolveProfileArn(context.Background(), &KiroCredential{Provider: ProviderEnterprise, AccessToken: "t", Region: "us-east-1"})
	if arn != "" || err == nil {
		t.Fatalf("arn=%q err=%v, want empty + error", arn, err)
	}
}

func TestAwsPartition(t *testing.T) {
	for region, want := range map[string]string{
		"us-east-1": "aws", "eu-central-1": "aws", "cn-north-1": "aws-cn",
		"us-gov-west-1": "aws-us-gov", "us-iso-east-1": "aws-iso", "us-isob-east-1": "aws-iso-b", "us-isof-south-1": "aws-iso-f",
	} {
		if got := awsPartition(region); got != want {
			t.Errorf("awsPartition(%s) = %s, want %s", region, got, want)
		}
	}
}
