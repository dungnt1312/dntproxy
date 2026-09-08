package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

// fakeRefresher lets tests control ForceRefresh outcomes per connection.
type fakeRefresher struct {
	// refreshErr returns an error for the given connection (nil = success).
	refreshErr func(connID string) error
	// shouldRefresh overrides ShouldProactivelyRefresh (default: true for oauth).
	shouldRefresh func(conn *domain.ProviderConnection) bool
	calls         []string
}

func (f *fakeRefresher) ForceRefresh(conn *domain.ProviderConnection) (*domain.ProviderConnection, error) {
	f.calls = append(f.calls, conn.ID)
	if f.refreshErr != nil {
		if err := f.refreshErr(conn.ID); err != nil {
			return nil, err
		}
	}
	// Simulate a successful refresh: fresh expiry far in the future.
	out := *conn
	out.ExpiresAt = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	return &out, nil
}

func (f *fakeRefresher) ShouldProactivelyRefresh(conn *domain.ProviderConnection) bool {
	if f.shouldRefresh != nil {
		return f.shouldRefresh(conn)
	}
	return conn.AuthType == "oauth"
}

func oauthTestConn(id string, weight int, expiredAt string) domain.ProviderConnection {
	return domain.ProviderConnection{
		ID:              id,
		Provider:        "openai",
		Name:            id,
		IsActive:        true,
		AuthType:        "oauth",
		Weight:          weight,
		Priority:        0,
		AccessToken:     "at_" + id,
		RefreshToken:    "rt_" + id,
		ExpiresAt:       expiredAt,
		SupportedModels: []string{"gpt-5.6-terra"},
	}
}

func oauthExpired(t time.Time) string {
	return t.Add(-time.Hour).UTC().Format(time.RFC3339)
}

// SelectCredentials must skip a candidate whose OAuth refresh fails (dead
// refresh token) and fall through to the next healthy candidate instead of
// aborting the whole request with a raw refresh error.
func TestSelectCredentialsSkipsDeadRefreshTokenAccount(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			oauthTestConn("dead-a", 100, oauthExpired(time.Now())), // fill-first pick
			oauthTestConn("good-b", 100, oauthExpired(time.Now())),
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})

	refresher := &fakeRefresher{
		refreshErr: func(connID string) error {
			if connID == "dead-a" {
				return errors.New("openai refresh failed: token refresh failed (401): invalid refresh token")
			}
			return nil
		},
	}
	sel := NewAccountSelector(store)
	sel.tokenRefresh = refresher

	creds, err := sel.SelectCredentials("openai", nil, "gpt-5.6-terra", nil)
	if err != nil {
		t.Fatalf("SelectCredentials: %v", err)
	}
	if creds.ConnectionID != "good-b" {
		t.Fatalf("want fallback to good-b, got %q", creds.ConnectionID)
	}
	if len(refresher.calls) != 2 || refresher.calls[0] != "dead-a" || refresher.calls[1] != "good-b" {
		t.Fatalf("refresh calls = %v, want [dead-a good-b]", refresher.calls)
	}

	// dead-a must now be sidelined so subsequent requests skip it.
	conn, err := store.GetConnectionByID("dead-a")
	if err != nil || conn == nil {
		t.Fatalf("get dead-a: %v %v", conn, err)
	}
	if !domain.IsAccountUnavailable(conn.RateLimitedUntil) {
		t.Fatalf("dead-a RateLimitedUntil=%q want active cooldown", conn.RateLimitedUntil)
	}
}

// When every candidate's refresh fails, SelectCredentials must return an
// AccountSelectionError (not a raw refresh error) so chat-service can map it.
func TestSelectCredentialsAllRefreshFailReturnsSelectionError(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			oauthTestConn("dead-a", 100, oauthExpired(time.Now())),
			oauthTestConn("dead-b", 100, oauthExpired(time.Now())),
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})

	refresher := &fakeRefresher{
		refreshErr: func(connID string) error {
			return errors.New("openai refresh failed: token refresh failed (401): invalid refresh token")
		},
	}
	sel := NewAccountSelector(store)
	sel.tokenRefresh = refresher

	_, err := sel.SelectCredentials("openai", nil, "gpt-5.6-terra", nil)
	if err == nil {
		t.Fatal("want error when all accounts fail refresh")
	}
	var selErr *AccountSelectionError
	if !errors.As(err, &selErr) {
		t.Fatalf("want AccountSelectionError, got %T: %v", err, err)
	}
	if selErr.Kind != SelectionErrUnavailable {
		t.Fatalf("kind = %q, want %q", selErr.Kind, SelectionErrUnavailable)
	}
}

