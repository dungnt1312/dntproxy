# API Key Authorization System Research Report

**Date:** 2026-04-17  
**Context:** dntproxy - OpenAI-compatible proxy routing to multiple AI providers  
**Stack:** Go 1.25+, Gin, SQLite, JSON file storage

## Executive Summary

Research into Go-based API key authorization systems for proxy applications, focusing on hierarchical permissions, rate limiting, usage quotas, audit logging, and in-memory caching strategies.

---

## 1. Recommended Go Packages

### Rate Limiting Libraries

| Package | Algorithm | Pros | Cons | Recommendation |
|---------|-----------|------|------|----------------|
| **golang.org/x/time/rate** | Token Bucket | Standard library, zero deps, mutex-protected, concurrent-safe | Fixed 1-second window, limited flexibility | ✅ **Primary choice** - Already used by dntproxy |
| **github.com/didip/tollbooth** | Token Bucket (wraps x/time/rate) | HTTP middleware, per-IP/header/method limiting, clean API | Extra dependency, opinionated | ✅ **Secondary** - For HTTP-specific features |
| **github.com/uber-go/ratelimit** | Leaky Bucket | Simple API, minimal overhead, blocking behavior | No burst support, less flexible | ⚠️ Consider for worker pools |
| **github.com/sethvargo/go-limiter** | Token Bucket | Pluggable stores (memory/Redis), HTTP middleware | Newer, smaller community | ⚠️ Alternative if Redis needed |

### In-Memory Caching

| Package | Type | Pros | Cons | Recommendation |
|---------|------|------|------|----------------|
| **sync.Map** | Concurrent map | Standard library, lock-free reads | Not optimized for frequent writes, no TTL | ❌ Not suitable (unstable keys) |
| **github.com/patrickmn/go-cache** | TTL cache | Simple API, auto-expiration, thread-safe | Known goroutine leaks | ❌ Avoid |
| **github.com/go-pkgz/expirable-cache** | TTL cache | No goroutine leaks, clean API | Smaller community | ✅ **Recommended** |
| **Custom map + sync.RWMutex** | Manual | Full control, no deps | More code to maintain | ✅ **Best for dntproxy** |

**Rationale for custom cache:** API keys are relatively stable (not frequently added/removed), and we need fine-grained control over permission checks and quota tracking.

---

## 2. Hierarchical Permission Model

### Recommended Structure

```
Resource-Level (most specific)
    ↓
Connection-Level (provider account)
    ↓
Provider-Level (provider type)
    ↓
Global-Level (default)
```

### Implementation Pattern

```go
type Permission struct {
    ResourceID   string   // e.g., "kr/Codex-sonnet-4-5", "oai/*", "*"
    Actions      []string // ["chat", "models", "quota"]
    RateLimit    *RateLimit
    UsageQuota   *UsageQuota
}

type APIKey struct {
    ID           string
    Key          string
    Permissions  []Permission
    GlobalLimit  *RateLimit
    CreatedAt    time.Time
    ExpiresAt    *time.Time
}

// Permission resolution: most specific wins
func (k *APIKey) GetEffectivePermission(resource string) *Permission {
    // 1. Exact match
    // 2. Wildcard match (kr/*, oai/*)
    // 3. Global match (*)
    // 4. Fallback to global limit
}
```

### Hybrid Allow/Deny Pattern

```go
type PermissionRule struct {
    Type     string // "allow" or "deny"
    Pattern  string // "kr/*", "oai/gpt-4", etc.
    Priority int    // Higher priority wins
}

// Evaluation order:
// 1. Explicit deny rules (highest priority)
// 2. Explicit allow rules
// 3. Default deny (if no allow rules match)
```

---

## 3. Rate Limiting Implementation

### Sliding Window with SQLite Persistence

**Algorithm:** Token bucket with sliding window tracking for accurate rate limiting across restarts.

```go
type RateLimiter struct {
    tokens    float64
    maxTokens float64
    refillRate float64 // tokens per second
    lastRefill time.Time
    mu        sync.Mutex
}

// SQLite schema for persistence
CREATE TABLE rate_limit_state (
    key_id TEXT PRIMARY KEY,
    resource TEXT,
    tokens REAL,
    last_refill INTEGER, -- unix nano
    window_start INTEGER,
    request_count INTEGER
);

// Hybrid approach: in-memory for speed, SQLite for durability
type PersistentRateLimiter struct {
    memory map[string]*RateLimiter
    db     *sql.DB
    mu     sync.RWMutex
    
    // Flush to SQLite every 10 seconds or on shutdown
    flushInterval time.Duration
}
```

