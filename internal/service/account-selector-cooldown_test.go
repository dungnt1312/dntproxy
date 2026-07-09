package service

import (
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

func boolPtr(v bool) *bool { return &v }

func cooldownTestConn(id string, backoff int, psd map[string]interface{}) domain.ProviderConnection {
	return domain.ProviderConnection{
		ID:                   id,
		Provider:             "openai",
		Name:                 id,
		IsActive:             true,
		AuthType:             "api-key",
		SupportedModels:      []string{"gpt"},
		BackoffLevel:         backoff,
		ProviderSpecificData: psd,
	}
}

func TestMarkUnavailableHonorsDisableCooling(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			cooldownTestConn("conn-a", 0, map[string]interface{}{"disableCooling": true}),
		},
		Settings: domain.Settings{},
	})
	sel := NewAccountSelector(store)

	if err := sel.MarkUnavailable("conn-a", 429, "rate limit", "gpt"); err != nil {
		t.Fatalf("MarkUnavailable: %v", err)
	}

	conn, err := store.GetConnectionByID("conn-a")
	if err != nil || conn == nil {
		t.Fatalf("get conn: %v %v", conn, err)
	}
	if conn.RateLimitedUntil != "" {
		t.Fatalf("RateLimitedUntil want empty, got %q", conn.RateLimitedUntil)
	}
	if len(conn.ModelLocks) != 0 {
		t.Fatalf("ModelLocks want empty, got %#v", conn.ModelLocks)
	}
	if conn.BackoffLevel != 0 {
		t.Fatalf("BackoffLevel want 0, got %d", conn.BackoffLevel)
	}
}

func TestMarkUnavailableHonorsCooldownDisabled(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			cooldownTestConn("conn-b", 0, nil),
		},
		Settings: domain.Settings{
			CooldownEnabled: boolPtr(false),
		},
	})
	sel := NewAccountSelector(store)

	if err := sel.MarkUnavailable("conn-b", 503, "", "gpt"); err != nil {
		t.Fatalf("MarkUnavailable: %v", err)
	}

	conn, err := store.GetConnectionByID("conn-b")
	if err != nil || conn == nil {
		t.Fatalf("get conn: %v %v", conn, err)
	}
	if conn.RateLimitedUntil != "" {
		t.Fatalf("RateLimitedUntil want empty, got %q", conn.RateLimitedUntil)
	}
	if len(conn.ModelLocks) != 0 {
		t.Fatalf("ModelLocks want empty, got %#v", conn.ModelLocks)
	}
}

func TestMarkUnavailableClampsMaxCooldown(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			// High backoff so quota cooldown >> 1s
			cooldownTestConn("conn-c", domain.BackoffMaxLevel, nil),
		},
		Settings: domain.Settings{
			MaxCooldownSeconds: 1,
		},
	})
	sel := NewAccountSelector(store)
	before := time.Now()

	if err := sel.MarkUnavailable("conn-c", 429, "quota exceeded", "gpt"); err != nil {
		t.Fatalf("MarkUnavailable: %v", err)
	}

	conn, err := store.GetConnectionByID("conn-c")
	if err != nil || conn == nil {
		t.Fatalf("get conn: %v %v", conn, err)
	}
	if conn.RateLimitedUntil == "" {
		t.Fatal("RateLimitedUntil want set")
	}
	until, err := time.Parse(time.RFC3339, conn.RateLimitedUntil)
	if err != nil {
		t.Fatalf("parse RateLimitedUntil: %v", err)
	}
	// Must be <= now+1s+slack (250ms). CooldownUntil formats RFC3339 without
	// fractional seconds, so allow truncation slack on the lower bound.
	maxAllowed := before.Add(time.Second + 250*time.Millisecond)
	if until.After(maxAllowed) {
		t.Fatalf("cooldown until %v exceeds clamp max %v (elapsed %v)", until, maxAllowed, until.Sub(before))
	}
	// Unclamped GetQuotaCooldown(BackoffMaxLevel) is 120s; ensure we actually cooled down.
	if !until.After(before.Add(-time.Second)) {
		t.Fatalf("cooldown until %v not after before %v", until, before)
	}
	// Duration must be well under the unclamped 120s quota backoff.
	if until.Sub(before) > 2*time.Second {
		t.Fatalf("cooldown duration %v not clamped to ~1s", until.Sub(before))
	}
}