// A transient (non-auth) refresh failure should also move to the next
// candidate, and the failed account gets a (short) cooldown rather than none.
func TestSelectCredentialsSkipsTransientRefreshFailure(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			oauthTestConn("flaky-a", 100, oauthExpired(time.Now())),
			oauthTestConn("good-b", 100, oauthExpired(time.Now())),
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})

	refresher := &fakeRefresher{
		refreshErr: func(connID string) error {
			if connID == "flaky-a" {
				return errors.New("openai refresh failed: request failed: Post timeout")
			}
			return nil
		},
	}
	sel := NewAccountSelector(store)
	sel.tokenRefresh = refresher

	creds, err := sel.SelectCredentials("openai", nil, "gpt-5.6-terra", nil)
	if err != nil {
		t.Fatalf("SelectCredentials: %v", err)
	}
	if creds.ConnectionID != "good-b" {
		t.Fatalf("want fallback to good-b, got %q", creds.ConnectionID)
	}
}

// Pinned selection must sideline a dead-token connection and fail fast
// (never spills over to another connection).
func TestSelectPinnedConnectionDeadRefreshSidelines(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			oauthTestConn("pin-a", 100, oauthExpired(time.Now())),
			oauthTestConn("other-b", 100, oauthExpired(time.Now())),
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})

	refresher := &fakeRefresher{
		refreshErr: func(connID string) error {
			return errors.New("openai refresh failed: token refresh failed (401): invalid refresh token")
		},
	}
	sel := NewAccountSelector(store)
	sel.tokenRefresh = refresher

	_, err := sel.SelectCredentialsForModel("openai/gpt-5.6-terra@pin-a", nil, nil, "")
	if err == nil {
		t.Fatal("want error for pinned dead-token connection")
	}
	var selErr *AccountSelectionError
	if !errors.As(err, &selErr) {
		t.Fatalf("want AccountSelectionError, got %T: %v", err, err)
	}

	conn, err := store.GetConnectionByID("pin-a")
	if err != nil || conn == nil {
		t.Fatalf("get pin-a: %v %v", conn, err)
	}
	if !domain.IsAccountUnavailable(conn.RateLimitedUntil) {
		t.Fatalf("pin-a RateLimitedUntil=%q want active cooldown", conn.RateLimitedUntil)
	}
}

// Sanity: a healthy candidate with a fresh token is returned with no refresh
// call at all.
func TestSelectCredentialsFreshTokenNoRefresh(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			oauthTestConn("fresh-a", 100, time.Now().Add(24*time.Hour).UTC().Format(time.RFC3339)),
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})
	refresher := &fakeRefresher{
		shouldRefresh: func(*domain.ProviderConnection) bool { return false },
	}
	sel := NewAccountSelector(store)
	sel.tokenRefresh = refresher

	creds, err := sel.SelectCredentials("openai", nil, "gpt-5.6-terra", nil)
	if err != nil {
		t.Fatalf("SelectCredentials: %v", err)
	}
	if creds.ConnectionID != "fresh-a" {
		t.Fatalf("want fresh-a, got %q", creds.ConnectionID)
	}
	if len(refresher.calls) != 0 {
		t.Fatalf("refresh calls = %v, want none", refresher.calls)
	}
}

func TestRemoveConnection(t *testing.T) {
	conns := []domain.ProviderConnection{
		{ID: "a"}, {ID: "b"}, {ID: "c"},
	}
	out := removeConnection(conns, "b")
	if len(out) != 2 || out[0].ID != "a" || out[1].ID != "c" {
		t.Fatalf("removeConnection = %#v", out)
	}
	if len(conns) != 3 {
		t.Fatalf("input mutated: %#v", conns)
	}
}

func TestIsDeadRefreshToken(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{fmt.Errorf("openai refresh failed: token refresh failed (401): %s", `{"error":{"message":"Invalid refresh token.","code":"invalid_refresh_token"}}`), true},
		{fmt.Errorf("token refresh failed (401): invalid_grant"), true},
		{fmt.Errorf("request failed: Post \"https://auth.openai.com\": dial tcp timeout"), false},
		{nil, false},
	}
	for i, tc := range cases {
		if got := isDeadRefreshToken(tc.err); got != tc.want {
			t.Fatalf("case %d: isDeadRefreshToken(%v) = %v, want %v", i, tc.err, got, tc.want)
		}
	}
}
