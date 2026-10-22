package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

// fakeCatalog stands in for management.<region>.kiro.dev and q.<region>.amazonaws.com.
// Both surfaces share one catalog; paths are /management/List-Available-Models
// and /q/ListAvailableModels.
type fakeCatalog struct {
	mu       sync.Mutex
	models   []string
	status   int      // non-zero: answer every request with this status
	pageSize int      // >0: paginate
	validTok string   // if set, any other bearer gets 403
	requests []string // model catalog requests only
	headers  []http.Header

	// getUsageLimits (account email lookup) is served separately so it never
	// counts as a model-catalog request.
	email      string // "" answers userInfo.email = null
	usageFail  int    // non-zero: answer getUsageLimits with this status
	usageCalls []string
}

func (f *fakeCatalog) usageRequests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.usageCalls...)
}

func (f *fakeCatalog) set(models ...string) {
	f.mu.Lock()
	f.models = models
	f.mu.Unlock()
}

func (f *fakeCatalog) failWith(status int) {
	f.mu.Lock()
	f.status = status
	f.mu.Unlock()
}

func (f *fakeCatalog) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *fakeCatalog) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.HasSuffix(r.URL.Path, "/getUsageLimits") {
		f.usageCalls = append(f.usageCalls, r.URL.RawQuery+"|TokenType="+r.Header.Get("TokenType"))
		if f.usageFail != 0 {
			w.WriteHeader(f.usageFail)
			return
		}
		var email interface{}
		if f.email != "" && r.URL.Query().Get("isEmailRequired") == "true" {
			email = f.email
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"userInfo": map[string]any{"email": email, "userId": "d-1.u-1"}})
		return
	}
	f.requests = append(f.requests, r.URL.String())
	f.headers = append(f.headers, r.Header.Clone())
	if !strings.HasSuffix(r.URL.Path, "/List-Available-Models") && !strings.HasSuffix(r.URL.Path, "/ListAvailableModels") {
		http.NotFound(w, r)
		return
	}
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(`{"message":"synthetic failure"}`))
		return
	}
	if f.validTok != "" && r.Header.Get("Authorization") != "Bearer "+f.validTok {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"message":"The bearer token included in the request is invalid."}`))
		return
	}

	start := 0
	if tok := r.URL.Query().Get("nextToken"); tok != "" {
		for i, m := range f.models {
			if m == tok {
				start = i
			}
		}
	}
	end := len(f.models)
	next := ""
	if f.pageSize > 0 && start+f.pageSize < end {
		end = start + f.pageSize
		next = f.models[end]
	}
	items := make([]map[string]any, 0, end-start)
	for _, id := range f.models[start:end] {
		items = append(items, map[string]any{
			"modelId": id, "modelName": strings.ToUpper(id), "modelProvider": "anthropic",
			"rateMultiplier": 1.5, "tokenLimits": map[string]any{"maxInputTokens": 200000, "maxOutputTokens": 8192},
		})
	}
	out := map[string]any{"models": items}
	if len(f.models) > 0 {
		out["defaultModel"] = map[string]any{"modelId": f.models[0]}
	}
	if next != "" {
		out["nextToken"] = next
	}
	_ = json.NewEncoder(w).Encode(out)
}

func (e *kiroTestEnv) put(t *testing.T, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+e.key)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (e *kiroTestEnv) conn(t *testing.T, id string) domain.ProviderConnection {
	t.Helper()
	for _, c := range e.connections(t) {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("connection %s not found", id)
	return domain.ProviderConnection{}
}

// addKiroConn seeds a Kiro connection with the given model list.
func (e *kiroTestEnv) addKiroConn(t *testing.T, id string, models []string) {
	t.Helper()
	cfg, err := e.db.Load()
	if err != nil {
		t.Fatal(err)
	}
	pd := map[string]interface{}{"authMethod": "idc", "clientId": "cid", "clientSecret": "sec", "region": "us-east-1"}
	cfg.ProviderConnections = append(cfg.ProviderConnections, domain.ProviderConnection{
		ID: id, Name: id, Provider: "kiro", AuthType: "oauth", IsActive: true, Weight: 100,
		AccessToken: "at", RefreshToken: "rt", ExpiresAt: "2099-01-01T00:00:00Z",
		SupportedModels: models, ProviderSpecificData: pd,
	})
	if err := e.db.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

// --- fetch-models: what the unchanged dashboard reads ------------------------

func TestFetchModelsKiroIsLiveAndKeepsDashboardShape(t *testing.T) {
	e := newKiroTestEnv(t)
	e.catalog.set("claude-opus-5", "claude-sonnet-5", "glm-5")
	e.addKiroConn(t, "k1", nil)

	code, body := e.post(t, "/api/connections/k1/fetch-models", `{}`, nil)
	if code != 200 {
		t.Fatalf("status %d: %v", code, body)
	}
	models, _ := body["models"].([]any)
	if len(models) != 3 || models[0] != "claude-opus-5" || models[2] != "glm-5" {
		t.Fatalf("models = %v (the dashboard reads res.models as string[])", body["models"])
	}
	// connection-result-panel.tsx shows "provider default models… not fetched live"
	// whenever source === 'provider-config'; a live answer must not say that.
	if body["source"] == "provider-config" || body["note"] != nil {
		t.Fatalf("live answer is flagged as defaults: %v", body)
	}
	if body["defaultModel"] != "claude-opus-5" {
		t.Fatalf("defaultModel = %v", body["defaultModel"])
	}
	// Fetching is read-only: the stored list is only changed when the operator saves.
	if got := e.conn(t, "k1"); len(got.SupportedModels) != 0 {
		t.Fatalf("fetch-models must not write: %v", got.SupportedModels)
	}
}

func TestFetchModelsKiroSendsIdeStyleRequest(t *testing.T) {
	e := newKiroTestEnv(t)
	e.catalog.set("claude-sonnet-5")
	e.addKiroConn(t, "k2", nil)
	e.post(t, "/api/connections/k2/fetch-models", `{}`, nil)

	if e.catalog.calls() == 0 {
		t.Fatal("no catalog request made")
	}
	first := e.catalog.requests[0]
	if !strings.HasPrefix(first, "/management/List-Available-Models?") || !strings.Contains(first, "origin=AI_EDITOR") {
		t.Fatalf("first request = %s", first)
	}
	h := e.catalog.headers[0]
	if h.Get("Authorization") != "Bearer at" || h.Get("TokenType") != "SSO_OIDC" || !strings.HasPrefix(h.Get("User-Agent"), "KiroIDE-") {
		t.Fatalf("headers = %v", h)
	}
}

func TestFetchModelsKiroFollowsPagination(t *testing.T) {
	e := newKiroTestEnv(t)
	e.catalog.set("a", "b", "c", "d", "e")
	e.catalog.pageSize = 2
	e.addKiroConn(t, "k3", nil)

	_, body := e.post(t, "/api/connections/k3/fetch-models", `{}`, nil)
	models, _ := body["models"].([]any)
	if len(models) != 5 {
		t.Fatalf("models = %v, want all 5 across pages", body["models"])
	}
	if e.catalog.calls() != 3 {
		t.Fatalf("calls = %d, want 3 pages", e.catalog.calls())
	}
}

func TestFetchModelsKiroFallsBackToDefaultsAndNeverReturnsUpstream401(t *testing.T) {
	e := newKiroTestEnv(t)
	e.addKiroConn(t, "k4", nil)

	// The dashboard treats ANY 401 as "dashboard session expired" and logs the
	// operator out, so an upstream rejection must never be forwarded as 401/403.
	for _, status := range []int{401, 403, 500, 404} {
		e.catalog.failWith(status)
		code, body := e.post(t, "/api/connections/k4/fetch-models", `{}`, nil)
		if code != 200 {
			t.Fatalf("upstream %d surfaced as HTTP %d: %v", status, code, body)
		}
		if body["source"] != "provider-config" || body["note"] == nil {
			t.Fatalf("upstream %d: want defaults + note, got %v", status, body)
		}
		if ms, _ := body["models"].([]any); len(ms) == 0 {
			t.Fatalf("upstream %d: fallback list is empty", status)
		}
	}
}

func TestFetchModelsKiroRefreshesRejectedTokenOnce(t *testing.T) {
	e := newKiroTestEnv(t)
	e.catalog.set("claude-sonnet-5")
	e.catalog.validTok = "at-new" // the OIDC fake mints this on refresh
	e.addKiroConn(t, "k5", nil)

	code, body := e.post(t, "/api/connections/k5/fetch-models", `{}`, nil)
	if code != 200 || body["source"] != "live" {
		t.Fatalf("after refresh: %d %v", code, body)
	}
	if got := e.conn(t, "k5"); got.AccessToken != "at-new" || got.RefreshToken != "rt-new" {
		t.Fatalf("refreshed credentials not persisted: %+v", got)
	}
}

func TestFetchModelsKiroEnforcesTenantOwnership(t *testing.T) {
	e := newKiroTestEnv(t)
	cfg, _ := e.db.Load()
	cfg.APIKeys = append(cfg.APIKeys, domain.APIKey{ID: "tk", Key: "sk-dnt-t", IsActive: true, DashboardAccess: true, TenantID: "other"})
	_ = e.db.Save(cfg)
	e.addKiroConn(t, "k6", nil)

	before := e.catalog.calls()
	e.key = "sk-dnt-t"
	if code, _ := e.post(t, "/api/connections/k6/fetch-models", `{}`, nil); code != 404 {
		t.Fatalf("cross-tenant fetch = %d, want 404", code)
	}
	if e.catalog.calls() != before {
		t.Fatal("a foreign tenant's request must not reach upstream with this connection's token")
	}
}

// --- nothing writes the model list except the operator's own Save -----------

func TestFetchModelsNeverWritesTheConnection(t *testing.T) {
	e := newKiroTestEnv(t)
	e.catalog.set("claude-sonnet-5", "claude-opus-5")
	e.addKiroConn(t, "w1", []string{"hand-picked"})

	// A plain fetch writes nothing at all.
	before := e.conn(t, "w1")
	e.post(t, "/api/connections/w1/fetch-models", `{}`, nil)
	if after := e.conn(t, "w1"); after.UpdatedAt != before.UpdatedAt || after.AccessToken != before.AccessToken {
		t.Fatalf("a plain fetch modified the connection: %+v", after)
	}

	// A fetch that has to refresh an expired token does persist the new token
	// (otherwise the rotated refresh token would be lost), but never the model list.
	e.catalog.validTok = "at-new"
	e.post(t, "/api/connections/w1/fetch-models", `{}`, nil)

	got := e.conn(t, "w1")
	if got.AccessToken != "at-new" {
		t.Fatalf("the refreshed token should be persisted, got %q", got.AccessToken)
	}
	if strings.Join(got.SupportedModels, ",") != "hand-picked" {
		t.Fatalf("fetch changed the model list: %v", got.SupportedModels)
	}
	for _, k := range []string{"modelsSource", "modelsSyncedAt", "modelsSyncError"} {
		if _, ok := got.ProviderSpecificData[k]; ok {
			t.Fatalf("fetch must leave no sync bookkeeping, found %q", k)
		}
	}
}

func TestSyncModelsEndpointIsGone(t *testing.T) {
	e := newKiroTestEnv(t)
	e.addKiroConn(t, "g1", nil)
	before := e.catalog.calls()
	code, _ := e.post(t, "/api/connections/g1/sync-models", `{}`, nil)
	if code != 404 {
		t.Fatalf("sync-models = %d; the operator syncs by fetching into the form and saving", code)
	}
	if e.catalog.calls() != before {
		t.Fatal("a removed endpoint must not reach upstream")
	}
}

func TestNewLoginIsNamedAfterAccountEmail(t *testing.T) {
	e := newKiroTestEnv(t)
	e.catalog.email = "dev@example.com"
	t.Setenv("KIRO_AUTH_PORTAL_URL", "")

	_, start := e.post(t, "/api/auth/kiro/start-social", `{"provider":"github"}`, nil)
	cb := "http://localhost:3128/oauth/callback?login_option=github&code=C&state=" + start["state"].(string)
	_, done := e.post(t, "/api/auth/kiro/exchange-social", `{"sessionId":"`+start["sessionId"].(string)+`","callbackUrl":"`+cb+`"}`, nil)
	if done["email"] != "dev@example.com" || done["name"] != "dev@example.com" {
		t.Fatalf("exchange = %v", done)
	}
	got := e.conn(t, done["id"].(string))
	if got.Email != "dev@example.com" || got.Name != "dev@example.com" {
		t.Fatalf("conn name=%q email=%q", got.Name, got.Email)
	}
	if reqs := e.catalog.usageRequests(); len(reqs) != 1 || !strings.Contains(reqs[0], "isEmailRequired=true") {
		t.Fatalf("email lookup requests = %v", reqs)
	}
}

func TestNewLoginFallsBackToReadableNameWhenEmailUnavailable(t *testing.T) {
	e := newKiroTestEnv(t)
	e.catalog.usageFail = 500
	t.Setenv("KIRO_AUTH_PORTAL_URL", "")

	_, start := e.post(t, "/api/auth/kiro/start-social", `{"provider":"google"}`, nil)
	cb := "http://localhost:3128/oauth/callback?login_option=google&code=C&state=" + start["state"].(string)
	code, done := e.post(t, "/api/auth/kiro/exchange-social", `{"sessionId":"`+start["sessionId"].(string)+`","callbackUrl":"`+cb+`"}`, nil)
	if code != 200 || done["id"] == nil {
		t.Fatalf("a failed email lookup must not fail the login: %d %v", code, done)
	}
	if name, _ := done["name"].(string); name != "Kiro Google" {
		t.Fatalf("fallback name = %q, want %q", name, "Kiro Google")
	}
}

func TestNewLoginKeepsDefaultsAndMakesNoCatalogRequest(t *testing.T) {
	e := newKiroTestEnv(t)
	e.catalog.set("claude-sonnet-5", "claude-opus-5", "glm-5")
	t.Setenv("KIRO_AUTH_PORTAL_URL", "")

	_, start := e.post(t, "/api/auth/kiro/start-social", `{"provider":"github"}`, nil)
	cb := "http://localhost:3128/oauth/callback?login_option=github&code=C&state=" + start["state"].(string)
	_, done := e.post(t, "/api/auth/kiro/exchange-social", `{"sessionId":"`+start["sessionId"].(string)+`","callbackUrl":"`+cb+`"}`, nil)
	if done["id"] == nil {
		t.Fatalf("exchange = %v", done)
	}
	if e.catalog.calls() != 0 {
		t.Fatalf("login must not call the model catalog on its own (%d calls)", e.catalog.calls())
	}
	got := e.conn(t, done["id"].(string))
	cfg, _ := e.db.Load()
	want := domain.GetDefaultConnectionModels(&cfg.Settings, "kiro")
	if len(want) == 0 || strings.Join(got.SupportedModels, ",") != strings.Join(want, ",") {
		t.Fatalf("a new login should keep the configured defaults\n got: %v\nwant: %v", got.SupportedModels, want)
	}
	if strings.Join(got.SupportedModels, ",") == "claude-sonnet-5,claude-opus-5,glm-5" {
		t.Fatal("the live catalog must not be applied without the operator saving it")
	}
	if _, ok := got.ProviderSpecificData["modelsSource"]; ok {
		t.Fatal("no sync marker should be written")
	}
}