func TestMarkUnavailableTransientOverrideSeconds(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			cooldownTestConn("conn-d", 0, nil),
		},
		Settings: domain.Settings{
			TransientCooldownSeconds: 5,
		},
	})
	sel := NewAccountSelector(store)
	before := time.Now()

	if err := sel.MarkUnavailable("conn-d", 503, "", "gpt"); err != nil {
		t.Fatalf("MarkUnavailable: %v", err)
	}

	conn, err := store.GetConnectionByID("conn-d")
	if err != nil || conn == nil {
		t.Fatalf("get conn: %v %v", conn, err)
	}
	if conn.RateLimitedUntil == "" {
		t.Fatal("RateLimitedUntil want set")
	}
	until, err := time.Parse(time.RFC3339, conn.RateLimitedUntil)
	if err != nil {
		t.Fatalf("parse RateLimitedUntil: %v", err)
	}
	dur := until.Sub(before)
	// ~5s, not legacy 2s. Allow slack for scheduling.
	if dur < 4*time.Second || dur > 6*time.Second {
		t.Fatalf("transient cooldown duration %v want ~5s (not legacy 2s)", dur)
	}
}

func TestMarkUnavailableModelLockOff(t *testing.T) {
	store := newTestCredentialStore(&domain.AppConfig{
		ProviderConnections: []domain.ProviderConnection{
			cooldownTestConn("conn-e", 0, nil),
		},
		Settings: domain.Settings{
			ModelLockEnabled: boolPtr(false),
		},
	})
	sel := NewAccountSelector(store)

	// Model entitlement uses ModelOnly cooldown; with ModelLockOff no ModelLocks write.
	if err := sel.MarkUnavailable("conn-e", 403, "model_not_entitled", "gpt"); err != nil {
		t.Fatalf("MarkUnavailable: %v", err)
	}

	conn, err := store.GetConnectionByID("conn-e")
	if err != nil || conn == nil {
		t.Fatalf("get conn: %v %v", conn, err)
	}
	if len(conn.ModelLocks) != 0 {
		t.Fatalf("ModelLocks want empty when ModelLockOff, got %#v", conn.ModelLocks)
	}
	// ModelOnly still should not set RateLimitedUntil
	if conn.RateLimitedUntil != "" {
		t.Fatalf("RateLimitedUntil want empty for ModelOnly, got %q", conn.RateLimitedUntil)
	}
}

func TestConnectionDisableCooling(t *testing.T) {
	if connectionDisableCooling(nil) {
		t.Fatal("nil conn should be false")
	}
	if connectionDisableCooling(&domain.ProviderConnection{}) {
		t.Fatal("empty PSD should be false")
	}
	if connectionDisableCooling(&domain.ProviderConnection{
		ProviderSpecificData: map[string]interface{}{"disableCooling": false},
	}) {
		t.Fatal("false should be false")
	}
	if !connectionDisableCooling(&domain.ProviderConnection{
		ProviderSpecificData: map[string]interface{}{"disableCooling": true},
	}) {
		t.Fatal("true should be true")
	}
}

func TestClampCooldownMs(t *testing.T) {
	if got := clampCooldownMs(5000, 0); got != 5000 {
		t.Fatalf("max=0 passthrough: got %d", got)
	}
	if got := clampCooldownMs(5000, 1); got != 1000 {
		t.Fatalf("clamp 5s to 1s: got %d", got)
	}
	if got := clampCooldownMs(500, 1); got != 500 {
		t.Fatalf("under max unchanged: got %d", got)
	}
	if got := clampCooldownMs(0, 1); got != 0 {
		t.Fatalf("zero ms: got %d", got)
	}
}
