package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gin-gonic/gin"
)

// These tests pin the contract the unchanged dashboard depends on
// (ui/src/api.ts, use-device-code-poll.ts, kiro-connection-form.tsx).

type kiroTestEnv struct {
	r       *gin.Engine
	db      *storage.JsonDB
	key     string
	oidc    *httptest.Server
	auth    *httptest.Server
	catalog *fakeCatalog
}

func newKiroTestEnv(t *testing.T) *kiroTestEnv {
	t.Helper()
	r, db, dash := newAuthHardeningRouter(t)
	// The dashboard saves a hand-picked model list through the real update route.
	r.Group("/api", dashboardKeyMiddleware(db)).PUT("/connections/:id", apiUpdateConnection(db))

	oidc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/client/register":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"clientId": "cid", "clientSecret": "sec",
				"clientSecretExpiresAt": time.Now().Add(90 * 24 * time.Hour).Unix(),
			})
		case "/token":
			_, _ = io.WriteString(w, `{"accessToken":"at-new","refreshToken":"rt-new","expiresIn":3600}`)
		case "/device_authorization":
			_, _ = io.WriteString(w, `{"deviceCode":"dc","userCode":"ABCD-EFGH","verificationUri":"https://device.example/verify","verificationUriComplete":"https://device.example/verify?c=ABCD-EFGH","expiresIn":600,"interval":5}`)
		default:
			http.NotFound(w, req)
		}
	}))
	restore := auth.OverrideOIDCBaseURL(oidc.URL)

	svc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/oauth/token" {
			_, _ = io.WriteString(w, `{"accessToken":"s-at","refreshToken":"s-rt","expiresIn":3600,"profileArn":"arn:social"}`)
			return
		}
		http.NotFound(w, req)
	}))
	t.Setenv("KIRO_AUTH_ENDPOINT", svc.URL)
	t.Setenv("KIRO_LOGIN_FLOW", "")

	// Model sync runs after every login; keep it off the real network and
	// controllable per test.
	catalog := &fakeCatalog{models: []string{"claude-sonnet-5", "claude-opus-5"}}
	cat := httptest.NewServer(catalog)
	restoreModels := auth.OverrideModelEndpoints(cat.URL)

	t.Cleanup(func() { restore(); restoreModels(); oidc.Close(); svc.Close(); cat.Close() })
	return &kiroTestEnv{r: r, db: db, key: dash.Key, oidc: oidc, auth: svc, catalog: catalog}
}

func (e *kiroTestEnv) post(t *testing.T, path, body string, hdr map[string]string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+e.key)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (e *kiroTestEnv) connections(t *testing.T) []domain.ProviderConnection {
	t.Helper()
	cfg, err := e.db.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg.ProviderConnections
}

// httptest requests have RemoteAddr 192.0.2.1:1234 (not loopback).
var local = map[string]string{}

func mustHaveDeviceShape(t *testing.T, body map[string]any) {
	t.Helper()
	for _, k := range []string{"sessionId", "userCode", "verificationUri", "verificationUriComplete", "expiresIn", "interval"} {
		if body[k] == nil || body[k] == "" {
			t.Fatalf("response missing %q (the dashboard needs it): %v", k, body)
		}
	}
}

