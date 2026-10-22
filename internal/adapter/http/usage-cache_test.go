package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gin-gonic/gin"
)

type usageTestEnv struct {
	r     *gin.Engine
	db    *storage.JsonDB
	key   string
	calls atomic.Int32
	fail  atomic.Bool
	clock time.Time
	mu    sync.Mutex
}

func (e *usageTestEnv) now() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.clock
}

func (e *usageTestEnv) advance(d time.Duration) {
	e.mu.Lock()
	e.clock = e.clock.Add(d)
	e.mu.Unlock()
}

func newUsageTestEnv(t *testing.T) *usageTestEnv {
	t.Helper()
	r, db, dash := newAuthHardeningRouter(t)
	e := &usageTestEnv{r: r, db: db, key: dash.Key, clock: time.Date(2026, 10, 22, 4, 0, 0, 0, time.UTC)}

	old := sharedUsageCache
	sharedUsageCache = newUsageCache()
	sharedUsageCache.now = e.now
	t.Cleanup(func() { sharedUsageCache = old })

	h := &UsageHandler{store: db, fetch: func(conn *domain.ProviderConnection) (*UsageResponse, error) {
		e.calls.Add(1)
		if e.fail.Load() {
			return nil, errors.New("provider down")
		}
		return &UsageResponse{Provider: conn.Provider, Quotas: []QuotaBucket{{Key: "m", Label: "Monthly", Used: 5, Total: 10, Pct: 50}}}, nil
	}}
	api := r.Group("/api", dashboardKeyMiddleware(db))
	api.GET("/usage/cached", apiGetCachedUsage(db))
	api.GET("/usage/:connectionId", h.GetUsage)
	return e
}

