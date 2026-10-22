package auth

import (
	"io"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
)

func newRefreshStore(t *testing.T, conns ...domain.ProviderConnection) *storage.JsonDB {
	t.Helper()
	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.DefaultConfig()
	cfg.ProviderConnections = conns
	if err := db.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	return db
}

func idcConn(id string, expiresIn time.Duration, pd map[string]interface{}) domain.ProviderConnection {
	base := map[string]interface{}{"authMethod": "idc", "clientId": "cid", "clientSecret": "sec", "region": "us-east-1"}
	for k, v := range pd {
		base[k] = v
	}
	return domain.ProviderConnection{
		ID: id, Name: id, Provider: "kiro", AuthType: "oauth", IsActive: true,
		AccessToken: "old-at", RefreshToken: "rt-0",
		ExpiresAt:            time.Now().Add(expiresIn).UTC().Format(time.RFC3339),
		ProviderSpecificData: base,
	}
}

func TestKiroRefreshThresholdIsTenMinutes(t *testing.T) {
	svc := NewTokenRefreshService(nil)
	soon := idcConn("a", 8*time.Minute, nil)
	later := idcConn("b", 12*time.Minute, nil)
	if !svc.NeedsRefresh(&soon) || svc.NeedsRefresh(&later) {
		t.Fatal("kiro refreshes inside 10 minutes only")
	}
	// Other providers keep the 5 minute buffer.
	other := soon
	other.Provider = "openai"
	if svc.NeedsRefresh(&other) {
		t.Fatal("non-kiro provider keeps its 5 minute buffer")
	}
}

func TestClientRegistrationExpiry(t *testing.T) {
	soon := time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)
	far := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if !clientRegistrationExpired(map[string]interface{}{"clientSecretExpiresAt": soon}) {
		t.Fatal("expiry inside 15 minutes counts as expired")
	}
	if clientRegistrationExpired(map[string]interface{}{"clientSecretExpiresAt": far}) {
		t.Fatal("far expiry is fine")
	}
	if clientRegistrationExpired(map[string]interface{}{}) || clientRegistrationExpired(nil) {
		t.Fatal("legacy connections without an expiry are not rejected")
	}
}

func TestRefreshIdCUsesSSOAndPersists(t *testing.T) {
	var gotBody string
	withOIDC(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = io.WriteString(w, `{"accessToken":"new-at","refreshToken":"rt-1","expiresIn":3600}`)
	}))
	store := newRefreshStore(t, idcConn("c1", time.Minute, nil))
	conn, _ := store.GetConnectionByID("c1")

	out, err := NewTokenRefreshService(store).ForceRefresh(conn)
	if err != nil || out.AccessToken != "new-at" || out.RefreshToken != "rt-1" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if !contains(gotBody, `"grantType":"refresh_token"`) || !contains(gotBody, `"refreshToken":"rt-0"`) {
		t.Fatalf("body = %s", gotBody)
	}
	saved, _ := store.GetConnectionByID("c1")
	if saved.AccessToken != "new-at" || saved.RefreshToken != "rt-1" {
		t.Fatalf("saved = %+v", saved)
	}
	if saved.ProviderSpecificData["clientId"] != "cid" {
		t.Fatal("persisting a refresh must not drop provider data")
	}
}

func TestRefreshRejectsExpiredClientRegistrationWithoutNetwork(t *testing.T) {
	var hits atomic.Int32
	withOIDC(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	expired := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	store := newRefreshStore(t, idcConn("c2", time.Minute, map[string]interface{}{"clientSecretExpiresAt": expired}))
	conn, _ := store.GetConnectionByID("c2")

	_, err := NewTokenRefreshService(store).ForceRefresh(conn)
	if !IsDeadRefreshError(err) || hits.Load() != 0 {
		t.Fatalf("err=%v hits=%d", err, hits.Load())
	}
}

func TestRefreshBackoffRejectsFromCache(t *testing.T) {
	var hits atomic.Int32
	withOIDC(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(400)
		_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
	}))
	kiroBackoff.reset("c3")
	store := newRefreshStore(t, idcConn("c3", time.Minute, nil))
	svc := NewTokenRefreshService(store)

	conn, _ := store.GetConnectionByID("c3")
	if _, err := svc.ForceRefresh(conn); err == nil {
		t.Fatal("first refresh should fail")
	}
	first := hits.Load()
	if first == 0 {
		t.Fatal("first refresh must reach the network")
	}
	for i := 0; i < 3; i++ {
		if _, err := svc.ForceRefresh(conn); err == nil || !IsDeadRefreshError(err) {
			t.Fatalf("cooldown must replay the original error, got %v", err)
		}
	}
	if hits.Load() != first {
		t.Fatalf("cooldown leaked %d extra network calls", hits.Load()-first)
	}

	// A new refresh token (re-auth / import) clears the cooldown.
	_ = store.Update(func(cfg *domain.AppConfig) { cfg.ProviderConnections[0].RefreshToken = "rt-new" })
	conn2, _ := store.GetConnectionByID("c3")
	_, _ = svc.ForceRefresh(conn2)
	if hits.Load() == first {
		t.Fatal("a new refresh token must bypass the cooldown")
	}
	kiroBackoff.reset("c3")
}