func TestStartBuilderIDAutoCodeFlowKeepsDashboardContract(t *testing.T) {
	e := newKiroTestEnv(t)
	t.Setenv("KIRO_LOGIN_FLOW", "auth-code")

	code, body := e.post(t, "/api/auth/kiro/start-builderid", "", nil)
	if code != 200 {
		t.Fatalf("status %d: %v", code, body)
	}
	mustHaveDeviceShape(t, body)
	u, _ := url.Parse(body["verificationUriComplete"].(string))
	if u.Path != "/authorize" || !strings.HasPrefix(u.Query().Get("redirect_uri"), "http://127.0.0.1:") {
		t.Fatalf("authorize url = %v", body["verificationUriComplete"])
	}
	if body["interval"].(float64) < 3 {
		t.Fatalf("interval %v would hammer the poll endpoint", body["interval"])
	}

	// Poll → pending until the browser returns.
	_, pend := e.post(t, "/api/auth/kiro/poll", `{"sessionId":"`+body["sessionId"].(string)+`"}`, nil)
	if pend["status"] != "pending" {
		t.Fatalf("poll = %v", pend)
	}

	// Browser comes back to the loopback listener.
	state := u.Query().Get("state")
	resp, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).
		Get(u.Query().Get("redirect_uri") + "?code=C&state=" + state)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	var done map[string]any
	for i := 0; i < 100; i++ {
		_, done = e.post(t, "/api/auth/kiro/poll", `{"sessionId":"`+body["sessionId"].(string)+`"}`, nil)
		if done["status"] != "pending" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if done["status"] != "success" || done["id"] == nil || done["name"] == nil {
		t.Fatalf("final poll = %v", done)
	}

	conns := e.connections(t)
	var got *domain.ProviderConnection
	for i := range conns {
		if conns[i].ID == done["id"] {
			got = &conns[i]
		}
	}
	if got == nil {
		t.Fatal("connection not saved")
	}
	pd := got.ProviderSpecificData
	if got.Provider != "kiro" || got.AuthType != "oauth" || got.AccessToken != "at-new" || got.RefreshToken != "rt-new" {
		t.Fatalf("conn = %+v", got)
	}
	// Same keys the rest of dntproxy (and the dashboard) already read.
	if pd["authMethod"] != "builder-id" || pd["clientId"] != "cid" || pd["clientSecret"] != "sec" || pd["region"] != "us-east-1" || pd["provider"] != "AWS Builder ID" {
		t.Fatalf("providerSpecificData = %v", pd)
	}
	if pd["clientSecretExpiresAt"] == nil {
		t.Fatal("client secret expiry must be stored so refresh can fail fast")
	}
}

func TestStartIDCDeviceFlowFallbackKeepsContract(t *testing.T) {
	e := newKiroTestEnv(t)
	t.Setenv("KIRO_LOGIN_FLOW", "device-code")

	code, body := e.post(t, "/api/auth/kiro/start-idc", `{"startUrl":"https://acme.awsapps.com/start","region":"us-west-1"}`, nil)
	if code != 200 {
		t.Fatalf("status %d: %v", code, body)
	}
	mustHaveDeviceShape(t, body)
	if body["userCode"] != "ABCD-EFGH" || body["verificationUriComplete"] != "https://device.example/verify?c=ABCD-EFGH" {
		t.Fatalf("device-code response = %v", body)
	}
}

func TestStartLoginPicksFlowByContext(t *testing.T) {
	e := newKiroTestEnv(t) // KIRO_LOGIN_FLOW unset

	// A request that arrived through a proxy cannot reach the loopback listener.
	_, viaProxy := e.post(t, "/api/auth/kiro/start-builderid", "", map[string]string{"X-Forwarded-For": "203.0.113.7"})
	if !strings.HasPrefix(viaProxy["verificationUriComplete"].(string), "https://device.example/") {
		t.Fatalf("forwarded request must use device-code: %v", viaProxy)
	}
	// httptest's RemoteAddr is 192.0.2.1 — not loopback either.
	_, remote := e.post(t, "/api/auth/kiro/start-builderid", "", nil)
	if !strings.HasPrefix(remote["verificationUriComplete"].(string), "https://device.example/") {
		t.Fatalf("non-loopback caller must use device-code: %v", remote)
	}
}

func TestStartIDCRequiresStartURL(t *testing.T) {
	e := newKiroTestEnv(t)
	if code, _ := e.post(t, "/api/auth/kiro/start-idc", `{}`, nil); code != 400 {
		t.Fatalf("status %d, want 400", code)
	}
}

func TestPollUnknownSessionIs404(t *testing.T) {
	e := newKiroTestEnv(t)
	if code, _ := e.post(t, "/api/auth/kiro/poll", `{"sessionId":"nope"}`, nil); code != 404 {
		t.Fatalf("status %d, want 404", code)
	}
}

