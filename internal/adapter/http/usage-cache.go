package http

import (
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
)

// Server-side quota cache. Every dashboard tab, Quota-auto poller and user
// shares it, so provider quota APIs are hit at most once per connection per TTL
// no matter how many viewers there are. An explicit refresh (?refresh=1, the
// dashboard's Refresh buttons) always goes to the provider.
//
// It lives in memory only: a restart starts empty, and the next refresh fills it.
const (
	// usageCacheTTL is how long a fetched quota is served without asking the
	// provider again.
	usageCacheTTL = 2 * time.Minute
	// usageCacheMaxAge bounds how old an entry may be and still be shown on
	// page load (with its age); older entries are dropped.
	usageCacheMaxAge = 24 * time.Hour
)

type usageCacheEntry struct {
	resp      *UsageResponse
	fetchedAt time.Time
}

// view returns a copy of the cached response stamped with its fetch time.
func (e usageCacheEntry) view(cached bool) UsageResponse {
	out := *e.resp
	out.FetchedAt = e.fetchedAt.UTC().Format(time.RFC3339Nano)
	out.Cached = cached
	return out
}

type usageCache struct {
	mu    sync.RWMutex
	items map[string]usageCacheEntry
	sf    singleflight.Group // one in-flight provider call per connection
	now   func() time.Time
}

func newUsageCache() *usageCache {
	return &usageCache{items: make(map[string]usageCacheEntry), now: time.Now}
}

// sharedUsageCache backs GET /usage/:id and GET /usage/cached.
var sharedUsageCache = newUsageCache()

// fresh returns an entry still inside the TTL.
func (c *usageCache) fresh(id string) (usageCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[id]
	if !ok || c.now().Sub(e.fetchedAt) >= usageCacheTTL {
		return usageCacheEntry{}, false
	}
	return e, true
}

// last returns the newest entry regardless of TTL (for display with its age).
func (c *usageCache) last(id string) (usageCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[id]
	if !ok || c.now().Sub(e.fetchedAt) >= usageCacheMaxAge {
		return usageCacheEntry{}, false
	}
	return e, true
}

func (c *usageCache) put(id string, resp *UsageResponse) usageCacheEntry {
	e := usageCacheEntry{resp: resp, fetchedAt: c.now()}
	c.mu.Lock()
	c.items[id] = e
	c.mu.Unlock()
	return e
}

// prune drops entries for connections that no longer exist or are too old.
func (c *usageCache) prune(existing []domain.ProviderConnection) {
	keep := make(map[string]bool, len(existing))
	for _, conn := range existing {
		keep[conn.ID] = true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, e := range c.items {
		if !keep[id] || c.now().Sub(e.fetchedAt) >= usageCacheMaxAge {
			delete(c.items, id)
		}
	}
}

// apiGetCachedUsage is GET /api/usage/cached: the last known quota of every
// connection the caller can see, straight from the cache. It never calls a
// provider, so the dashboard can show quotas the moment it loads.
func apiGetCachedUsage(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil || cfg == nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}
		sharedUsageCache.prune(cfg.ProviderConnections)

		entries := make(map[string]UsageResponse)
		for _, conn := range domain.FilterConnectionsByTenant(cfg.ProviderConnections, GetTenantID(c)) {
			if e, ok := sharedUsageCache.last(conn.ID); ok {
				entries[conn.ID] = e.view(true)
			}
		}
		c.JSON(200, gin.H{"entries": entries, "ttlSeconds": int(usageCacheTTL / time.Second)})
	}
}
