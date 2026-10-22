package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

// usageServer fakes q.<region>.amazonaws.com/getUsageLimits.
type usageServer struct {
	mu      sync.Mutex
	email   string // "" -> userInfo.email = null
	status  int
	queries []string
	tokens  []string
}

func (u *usageServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !strings.HasSuffix(r.URL.Path, "/getUsageLimits") {
		http.NotFound(w, r)
		return
	}
	u.queries = append(u.queries, r.URL.RawQuery)
	u.tokens = append(u.tokens, r.Header.Get("TokenType"))
	if u.status != 0 {
		w.WriteHeader(u.status)
		return
	}
	var email interface{}
	// Mirrors Kiro: the email is only returned when explicitly requested.
	if u.email != "" && r.URL.Query().Get("isEmailRequired") == "true" {
		email = u.email
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"userInfo": map[string]any{"email": email, "userId": "d-1.u-1"}})
}

// kiroConn is an active IdC Kiro connection with a valid token.
func kiroConn(id, _ string, models []string) domain.ProviderConnection {
	c := idcConn(id, time.Hour, nil)
	c.SupportedModels = models
	return c
}

func withUsage(t *testing.T, email string) *usageServer {
	t.Helper()
	us := &usageServer{email: email}
	srv := httptest.NewServer(us)
	restore := OverrideModelEndpoints(srv.URL)
	t.Cleanup(func() { restore(); srv.Close() })
	return us
}

func TestFetchKiroAccountEmailAsksForIt(t *testing.T) {
	us := withUsage(t, "dev@example.com")
	conn := kiroConn("e1", "", nil)
	conn.ProviderSpecificData["profileArn"] = "arn:aws:codewhisperer:us-east-1:1:profile/P"

	email, err := FetchKiroAccountEmail(context.Background(), &conn)
	if err != nil || email != "dev@example.com" {
		t.Fatalf("email=%q err=%v", email, err)
	}
	q := us.queries[0]
	for _, want := range []string{"isEmailRequired=true", "origin=AI_EDITOR", "resourceType=AGENTIC_REQUEST", "profileArn="} {
		if !strings.Contains(q, want) {
			t.Fatalf("query %q missing %q", q, want)
		}
	}
	if us.tokens[0] != "SSO_OIDC" {
		t.Fatalf("TokenType = %q", us.tokens[0])
	}
}

func TestFetchKiroAccountEmailHandlesNullAndErrors(t *testing.T) {
	us := withUsage(t, "") // Kiro answers email: null
	conn := kiroConn("e2", "", nil)
	if email, err := FetchKiroAccountEmail(context.Background(), &conn); err != nil || email != "" {
		t.Fatalf("null email: %q %v", email, err)
	}
	us.status = 403
	if _, err := FetchKiroAccountEmail(context.Background(), &conn); err == nil {
		t.Fatal("HTTP 403 should be an error")
	}
	empty := domain.ProviderConnection{Provider: "kiro"}
	if _, err := FetchKiroAccountEmail(context.Background(), &empty); err == nil {
		t.Fatal("no credential should be an error")
	}
}

func TestResolveKiroEmailNeverFailsTheCaller(t *testing.T) {
	us := withUsage(t, "")
	us.status = 500
	conn := kiroConn("e3", "", nil)
	ResolveKiroEmail(context.Background(), &conn) // must not panic or block
	if conn.Email != "" {
		t.Fatalf("email = %q, want empty on failure", conn.Email)
	}
}

func TestKiroConnectionName(t *testing.T) {
	withEmail := kiroConn("n1", "", nil)
	withEmail.Email = "dev@example.com"
	if got := KiroConnectionName(&withEmail, nil); got != "dev@example.com" {
		t.Fatalf("with email: %q", got)
	}

	idc := kiroConn("n2", "", nil)
	idc.ProviderSpecificData["startUrl"] = "https://acme.awsapps.com/start"
	if got := KiroConnectionName(&idc, nil); got != "Kiro IdC (acme)" {
		t.Fatalf("idc: %q", got)
	}

	bid := domain.ProviderConnection{Provider: "kiro", ProviderSpecificData: map[string]interface{}{"authMethod": "builder-id"}}
	existing := []domain.ProviderConnection{
		{Name: "Kiro Builder ID"}, {Name: "Kiro Builder ID 2"},
		// Connections of other providers no longer inflate the number.
		{Name: "openai 1", Provider: "openai"}, {Name: "glm", Provider: "glm"},
	}
	if got := KiroConnectionName(&bid, existing); got != "Kiro Builder ID 3" {
		t.Fatalf("numbered: %q", got)
	}

	key := domain.ProviderConnection{Provider: "kiro", ProviderSpecificData: map[string]interface{}{"authMethod": "api_key"}}
	if got := KiroConnectionName(&key, nil); got != "Kiro API Key" {
		t.Fatalf("api key: %q", got)
	}
}

func TestIsDefaultKiroName(t *testing.T) {
	for _, name := range []string{
		"", "AWS IAM Identity Center Account 62", "AWS Builder ID Account 3", "Google Account 1",
		"Imported Account 9", "Kiro API Key 2", "Kiro IdC (acme)", "Kiro Builder ID", "Kiro Builder ID 4",
	} {
		if !IsDefaultKiroName(name) {
			t.Errorf("%q should count as a generated name", name)
		}
	}
	for _, name := range []string{"Team AWS", "dev@example.com", "My Kiro Account", "AWS IAM Identity Center prod"} {
		if IsDefaultKiroName(name) {
			t.Errorf("%q is operator-chosen and must be kept", name)
		}
	}
}

func TestBackfillFillsEmailAndOnlyRenamesPlaceholders(t *testing.T) {
	withUsage(t, "dev@example.com")

	placeholder := kiroConn("b1", "", nil)
	placeholder.Name = "AWS IAM Identity Center Account 62"
	custom := kiroConn("b2", "", nil)
	custom.Name = "Team AWS"
	hasEmail := kiroConn("b3", "", nil)
	hasEmail.Name, hasEmail.Email = "keep", "old@example.com"
	inactive := kiroConn("b4", "", nil)
	inactive.IsActive = false
	sidelined := kiroConn("b5", "", nil)
	sidelined.RateLimitedUntil = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	other := kiroConn("b6", "", nil)
	other.Provider = "openai"

	store := newRefreshStore(t, placeholder, custom, hasEmail, inactive, sidelined, other)
	if n := BackfillKiroAccountEmails(context.Background(), store); n != 2 {
		t.Fatalf("updated = %d, want 2 (placeholder + custom)", n)
	}

	check := func(id, wantName, wantEmail string) {
		t.Helper()
		c, _ := store.GetConnectionByID(id)
		if c.Name != wantName || c.Email != wantEmail {
			t.Errorf("%s: name=%q email=%q, want %q / %q", id, c.Name, c.Email, wantName, wantEmail)
		}
	}
	check("b1", "dev@example.com", "dev@example.com") // placeholder replaced
	check("b2", "Team AWS", "dev@example.com")        // operator name kept, email filled
	check("b3", "keep", "old@example.com")            // already had an email
	check("b4", "b4", "")                             // inactive skipped
	check("b5", "b5", "")                             // sidelined skipped
	check("b6", "b6", "")                             // not kiro

	// Idempotent: a second run finds nothing to do.
	if n := BackfillKiroAccountEmails(context.Background(), store); n != 0 {
		t.Fatalf("second run updated %d", n)
	}
}