### Token Bucket Implementation (x/time/rate)

```go
import "golang.org/x/time/rate"

type KeyLimiter struct {
    limiter  *rate.Limiter
    lastSeen time.Time
}

type RateLimitMiddleware struct {
    limiters map[string]*KeyLimiter
    mu       sync.RWMutex
    
    // Cleanup old entries every 5 minutes
    cleanupInterval time.Duration
}

func (m *RateLimitMiddleware) GetLimiter(apiKey string, limit rate.Limit, burst int) *rate.Limiter {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    kl, exists := m.limiters[apiKey]
    if !exists {
        kl = &KeyLimiter{
            limiter:  rate.NewLimiter(limit, burst),
            lastSeen: time.Now(),
        }
        m.limiters[apiKey] = kl
    }
    
    kl.lastSeen = time.Now()
    return kl.limiter
}

// Cleanup goroutine
func (m *RateLimitMiddleware) startCleanup() {
    ticker := time.NewTicker(m.cleanupInterval)
    go func() {
        for range ticker.C {
            m.mu.Lock()
            cutoff := time.Now().Add(-10 * time.Minute)
            for key, kl := range m.limiters {
                if kl.lastSeen.Before(cutoff) {
                    delete(m.limiters, key)
                }
            }
            m.mu.Unlock()
        }
    }()
}
```

### Rate Limit Headers (RFC 6585 + Draft IETF)

```go
func setRateLimitHeaders(c *gin.Context, limit, remaining int, reset time.Time) {
    c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
    c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
    c.Header("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
    c.Header("RateLimit-Limit", strconv.Itoa(limit))
    c.Header("RateLimit-Remaining", strconv.Itoa(remaining))
    c.Header("RateLimit-Reset", strconv.Itoa(int(time.Until(reset).Seconds())))
    
    if remaining == 0 {
        c.Header("Retry-After", strconv.Itoa(int(time.Until(reset).Seconds())))
    }
}
```

---

## 4. Usage Quota Tracking

### SQLite Schema

```sql
CREATE TABLE api_key_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key_id TEXT NOT NULL,
    resource TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    estimated_cost REAL DEFAULT 0.0,
    request_count INTEGER DEFAULT 1,
    INDEX idx_key_timestamp (key_id, timestamp),
    INDEX idx_key_resource (key_id, resource)
);

CREATE TABLE api_key_quotas (
    key_id TEXT PRIMARY KEY,
    max_requests INTEGER,
    max_tokens INTEGER,
    max_cost REAL,
    period_seconds INTEGER, -- rolling window size
    current_requests INTEGER DEFAULT 0,
    current_tokens INTEGER DEFAULT 0,
    current_cost REAL DEFAULT 0.0,
    window_start INTEGER,
    FOREIGN KEY (key_id) REFERENCES api_keys(id)
);
```

### In-Memory Quota Tracker

```go
type QuotaTracker struct {
    keyQuotas map[string]*KeyQuota
    mu        sync.RWMutex
    db        *sql.DB
}

type KeyQuota struct {
    MaxRequests int
    MaxTokens   int64
    MaxCost     float64
    Period      time.Duration
    
    CurrentRequests int
    CurrentTokens   int64
    CurrentCost     float64
    WindowStart     time.Time
    
    mu sync.Mutex
}

func (q *QuotaTracker) CheckAndIncrement(keyID string, tokens int64, cost float64) error {
    q.mu.RLock()
    kq, exists := q.keyQuotas[keyID]
    q.mu.RUnlock()
    
    if !exists {
        kq = q.loadFromDB(keyID)
        q.mu.Lock()
        q.keyQuotas[keyID] = kq
        q.mu.Unlock()
    }
    
    kq.mu.Lock()
    defer kq.mu.Unlock()
    
    // Reset window if expired
    if time.Since(kq.WindowStart) > kq.Period {
        kq.CurrentRequests = 0
        kq.CurrentTokens = 0
        kq.CurrentCost = 0.0
        kq.WindowStart = time.Now()
    }
    
    // Check limits
    if kq.MaxRequests > 0 && kq.CurrentRequests >= kq.MaxRequests {
        return ErrQuotaExceeded
    }
    if kq.MaxTokens > 0 && kq.CurrentTokens+tokens > kq.MaxTokens {
        return ErrQuotaExceeded
    }
    if kq.MaxCost > 0 && kq.CurrentCost+cost > kq.MaxCost {
        return ErrQuotaExceeded
    }
    
    // Increment
    kq.CurrentRequests++
    kq.CurrentTokens += tokens
    kq.CurrentCost += cost
    
    return nil
}

// Async flush to SQLite every 30 seconds
func (q *QuotaTracker) startPeriodicFlush() {
    ticker := time.NewTicker(30 * time.Second)
    go func() {
        for range ticker.C {
            q.flushToDB()
        }
    }()
}
```

