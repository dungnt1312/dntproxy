package service

import (
	"errors"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

func glmTestConn(id string, priority int) domain.ProviderConnection {
	return domain.ProviderConnection{
		ID:              id,
		Name:            id,
		Provider:        "glm",
		IsActive:        true,
		AuthType:        "apikey",
		Priority:        priority,
		Weight:          100,
		SupportedModels: []string{"glm-5.3-flash"},
	}
}

// GLM reports an exhausted balance as a 400. That used to be classified as a
// client error, so the request failed without any failover and nothing was
// recorded — the next request picked the same broke account again. It must now
// rotate to the next account and sideline the broke one.
func TestHandleChat_PaymentExhausted400RotatesAccountAndCools(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			glmTestConn("lite", 1),
			glmTestConn("pro", 2),
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"lite|glm-5.3-flash": {
			Status: 400,
			Err:    errors.New(`returned 400: {"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`),
		},
		"pro|glm-5.3-flash": {Status: 200, Body: "data: [DONE]\n\n"},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("glm", exec)

	svc := NewChatService(store, registry)

	result := svc.HandleChat([]byte(`{"model":"glm/glm-5.3-flash","messages":[]}`), "glm/glm-5.3-flash", "req-glm-1", nil)
	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected fallback success, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()

	if len(exec.calls) != 2 {
		t.Fatalf("expected 2 attempts (lite then pro), got %d: %+v", len(exec.calls), exec.calls)
	}

	lite, _ := store.GetConnectionByID("lite")
	if lite == nil || lite.RateLimitedUntil == "" {
		t.Fatalf("lite must be sidelined after a payment exhaustion, conn=%+v", lite)
	}
	if !domain.IsAccountUnavailable(lite.RateLimitedUntil) {
		t.Fatalf("lite cooldown must be in the future, got %q", lite.RateLimitedUntil)
	}
	if !domain.IsStickyAccountCooldown(lite.LastError) {
		t.Fatalf("lite LastError %q must be sticky", lite.LastError)
	}

	// The next request must skip the broke account entirely.
	again := svc.HandleChat([]byte(`{"model":"glm/glm-5.3-flash","messages":[]}`), "glm/glm-5.3-flash", "req-glm-2", nil)
	if again.StatusCode != 200 || again.Stream == nil {
		t.Fatalf("second request: status=%d err=%q", again.StatusCode, again.Error)
	}
	again.Stream.Close()

	if len(exec.calls) != 3 {
		t.Fatalf("second request must not retry the cooled account, calls=%+v", exec.calls)
	}
	if exec.calls[2].ConnectionID != "pro" {
		t.Fatalf("second request went to %q, want pro", exec.calls[2].ConnectionID)
	}
}

// A banned account (403 封禁) is account-level: it must not keep serving other
// models while only the failing model is locked.
func TestHandleChat_BannedAccountSidelinedForAllModels(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			glmTestConn("banned", 1),
			glmTestConn("healthy", 2),
		},
		Settings: domain.Settings{ConnectionStrategy: ConnectionStrategyFillFirst},
	})

	exec := newFakeExecutor(map[string]fakeExecuteResponse{
		"banned|glm-5.3-flash": {
			Status: 403,
			Err:    errors.New(`returned 403: {"error":{"code":"1112","message":"您的账户已被封禁"}}`),
		},
		"healthy|glm-5.3-flash": {Status: 200, Body: "data: [DONE]\n\n"},
	})

	registry := newTestProviderRegistry()
	registry.RegisterExecutor("glm", exec)

	svc := NewChatService(store, registry)

	result := svc.HandleChat([]byte(`{"model":"glm/glm-5.3-flash","messages":[]}`), "glm/glm-5.3-flash", "req-ban-1", nil)
	if result.StatusCode != 200 || result.Stream == nil {
		t.Fatalf("expected fallback success, got status=%d err=%q", result.StatusCode, result.Error)
	}
	result.Stream.Close()

	banned, _ := store.GetConnectionByID("banned")
	if banned == nil || banned.RateLimitedUntil == "" {
		t.Fatalf("banned account must be sidelined at account level, conn=%+v", banned)
	}
	if domain.IsAccountUnavailable(banned.RateLimitedUntil) == false {
		t.Fatalf("banned cooldown must be in the future, got %q", banned.RateLimitedUntil)
	}
}

// A later success on the same connection must not erase a hard quota cooldown:
// with concurrent requests a slow success can finish after another request
// recorded the exhaustion, and clearing it would put the spent account straight
// back into rotation.
func TestClearError_PreservesStickyCooldown(t *testing.T) {
	reset := time.Now().Add(90 * time.Minute).UTC().Format(time.RFC3339)
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{
				ID:               "spent",
				Name:             "spent",
				Provider:         "glm",
				IsActive:         true,
				AuthType:         "apikey",
				Weight:           100,
				SupportedModels:  []string{"glm-5.3-flash"},
				RateLimitedUntil: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
				LastError:        `{"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`,
				LastErrorAt:      time.Now().UTC().Format(time.RFC3339),
				BackoffLevel:     domain.BackoffMaxLevel,
				ModelLocks:       map[string]string{"glm-5.3-flash": reset},
			},
		},
	})
	sel := NewAccountSelector(store)

	if err := sel.ClearError("spent", "glm-5.3-flash"); err != nil {
		t.Fatalf("ClearError: %v", err)
	}

	conn, _ := store.GetConnectionByID("spent")
	if conn == nil {
		t.Fatal("connection missing")
	}
	if conn.RateLimitedUntil == "" {
		t.Fatal("sticky account cooldown must survive a success")
	}
	if conn.BackoffLevel != domain.BackoffMaxLevel {
		t.Fatalf("BackoffLevel = %d, want %d", conn.BackoffLevel, domain.BackoffMaxLevel)
	}
	if conn.LastError == "" {
		t.Fatal("LastError must be kept so the operator can see why")
	}
	// The model that just worked is no longer suspect.
	if domain.IsModelLockActive(conn.ModelLocks, "glm-5.3-flash") {
		t.Fatalf("the successful model lock should be cleared, got %#v", conn.ModelLocks)
	}
}

// A plain transient failure keeps the old behavior: a success clears it.
func TestClearError_StillClearsTransientCooldown(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			{
				ID:               "flaky",
				Name:             "flaky",
				Provider:         "glm",
				IsActive:         true,
				AuthType:         "apikey",
				Weight:           100,
				SupportedModels:  []string{"glm-5.3-flash"},
				RateLimitedUntil: time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339),
				LastError:        "http2: response body closed",
				LastErrorAt:      time.Now().UTC().Format(time.RFC3339),
				BackoffLevel:     2,
			},
		},
	})
	sel := NewAccountSelector(store)

	if err := sel.ClearError("flaky", "glm-5.3-flash"); err != nil {
		t.Fatalf("ClearError: %v", err)
	}

	conn, _ := store.GetConnectionByID("flaky")
	if conn == nil {
		t.Fatal("connection missing")
	}
	if conn.RateLimitedUntil != "" || conn.BackoffLevel != 0 || conn.LastError != "" {
		t.Fatalf("transient cooldown should be cleared, got %+v", conn)
	}
}