func TestBackoffDelayDoublesFiveToSixty(t *testing.T) {
	want := []time.Duration{5, 10, 20, 40, 60, 60}
	for i, w := range want {
		if got := backoffDelay(i + 1); got != w*time.Second {
			t.Errorf("failures=%d: %v, want %v", i+1, got, w*time.Second)
		}
	}
}

func TestRefreshSupersededByConcurrentRefresh(t *testing.T) {
	// AWS rejects our stale refresh token, but another worker already stored a
	// fresh pair: Kiro uses it instead of failing.
	var store *storage.JsonDB
	withOIDC(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = store.Update(func(cfg *domain.AppConfig) {
			cfg.ProviderConnections[0].RefreshToken = "rt-other"
			cfg.ProviderConnections[0].AccessToken = "at-other"
			cfg.ProviderConnections[0].ExpiresAt = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
		})
		w.WriteHeader(400)
		_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
	}))
	kiroBackoff.reset("c4")
	store = newRefreshStore(t, idcConn("c4", time.Minute, nil))
	conn, _ := store.GetConnectionByID("c4")

	out, err := NewTokenRefreshService(store).ForceRefresh(conn)
	if err != nil || out.AccessToken != "at-other" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}

func TestPersistDoesNotOverwriteNewerRefreshToken(t *testing.T) {
	store := newRefreshStore(t, idcConn("c5", time.Minute, nil))
	_ = store.Update(func(cfg *domain.AppConfig) { cfg.ProviderConnections[0].RefreshToken = "rt-newer" })
	updated := idcConn("c5", time.Hour, nil)
	updated.AccessToken, updated.RefreshToken = "stale-at", "stale-rt"
	if err := persistKiroRefresh(store, &updated, "rt-0"); err != nil {
		t.Fatal(err)
	}
	got, _ := store.GetConnectionByID("c5")
	if got.RefreshToken != "rt-newer" || got.AccessToken == "stale-at" {
		t.Fatalf("a stale refresh must not clobber newer credentials: %+v", got)
	}
}

func TestRefreshSocialUsesAuthServiceAndKeepsProfileArn(t *testing.T) {
	var gotUA, gotBody string
	srv := newAuthService(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if r.URL.Path != "/refreshToken" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"accessToken":"s-at","refreshToken":"s-rt","expiresIn":900,"profileArn":"arn:from-service"}`)
	})
	_ = srv
	kiroBackoff.reset("c6")
	social := domain.ProviderConnection{
		ID: "c6", Name: "c6", Provider: "kiro", AuthType: "oauth", IsActive: true,
		AccessToken: "x", RefreshToken: "s-rt-0",
		ExpiresAt:            time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
		ProviderSpecificData: map[string]interface{}{"authMethod": "google", "profileArn": "arn:stored"},
	}
	store := newRefreshStore(t, social)
	conn, _ := store.GetConnectionByID("c6")
	out, err := NewTokenRefreshService(store).ForceRefresh(conn)
	if err != nil || out.AccessToken != "s-at" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if !contains(gotBody, `"refreshToken":"s-rt-0"`) || !hasPrefix(gotUA, "KiroIDE-") {
		t.Fatalf("body=%s ua=%q", gotBody, gotUA)
	}
	if out.ProviderSpecificData["profileArn"] != "arn:stored" {
		t.Fatalf("stored profileArn must win, got %v", out.ProviderSpecificData["profileArn"])
	}
}