---

## 5. Audit Logging

### Buffered Channel Pattern

```go
type AuditEvent struct {
    Timestamp   time.Time
    KeyID       string
    Action      string // "request", "quota_exceeded", "rate_limited", "denied"
    Resource    string
    IPAddress   string
    StatusCode  int
    Tokens      int64
    Cost        float64
    Error       string
}

type AuditLogger struct {
    events chan AuditEvent
    db     *sql.DB
    buffer []AuditEvent
    mu     sync.Mutex
}

func NewAuditLogger(db *sql.DB, bufferSize int) *AuditLogger {
    al := &AuditLogger{
        events: make(chan AuditEvent, bufferSize),
        db:     db,
        buffer: make([]AuditEvent, 0, 100),
    }
    go al.processEvents()
    return al
}

func (al *AuditLogger) processEvents() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case event := <-al.events:
            al.mu.Lock()
            al.buffer = append(al.buffer, event)
            if len(al.buffer) >= 100 {
                al.flushBuffer()
            }
            al.mu.Unlock()
            
        case <-ticker.C:
            al.mu.Lock()
            if len(al.buffer) > 0 {
                al.flushBuffer()
            }
            al.mu.Unlock()
        }
    }
}

func (al *AuditLogger) flushBuffer() {
    if len(al.buffer) == 0 {
        return
    }
    
    tx, _ := al.db.Begin()
    stmt, _ := tx.Prepare(`
        INSERT INTO audit_log (timestamp, key_id, action, resource, ip_address, status_code, tokens, cost, error)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `)
    
    for _, event := range al.buffer {
        stmt.Exec(event.Timestamp.UnixNano(), event.KeyID, event.Action, 
                  event.Resource, event.IPAddress, event.StatusCode, 
                  event.Tokens, event.Cost, event.Error)
    }
    
    stmt.Close()
    tx.Commit()
    al.buffer = al.buffer[:0]
}

// Non-blocking log
func (al *AuditLogger) Log(event AuditEvent) {
    select {
    case al.events <- event:
    default:
        // Channel full, drop event or log warning
    }
}
```

---

## 6. API Key Rotation Best Practices

### Rotation Strategy

1. **Dual-key period:** Allow both old and new keys to work simultaneously for 24-48 hours
2. **Automatic expiration:** Set `expires_at` on old key when new key is generated
3. **Notification:** Log rotation events and optionally notify via webhook
4. **Grace period:** Reject old key with `410 Gone` after expiration, include new key hint in response

```go
type KeyRotation struct {
    OldKeyID  string
    NewKeyID  string
    CreatedAt time.Time
    ExpiresAt time.Time
}

func (s *KeyService) RotateKey(oldKeyID string) (*APIKey, error) {
    // Generate new key
    newKey := s.GenerateKey()
    
    // Set expiration on old key (48 hours from now)
    s.SetKeyExpiration(oldKeyID, time.Now().Add(48*time.Hour))
    
    // Copy permissions from old to new
    s.CopyPermissions(oldKeyID, newKey.ID)
    
    // Log rotation
    s.auditLogger.Log(AuditEvent{
        Action: "key_rotated",
        KeyID:  oldKeyID,
        Metadata: map[string]string{"new_key_id": newKey.ID},
    })
    
    return newKey, nil
}
```

