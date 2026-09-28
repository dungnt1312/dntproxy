package autologin

import (
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestClassifyAccounts(t *testing.T) {
	now := time.Now()
	fresh := now.Add(10 * 24 * time.Hour).Format(time.RFC3339)
	stale := now.Add(-24 * time.Hour).Format(time.RFC3339)
	soon := now.Add(2 * time.Hour).Format(time.RFC3339) // inside skipHealthHorizon

	conns := []domain.ProviderConnection{
		{ID: "healthy", Provider: "openai", AuthType: "oauth", Email: "ok@x.com", IsActive: true, RefreshToken: "rt", ExpiresAt: fresh},
		{ID: "expired", Provider: "openai", AuthType: "oauth", Email: "stale@x.com", IsActive: true, RefreshToken: "rt", ExpiresAt: stale},
		{ID: "no-refresh", Provider: "openai", AuthType: "oauth", Email: "nort@x.com", IsActive: true, ExpiresAt: fresh},
		{ID: "inactive", Provider: "openai", AuthType: "oauth", Email: "off@x.com", IsActive: false, RefreshToken: "rt", ExpiresAt: fresh},
		{ID: "soon", Provider: "openai", AuthType: "oauth", Email: "soon@x.com", IsActive: true, RefreshToken: "rt", ExpiresAt: soon},
	}

	accounts := []account{
		{Email: "ok@x.com", Password: "p"},
		{Email: "OK@X.com", Password: "p"}, // same as ok@x.com, case-insensitive
		{Email: "stale@x.com", Password: "p"},
		{Email: "nort@x.com", Password: "p"},
		{Email: "off@x.com", Password: "p"},
		{Email: "soon@x.com", Password: "p"},
		{Email: "brand-new@x.com", Password: "p"},
	}

	process, skipped := classifyAccounts(accounts, conns, now)

	// Only stale, no-refresh, inactive, soon, and brand-new need a login.
	gotEmails := make(map[string]bool)
	for _, a := range process {
		gotEmails[a.Email] = true
	}
	wantProcess := []string{"stale@x.com", "nort@x.com", "off@x.com", "soon@x.com", "brand-new@x.com"}
	if len(process) != len(wantProcess) {
		t.Fatalf("process = %v, want %v", process, wantProcess)
	}
	for _, e := range wantProcess {
		if !gotEmails[e] {
			t.Errorf("expected %q in process, got %v", e, gotEmails)
		}
	}

	// Both ok@x.com lines skip (dedupe happened earlier; each is healthy).
	if len(skipped) != 2 {
		t.Fatalf("skipped = %d entries, want 2: %+v", len(skipped), skipped)
	}
	for _, s := range skipped {
		if s.ConnectionID != "healthy" {
			t.Errorf("skipped entry %q pointed at %q", s.Email, s.ConnectionID)
		}
		if s.Status != "skipped" {
			t.Errorf("status = %q, want skipped", s.Status)
		}
	}
}

func TestHealthyConnectionBadExpiry(t *testing.T) {
	now := time.Now()
	conns := []domain.ProviderConnection{
		{ID: "bad-exp", Provider: "openai", AuthType: "oauth", Email: "a@x.com", IsActive: true, RefreshToken: "rt", ExpiresAt: "not-a-time"},
	}
	if _, ok := healthyConnection(conns, "a@x.com", now); ok {
		t.Error("connection with unparseable expiry must not be healthy")
	}
}
