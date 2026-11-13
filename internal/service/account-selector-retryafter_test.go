package service

import (
	"errors"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

// When every account is cooling, the caller should be told when the pool
// reopens (Retry-After) instead of only "all accounts rate limited".
func TestSelectCredentials_RateLimitedPoolReportsRetryAfter(t *testing.T) {
	soonest := time.Now().Add(20 * time.Minute).UTC().Format(time.RFC3339)
	later := time.Now().Add(50 * time.Minute).UTC().Format(time.RFC3339)

	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "a", Name: "a", Provider: "glm", IsActive: true, AuthType: "apikey", Weight: 100,
				SupportedModels: []string{"glm-5.3-flash"}, RateLimitedUntil: later},
			{ID: "b", Name: "b", Provider: "glm", IsActive: true, AuthType: "apikey", Weight: 100,
				SupportedModels: []string{"glm-5.3-flash"}, RateLimitedUntil: soonest},
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})
	sel := NewAccountSelector(store)

	_, err := sel.SelectCredentials("glm", nil, "glm-5.3-flash", nil)
	if err == nil {
		t.Fatal("want a selection error when the whole pool is cooling")
	}
	if !IsAccountSelectionError(err, SelectionErrRateLimited) {
		t.Fatalf("kind error = %v", err)
	}
	var selErr *AccountSelectionError
	if !errors.As(err, &selErr) {
		t.Fatal("not an AccountSelectionError")
	}
	if selErr.RetryAfter != soonest {
		t.Fatalf("RetryAfter = %q, want the earliest reopen %q", selErr.RetryAfter, soonest)
	}
}

// A model-locked pool reports the lock expiry the same way.
func TestSelectCredentials_ModelLockedPoolReportsRetryAfter(t *testing.T) {
	until := time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{ID: "a", Name: "a", Provider: "glm", IsActive: true, AuthType: "apikey", Weight: 100,
				SupportedModels: []string{"glm-5.3-flash"}, ModelLocks: map[string]string{"glm-5.3-flash": until}},
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})
	sel := NewAccountSelector(store)

	_, err := sel.SelectCredentials("glm", nil, "glm-5.3-flash", nil)
	var selErr *AccountSelectionError
	if !errors.As(err, &selErr) || selErr.Kind != SelectionErrModelLocked {
		t.Fatalf("err = %v, want model_locked", err)
	}
	if selErr.RetryAfter != until {
		t.Fatalf("RetryAfter = %q, want %q", selErr.RetryAfter, until)
	}
}

// A transient blip recorded while a hard quota cooldown is active must not
// downgrade the account-level cooldown to a 2-second model lock: two requests
// racing would otherwise reopen a spent account seconds later.
func TestMarkUnavailable_WeakFailureDoesNotDowngradeStickyCooldown(t *testing.T) {
	stickyUntil := time.Now().Add(90 * time.Minute).UTC().Format(time.RFC3339)
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{
				ID: "spent", Name: "spent", Provider: "glm", IsActive: true, AuthType: "apikey", Weight: 100,
				SupportedModels:  []string{"glm-5.3-flash"},
				RateLimitedUntil: stickyUntil,
				LastError:        `{"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`,
				LastErrorAt:      time.Now().UTC().Format(time.RFC3339),
				BackoffLevel:     domain.BackoffMaxLevel,
			},
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})
	sel := NewAccountSelector(store)

	if err := sel.MarkUnavailable("spent", 502, "http2: response body closed", "glm-5.3-flash"); err != nil {
		t.Fatalf("MarkUnavailable: %v", err)
	}

	conn, _ := store.GetConnectionByID("spent")
	if conn.RateLimitedUntil != stickyUntil {
		t.Fatalf("RateLimitedUntil = %q, want the sticky cooldown %q", conn.RateLimitedUntil, stickyUntil)
	}
	if !domain.IsStickyAccountCooldown(conn.LastError) {
		t.Fatalf("LastError = %q, want the original quota reason", conn.LastError)
	}
	if conn.BackoffLevel != domain.BackoffMaxLevel {
		t.Fatalf("BackoffLevel = %d, want %d", conn.BackoffLevel, domain.BackoffMaxLevel)
	}
}

// Two account-level failures keep whichever cooldown lasts longer.
func TestMarkUnavailable_KeepsLongerStickyCooldown(t *testing.T) {
	existing := time.Now().Add(90 * time.Minute).UTC().Format(time.RFC3339)
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{
				ID: "bad", Name: "bad", Provider: "glm", IsActive: true, AuthType: "apikey", Weight: 100,
				SupportedModels:  []string{"glm-5.3-flash"},
				RateLimitedUntil: existing,
				LastError:        `{"error":{"code":"1113","message":"余额不足"}}`,
				LastErrorAt:      time.Now().UTC().Format(time.RFC3339),
			},
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})
	sel := NewAccountSelector(store)

	// A ban is sticky too, but its 30-minute window is shorter than the balance
	// cooldown still running, so the later deadline must win.
	if err := sel.MarkUnavailable("bad", 403, `{"error":{"code":"1112","message":"您的账户已被封禁"}}`, "glm-5.3-flash"); err != nil {
		t.Fatalf("MarkUnavailable: %v", err)
	}

	conn, _ := store.GetConnectionByID("bad")
	if conn.RateLimitedUntil != existing {
		t.Fatalf("RateLimitedUntil = %q, want the longer cooldown %q", conn.RateLimitedUntil, existing)
	}
}

// A sticky cooldown that already expired must not block recovery.
func TestMarkUnavailable_ExpiredStickyCooldownDoesNotBlockNewState(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{
				ID: "recovered", Name: "recovered", Provider: "glm", IsActive: true, AuthType: "apikey", Weight: 100,
				SupportedModels:  []string{"glm-5.3-flash"},
				RateLimitedUntil: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
				LastError:        `{"error":{"code":"1113","message":"余额不足"}}`,
				LastErrorAt:      time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
			},
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})
	sel := NewAccountSelector(store)

	if err := sel.MarkUnavailable("recovered", 429, "rate limit, please retry", "glm-5.3-flash"); err != nil {
		t.Fatalf("MarkUnavailable: %v", err)
	}

	conn, _ := store.GetConnectionByID("recovered")
	if conn.RateLimitedUntil == "" {
		t.Fatal("an expired cooldown must be replaced by the new one")
	}
	if domain.IsStickyAccountCooldown(conn.LastError) {
		t.Fatalf("LastError = %q, want the new short-term reason", conn.LastError)
	}
}