func TestSocialLoginViaPortalKeepsDashboardContract(t *testing.T) {
	e := newKiroTestEnv(t)
	t.Setenv("KIRO_AUTH_PORTAL_URL", "")

	code, start := e.post(t, "/api/auth/kiro/start-social", `{"provider":"github"}`, nil)
	if code != 200 || start["sessionId"] == nil || start["loginUrl"] == nil || start["state"] == nil {
		t.Fatalf("start = %d %v", code, start)
	}
	if !strings.HasPrefix(start["loginUrl"].(string), "https://app.kiro.dev/signin?") {
		t.Fatalf("loginUrl = %v", start["loginUrl"])
	}

	// The dashboard pastes whatever is in the address bar after the redirect.
	cb := "http://localhost:3128/oauth/callback?login_option=github&code=C&state=" + url.QueryEscape(start["state"].(string))
	code, done := e.post(t, "/api/auth/kiro/exchange-social", `{"sessionId":"`+start["sessionId"].(string)+`","callbackUrl":"`+cb+`"}`, nil)
	if code != 200 || done["id"] == nil || done["name"] == nil {
		t.Fatalf("exchange = %d %v", code, done)
	}

	var got *domain.ProviderConnection
	for _, c := range e.connections(t) {
		if c.ID == done["id"] {
			c := c
			got = &c
		}
	}
	if got == nil || got.RefreshToken != "s-rt" {
		t.Fatalf("conn = %+v", got)
	}
	pd := got.ProviderSpecificData
	if pd["authMethod"] != "github" || pd["provider"] != "GitHub" || pd["profileArn"] != "arn:social" || pd["clientId"] != nil {
		t.Fatalf("providerSpecificData = %v", pd)
	}
}

func TestSocialExchangeRejectsForgedState(t *testing.T) {
	e := newKiroTestEnv(t)
	_, start := e.post(t, "/api/auth/kiro/start-social", `{"provider":"google"}`, nil)
	cb := "http://localhost:3128/oauth/callback?login_option=google&code=C&state=forged"
	code, body := e.post(t, "/api/auth/kiro/exchange-social", `{"sessionId":"`+start["sessionId"].(string)+`","callbackUrl":"`+cb+`"}`, nil)
	if code != 400 || body["error"] == nil {
		t.Fatalf("forged state = %d %v", code, body)
	}
	if len(e.connections(t)) != 2 { // only the two seeded fixtures
		t.Fatal("a forged callback must not create a connection")
	}
}

func TestSocialExchangeAcceptsLegacyKiroSchemeAndBareCode(t *testing.T) {
	e := newKiroTestEnv(t)

	_, s1 := e.post(t, "/api/auth/kiro/start-social", `{"provider":"google"}`, nil)
	cb := "kiro://kiro.kiroAgent/authenticate-success?code=C&state=" + url.QueryEscape(s1["state"].(string))
	if code, body := e.post(t, "/api/auth/kiro/exchange-social", `{"sessionId":"`+s1["sessionId"].(string)+`","callbackUrl":"`+cb+`"}`, nil); code != 200 {
		t.Fatalf("legacy scheme = %d %v", code, body)
	}

	_, s2 := e.post(t, "/api/auth/kiro/start-social", `{"provider":"github"}`, nil)
	if code, body := e.post(t, "/api/auth/kiro/exchange-social", `{"sessionId":"`+s2["sessionId"].(string)+`","code":"C"}`, nil); code != 200 {
		t.Fatalf("bare code = %d %v", code, body)
	}
}

func TestSocialStartValidatesProvider(t *testing.T) {
	e := newKiroTestEnv(t)
	if code, _ := e.post(t, "/api/auth/kiro/start-social", `{"provider":"gitlab"}`, nil); code != 400 {
		t.Fatalf("status %d, want 400", code)
	}
}

func TestExchangeSocialRejectsCrossPrincipalSession(t *testing.T) {
	e := newKiroTestEnv(t)
	cfg, _ := e.db.Load()
	cfg.APIKeys = append(cfg.APIKeys, domain.APIKey{ID: "other", Key: "sk-dnt-other", IsActive: true, DashboardAccess: true, TenantID: "acme"})
	_ = e.db.Save(cfg)

	_, start := e.post(t, "/api/auth/kiro/start-social", `{"provider":"google"}`, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/kiro/exchange-social",
		bytes.NewBufferString(`{"sessionId":"`+start["sessionId"].(string)+`","code":"C"}`))
	req.Header.Set("Authorization", "Bearer sk-dnt-other")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", w.Code)
	}
}