---

## 7. Security Best Practices

### Key Storage

- **Hash keys in database:** Use bcrypt or argon2 for stored keys
- **Prefix keys:** Use `dnt_` prefix for easy identification
- **Entropy:** Generate keys with crypto/rand (32+ bytes)

```go
import "crypto/rand"

func GenerateAPIKey() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return "dnt_" + base64.URLEncoding.EncodeToString(b), nil
}
```

### Middleware Pattern

```go
func APIKeyAuthMiddleware(keyStore *KeyStore, rateLimiter *RateLimiter, quotaTracker *QuotaTracker, auditLogger *AuditLogger) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract key from header
        apiKey := c.GetHeader("Authorization")
        if apiKey == "" {
            apiKey = c.Query("api_key")
        }
        
        if apiKey == "" {
            auditLogger.Log(AuditEvent{Action: "missing_key", IPAddress: c.ClientIP()})
            c.JSON(401, gin.H{"error": "API key required"})
            c.Abort()
            return
        }
        
        // Validate key
        key, err := keyStore.GetKey(apiKey)
        if err != nil {
            auditLogger.Log(AuditEvent{Action: "invalid_key", IPAddress: c.ClientIP()})
            c.JSON(401, gin.H{"error": "Invalid API key"})
            c.Abort()
            return
        }
        
        // Check expiration
        if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
            auditLogger.Log(AuditEvent{Action: "expired_key", KeyID: key.ID})
            c.JSON(401, gin.H{"error": "API key expired"})
            c.Abort()
            return
        }
        
        // Rate limiting
        limiter := rateLimiter.GetLimiter(key.ID, key.RateLimit)
        if !limiter.Allow() {
            auditLogger.Log(AuditEvent{Action: "rate_limited", KeyID: key.ID})
            c.JSON(429, gin.H{"error": "Rate limit exceeded"})
            c.Abort()
            return
        }
        
        // Store key in context for downstream handlers
        c.Set("api_key", key)
        c.Next()
        
        // Post-request: track usage
        if c.Writer.Status() == 200 {
            // Extract token usage from response
            tokens := c.GetInt64("tokens_used")
            cost := c.GetFloat64("estimated_cost")
            
            quotaTracker.Increment(key.ID, tokens, cost)
            auditLogger.Log(AuditEvent{
                Action:     "request",
                KeyID:      key.ID,
                Resource:   c.Request.URL.Path,
                StatusCode: c.Writer.Status(),
                Tokens:     tokens,
                Cost:       cost,
            })
        }
    }
}
```

---

## 8. Performance Considerations

### Benchmarks (Expected)

- **In-memory key lookup:** < 100ns
- **Rate limit check (x/time/rate):** ~150ns
- **Quota check (in-memory):** < 200ns
- **Audit log (async):** < 10ns (channel send)
- **Total middleware overhead:** < 500ns per request

### Optimization Strategies

1. **Read-heavy optimization:** Use `sync.RWMutex` for key cache
2. **Batch SQLite writes:** Flush every 30s or 100 events
3. **Lazy loading:** Load key permissions on first use
4. **TTL-based cleanup:** Remove inactive keys from memory after 10 minutes
5. **Connection pooling:** Set `db.SetMaxOpenConns(25)` for SQLite

---

## 9. Recommended Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Gin HTTP Handler                        │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│              APIKeyAuthMiddleware                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │  Key Store   │  │ Rate Limiter │  │Quota Tracker │     │
│  │ (in-memory)  │  │(x/time/rate) │  │ (in-memory)  │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                   Audit Logger (async)                      │
│                  Buffered Channel → SQLite                  │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                    Existing Chat Service                    │
└─────────────────────────────────────────────────────────────┘
```

---

## 10. Implementation Roadmap

### Phase 1: Core Authorization (Week 1)
- [ ] API key generation and storage (JSON + SQLite)
- [ ] Basic authentication middleware
- [ ] Key CRUD endpoints
- [ ] Key hashing and validation

### Phase 2: Rate Limiting (Week 2)
- [ ] Integrate x/time/rate for per-key limiting
- [ ] In-memory limiter cache with cleanup
- [ ] Rate limit headers
- [ ] SQLite persistence for rate limit state

### Phase 3: Permissions & Quotas (Week 3)
- [ ] Hierarchical permission model
- [ ] Resource-level access control
- [ ] Usage quota tracking (SQLite)
- [ ] Quota check middleware

### Phase 4: Audit & Monitoring (Week 4)
- [ ] Async audit logging with buffered channels
- [ ] SQLite audit log schema
- [ ] Usage analytics endpoints
- [ ] Key rotation API

### Phase 5: Polish & Testing (Week 5)
- [ ] Comprehensive unit tests
- [ ] Load testing (10k req/s target)
- [ ] Documentation
- [ ] Migration scripts

---

## 11. Code Examples

### Complete Middleware Integration

```go
// internal/adapter/http/middleware/api-key-auth.go
package middleware