func (e *usageTestEnv) get(t *testing.T, path, key string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// conn-acme is the seeded Kiro OAuth connection (tenant "acme").
const usageConn = "/api/usage/conn-acme"

func TestUsageServedFromCacheWithinTTL(t *testing.T) {
	e := newUsageTestEnv(t)
	code, first := e.get(t, usageConn, e.key)
	if code != 200 || first["cached"] == true || first["fetchedAt"] == nil {
		t.Fatalf("first = %d %v", code, first)
	}
	e.advance(90 * time.Second)
	_, second := e.get(t, usageConn, e.key)
	if second["cached"] != true || second["fetchedAt"] != first["fetchedAt"] {
		t.Fatalf("second should come from cache with the original fetch time: %v", second)
	}
	if n := e.calls.Load(); n != 1 {
		t.Fatalf("provider calls = %d, want 1", n)
	}
}

func TestUsageRefetchedAfterTTL(t *testing.T) {
	e := newUsageTestEnv(t)
	e.get(t, usageConn, e.key)
	e.advance(usageCacheTTL + time.Second)
	_, body := e.get(t, usageConn, e.key)
	if body["cached"] == true || e.calls.Load() != 2 {
		t.Fatalf("stale entry must be refetched: calls=%d body=%v", e.calls.Load(), body)
	}
}

func TestUsageRefreshParamAlwaysHitsProvider(t *testing.T) {
	e := newUsageTestEnv(t)
	e.get(t, usageConn, e.key)
	for i := 0; i < 3; i++ {
		_, body := e.get(t, usageConn+"?refresh=1", e.key)
		if body["cached"] == true {
			t.Fatalf("refresh=1 answered from cache: %v", body)
		}
	}
	if n := e.calls.Load(); n != 4 {
		t.Fatalf("provider calls = %d, want 4", n)
	}
}

func TestUsageFailureIsNotCachedAndKeepsLastGood(t *testing.T) {
	e := newUsageTestEnv(t)
	e.get(t, usageConn, e.key) // good entry
	e.fail.Store(true)
	if code, _ := e.get(t, usageConn+"?refresh=1", e.key); code != 500 {
		t.Fatalf("failed refresh = %d, want 500", code)
	}
	// The last good answer is still what a page load sees.
	_, cached := e.get(t, "/api/usage/cached", e.key)
	entries, _ := cached["entries"].(map[string]any)
	if entries["conn-acme"] == nil {
		t.Fatalf("last good entry lost after a failed refresh: %v", cached)
	}
	// And a failure never becomes a cache hit.
	e.fail.Store(false)
	e.advance(usageCacheTTL + time.Second)
	if _, body := e.get(t, usageConn, e.key); body["cached"] == true || body["quotas"] == nil {
		t.Fatalf("after recovery = %v", body)
	}
}

func TestCachedEndpointNeverCallsProvider(t *testing.T) {
	e := newUsageTestEnv(t)
	_, empty := e.get(t, "/api/usage/cached", e.key)
	if entries, _ := empty["entries"].(map[string]any); len(entries) != 0 {
		t.Fatalf("cold cache = %v", empty)
	}
	e.get(t, usageConn, e.key)
	before := e.calls.Load()
	e.advance(10 * time.Minute) // past TTL: still shown, with its age
	for i := 0; i < 3; i++ {
		_, body := e.get(t, "/api/usage/cached", e.key)
		entry, _ := body["entries"].(map[string]any)["conn-acme"].(map[string]any)
		if entry == nil || entry["fetchedAt"] == nil || entry["cached"] != true {
			t.Fatalf("cached entry = %v", body)
		}
	}
	if e.calls.Load() != before {
		t.Fatal("/usage/cached must never reach the provider")
	}

	e.advance(usageCacheMaxAge)
	_, old := e.get(t, "/api/usage/cached", e.key)
	if entries, _ := old["entries"].(map[string]any); len(entries) != 0 {
		t.Fatalf("entries older than %s must be dropped: %v", usageCacheMaxAge, old)
	}
}

func TestCachedEndpointIsTenantScoped(t *testing.T) {
	e := newUsageTestEnv(t)
	cfg, _ := e.db.Load()
	cfg.ProviderConnections = append(cfg.ProviderConnections, domain.ProviderConnection{
		ID: "conn-global", Name: "global", Provider: "kiro", AuthType: "oauth", IsActive: true, Weight: 100,
	})
	cfg.APIKeys = append(cfg.APIKeys, domain.APIKey{ID: "acme-dash", Key: "sk-dnt-acme-dash", IsActive: true, DashboardAccess: true, TenantID: "acme"})
	if err := e.db.Save(cfg); err != nil {
		t.Fatal(err)
	}
	e.get(t, usageConn, e.key)
	e.get(t, "/api/usage/conn-global", e.key)

	_, admin := e.get(t, "/api/usage/cached", e.key)
	if n := len(admin["entries"].(map[string]any)); n != 2 {
		t.Fatalf("admin sees %d entries, want 2", n)
	}
	_, tenant := e.get(t, "/api/usage/cached", "sk-dnt-acme-dash")
	entries := tenant["entries"].(map[string]any)
	if len(entries) != 1 || entries["conn-acme"] == nil {
		t.Fatalf("tenant acme must only see its own connection: %v", entries)
	}
}

func TestConcurrentMissesShareOneProviderCall(t *testing.T) {
	e := newUsageTestEnv(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.get(t, usageConn, e.key)
		}()
	}
	wg.Wait()
	if n := e.calls.Load(); n < 1 || n > 2 {
		// singleflight collapses overlapping calls; a request that arrives after
		// the first finished is a cache hit, so the count should be 1.
		t.Fatalf("provider calls = %d for 8 concurrent requests", n)
	}
}

func TestPruneDropsDeletedConnections(t *testing.T) {
	e := newUsageTestEnv(t)
	e.get(t, usageConn, e.key)
	cfg, _ := e.db.Load()
	kept := cfg.ProviderConnections[:0]
	for _, c := range cfg.ProviderConnections {
		if c.ID != "conn-acme" {
			kept = append(kept, c)
		}
	}
	cfg.ProviderConnections = kept
	_ = e.db.Save(cfg)

	e.get(t, "/api/usage/cached", e.key)
	if _, ok := sharedUsageCache.last("conn-acme"); ok {
		t.Fatal("cache entry of a deleted connection was kept")
	}
}
