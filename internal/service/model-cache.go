package service

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// ModelCache caches provider model lists to avoid repeated API calls.
type ModelCache struct {
	mu    sync.RWMutex
	cache map[string]*cachedModels // key: connectionID
	ttl   time.Duration
	group singleflight.Group       // dedup concurrent fetches
}

type cachedModels struct {
	modelIDs  []string
	fetchedAt time.Time
}

// NewModelCache creates a new ModelCache with the given TTL.
func NewModelCache(ttl time.Duration) *ModelCache {
	return &ModelCache{
		cache: make(map[string]*cachedModels),
		ttl:   ttl,
	}
}

// GetOrFetch returns cached model IDs or fetches them using the provided function.
// Concurrent fetches for the same key are deduplicated.
func (c *ModelCache) GetOrFetch(
	key string,
	fetcher func() ([]string, error),
) ([]string, error) {
	// Fast path: check cache
	c.mu.RLock()
	if cached, ok := c.cache[key]; ok && time.Since(cached.fetchedAt) < c.ttl {
		models := cached.modelIDs
		c.mu.RUnlock()
		return models, nil
	}
	c.mu.RUnlock()

	// Slow path: dedup concurrent fetches
	result, err, _ := c.group.Do(key, func() (interface{}, error) {
		// Double-check after acquiring write lock
		c.mu.RLock()
		if cached, ok := c.cache[key]; ok && time.Since(cached.fetchedAt) < c.ttl {
			c.mu.RUnlock()
			return cached.modelIDs, nil
		}
		c.mu.RUnlock()

		// Fetch from upstream
		models, err := fetcher()
		if err != nil {
			return nil, err
		}

		// Cache the result
		c.mu.Lock()
		c.cache[key] = &cachedModels{
			modelIDs:  models,
			fetchedAt: time.Now(),
		}
		c.mu.Unlock()

		return models, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]string), nil
}

// Invalidate removes a key from the cache.
func (c *ModelCache) Invalidate(key string) {
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()
}

// InvalidateAll clears the entire cache.
func (c *ModelCache) InvalidateAll() {
	c.mu.Lock()
	c.cache = make(map[string]*cachedModels)
	c.mu.Unlock()
}