import (
    "github.com/gin-gonic/gin"
    "golang.org/x/time/rate"
)

func APIKeyAuth(
    keyStore port.CredentialStore,
    rateLimiter *RateLimiter,
    quotaTracker *QuotaTracker,
    auditLogger *AuditLogger,
) gin.HandlerFunc {
    return func(c *gin.Context) {
        apiKey := extractAPIKey(c)
        if apiKey == "" {
            c.JSON(401, gin.H{"error": "API key required"})
            c.Abort()
            return
        }
        
        key, err := keyStore.GetAPIKey(c.Request.Context(), apiKey)
        if err != nil {
            c.JSON(401, gin.H{"error": "Invalid API key"})
            c.Abort()
            return
        }
        
        // Rate limiting
        if !rateLimiter.Allow(key.ID) {
            c.Header("Retry-After", "60")
            c.JSON(429, gin.H{"error": "Rate limit exceeded"})
            c.Abort()
            return
        }
        
        // Quota check
        if err := quotaTracker.CheckQuota(key.ID); err != nil {
            c.JSON(429, gin.H{"error": "Quota exceeded"})
            c.Abort()
            return
        }
        
        c.Set("api_key", key)
        c.Next()
        
        // Post-request tracking
        go func() {
            tokens := c.GetInt64("tokens_used")
            cost := c.GetFloat64("cost")
            quotaTracker.IncrementUsage(key.ID, tokens, cost)
            auditLogger.Log(AuditEvent{
                KeyID:      key.ID,
                Resource:   c.Request.URL.Path,
                StatusCode: c.Writer.Status(),
                Tokens:     tokens,
                Cost:       cost,
            })
        }()
    }
}
```

---

## 12. Unresolved Questions

1. **Redis vs SQLite for rate limit state?**
   - SQLite sufficient for single-instance deployment
   - Redis needed for multi-instance horizontal scaling
   - Recommendation: Start with SQLite, add Redis adapter later

2. **API key prefix strategy?**
   - `dnt_live_xxx` for production keys
   - `dnt_test_xxx` for test keys
   - Allows easy identification and separate rate limits

3. **Quota reset strategy?**
   - Rolling window (recommended): More accurate, complex
   - Fixed window: Simpler, potential burst at window boundary
   - Recommendation: Rolling window with SQLite tracking

4. **Permission inheritance?**
   - Should connection-level permissions override provider-level?
   - Recommendation: Most specific wins (resource > connection > provider > global)

5. **Audit log retention?**
   - 30 days (match existing log retention)
   - Configurable via settings
   - Auto-cleanup via SQLite trigger or background job

---

## References

- [Tollbooth GitHub](https://github.com/didip/tollbooth) - Token bucket implementation
- [uber-go/ratelimit](https://github.com/uber-go/ratelimit) - Leaky bucket implementation
- [sethvargo/go-limiter](https://github.com/sethvargo/go-limiter) - Pluggable rate limiter
- [Alex Edwards: Rate Limiting in Go](https://www.alexedwards.net/blog/how-to-rate-limit-http-requests)
- [LogRocket: Rate Limiting Go Apps](https://blog.logrocket.com/rate-limiting-go-application/)
- [Redis Rate Limiting Patterns](https://redis.io/glossary/rate-limiting/)
- [NGINX Rate Limiting Guide](https://www.nginx.com/blog/rate-limiting-nginx/)
- [RFC 6585: Additional HTTP Status Codes](https://tools.ietf.org/html/rfc6585)
- [IETF Draft: RateLimit Headers](https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers)
