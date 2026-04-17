---
status: pending
created: 2026-04-17
priority: high
estimated_duration: 2-3 weeks
---

# API Key Authorization System - Implementation Plan

## Overview

Implement a comprehensive API key authorization system with role-based access control, hierarchical permissions, rate limiting, usage quotas, and audit logging for dntproxy.

**Goal:** Enable admin/user role separation, granular model/connection/provider permissions, rate limiting, quota enforcement, and complete audit trail.

---

## Phase 1: Domain & Port Layer (Foundation)

**Duration:** 2-3 days
**Goal:** Define core types and interfaces without breaking existing code

### Tasks

#### 1.1 Extend Domain Types
**File:** `internal/domain/config.go`

**Changes:** Add new fields to `APIKey` struct and add new types:
```go
type APIKey struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Key         string            `json:"key"`
    IsActive    bool              `json:"isActive"`
    CreatedAt   string            `json:"createdAt,omitempty"`
    Role        string            `json:"role"`                  // "admin" | "user"
    Permissions APIKeyPermissions `json:"permissions,omitempty"`
    RateLimits  RateLimits        `json:"rateLimits,omitempty"`
    Quotas      UsageQuotas       `json:"quotas,omitempty"`
    ExpiresAt   *string           `json:"expiresAt,omitempty"`
}

type APIKeyPermissions struct {
    AllowModels      []string `json:"allowModels,omitempty"`
    DenyModels       []string `json:"denyModels,omitempty"`
    AllowConnections []string `json:"allowConnections,omitempty"`
    DenyConnections  []string `json:"denyConnections,omitempty"`
    AllowProviders   []string `json:"allowProviders,omitempty"`
    DenyProviders    []string `json:"denyProviders,omitempty"`
}

type RateLimits struct {
    RequestsPerMinute int     `json:"requestsPerMinute,omitempty"`
    RequestsPerHour   int     `json:"requestsPerHour,omitempty"`
    RequestsPerDay    int     `json:"requestsPerDay,omitempty"`
    TokensPerDay      int     `json:"tokensPerDay,omitempty"`
    CostPerDay        float64 `json:"costPerDay,omitempty"`
    CostPerMonth      float64 `json:"costPerMonth,omitempty"`
}

// UsageQuotas defines rolling limits per key.
// WindowType: "daily" | "monthly" | "lifetime" (default: "monthly")
type UsageQuotas struct {
    MaxTokens  int     `json:"maxTokens,omitempty"`
    MaxCost    float64 `json:"maxCost,omitempty"`
    WindowType string  `json:"windowType,omitempty"`
}
```

**Size:** ~60 lines added

---

#### 1.2 Create Auth Event Domain Type
**File:** `internal/domain/auth-event.go` (new)

```go
package domain

type AuthEvent struct {
    ID          string            `json:"id"`
    Timestamp   string            `json:"timestamp"`
    APIKeyID    string            `json:"apiKeyId"`
    APIKeyName  string            `json:"apiKeyName"`
    Action      string            `json:"action"`      // "chat", "list_models", etc.
    Resource    string            `json:"resource"`    // model/connection/provider
    Decision    string            `json:"decision"`    // "allow" | "deny"
    Reason      string            `json:"reason"`
    RequestPath string            `json:"requestPath"`
    IPAddress   string            `json:"ipAddress,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
}
```

**Size:** ~20 lines

---

#### 1.3 Create Port Interfaces

**File:** `internal/port/authorizer.go` (new)

```go
package port

import (
    "context"
    "github.com/dungnt/dntproxy/internal/domain"
)

type Authorizer interface {
    Authorize(ctx context.Context, apiKey *domain.APIKey, req AuthRequest) (*AuthDecision, error)
}

type AuthRequest struct {
    Action       string
    Model        string
    ConnectionID string
    Provider     string
    RequestPath  string
    IPAddress    string
}

type AuthDecision struct {
    Allowed bool
    Reason  string
}
```

**Size:** ~30 lines

---

**File:** `internal/port/rate-limiter.go` (new)

```go
package port

import (
    "context"
    "time"
    "github.com/dungnt/dntproxy/internal/domain"
)

type RateLimiter interface {
    // Allow checks and consumes one request slot.
    // Receives full RateLimits config so it knows the configured limit value.
    Allow(ctx context.Context, keyID string, limits domain.RateLimits, window RateLimitWindow) (bool, error)
    GetRemaining(ctx context.Context, keyID string, window RateLimitWindow) (int, error)
    GetResetTime(ctx context.Context, keyID string, window RateLimitWindow) (time.Time, error)
}

type RateLimitWindow string

const (
    WindowMinute RateLimitWindow = "minute"
    WindowHour   RateLimitWindow = "hour"
    WindowDay    RateLimitWindow = "day"
)
```

**Size:** ~30 lines

---

**File:** `internal/port/quota-tracker.go` (new)

```go
package port

import (
    "context"
    "github.com/dungnt/dntproxy/internal/domain"
)

type QuotaTracker interface {
    // CheckQuota returns error if usage would exceed limits.
    // Receives full APIKey so it can read Quotas config including WindowType.
    CheckQuota(ctx context.Context, apiKey *domain.APIKey, tokens int, cost float64) error
    IncrementUsage(ctx context.Context, keyID string, tokens int, cost float64) error
    GetUsage(ctx context.Context, keyID string) (*QuotaUsage, error)
    ResetUsage(ctx context.Context, keyID string) error
}

type QuotaUsage struct {
    CurrentTokens int
    CurrentCost   float64
    MaxTokens     int
    MaxCost       float64
    WindowStart   string
    WindowType    string
}
```

**Size:** ~30 lines

---

**File:** `internal/port/auth-event-store.go` (new)

```go
package port

import (
    "context"
    "time"
    "github.com/dungnt/dntproxy/internal/domain"
)

type AuthEventStore interface {
    LogAuthEvent(ctx context.Context, event *domain.AuthEvent) error
    GetAuthEvents(ctx context.Context, filter AuthEventFilter) ([]domain.AuthEvent, error)
    GetAuthEventsSummary(ctx context.Context, apiKeyID string) (*AuthEventSummary, error)
    ClearAuthEvents(ctx context.Context, before time.Time) error
}

type AuthEventFilter struct {
    APIKeyID  string
    Decision  string     // "allow" | "deny" | ""
    Action    string
    StartTime *time.Time
    EndTime   *time.Time
    Limit     int
    Offset    int
}

type AuthEventSummary struct {
    TotalEvents  int
    AllowedCount int
    DeniedCount  int
    TopActions   []ActionCount
    TopResources []ResourceCount
}

type ActionCount   struct { Action string; Count int }
type ResourceCount struct { Resource string; Count int }
```

**Size:** ~45 lines

---

#### 1.4 Update CredentialStore Interface
**File:** `internal/port/credential-store.go`

**Add 3 methods:**
```go
// GetAPIKeyByID returns a key by its ID.
GetAPIKeyByID(id string) (*domain.APIKey, error)
// GetAPIKeyByValue returns a key by its secret string (used by middleware).
GetAPIKeyByValue(key string) (*domain.APIKey, error)
// UpdateAPIKey persists changes to an existing key.
UpdateAPIKey(key *domain.APIKey) error
```

> Keep existing `ValidateAPIKey(key string) bool` — used by existing middleware, can delegate to `GetAPIKeyByValue` internally.

---

### Phase 1 Success Criteria
- [ ] All new domain types compile and serialize correctly
- [ ] Port interfaces defined with clear contracts
- [ ] No breaking changes to existing code
- [ ] Unit tests for domain type serialization

---

## Phase 2: Storage & Persistence

**Duration:** 3-4 days
**Goal:** Implement SQLite schema and storage adapters

### Tasks

#### 2.1 Create Auth Events SQLite Schema
**File:** `internal/adapter/storage/sqlite-auth-event-store-schema.go` (new)

```go
const authEventSchema = `
CREATE TABLE IF NOT EXISTS auth_events (
    id           TEXT PRIMARY KEY,
    timestamp    TEXT NOT NULL,
    api_key_id   TEXT NOT NULL,
    api_key_name TEXT NOT NULL,
    action       TEXT NOT NULL,
    resource     TEXT NOT NULL,
    decision     TEXT NOT NULL,
    reason       TEXT,
    request_path TEXT NOT NULL,
    ip_address   TEXT,
    metadata     TEXT
);

-- SQLite does not support inline INDEX in CREATE TABLE — use separate statements
CREATE INDEX IF NOT EXISTS idx_auth_events_timestamp  ON auth_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_auth_events_api_key_id ON auth_events(api_key_id);
CREATE INDEX IF NOT EXISTS idx_auth_events_decision   ON auth_events(decision);
CREATE INDEX IF NOT EXISTS idx_auth_events_action     ON auth_events(action);

-- rate_limit_counters: (key_id, window, window_start) PK allows multiple time windows
-- old rows cleaned up by background worker
CREATE TABLE IF NOT EXISTS rate_limit_counters (
    key_id        TEXT    NOT NULL,
    window        TEXT    NOT NULL,
    window_start  INTEGER NOT NULL,
    request_count INTEGER DEFAULT 0,
    PRIMARY KEY (key_id, window, window_start)
);

CREATE TABLE IF NOT EXISTS quota_usage (
    key_id         TEXT    PRIMARY KEY,
    current_tokens INTEGER DEFAULT 0,
    current_cost   REAL    DEFAULT 0.0,
    window_start   INTEGER NOT NULL,
    window_type    TEXT    NOT NULL DEFAULT 'monthly'
);
`
```

**Size:** ~50 lines

---

#### 2.2 Implement Auth Event Store
**File:** `internal/adapter/storage/sqlite-auth-event-store.go` (new)

**Implementation:**
- Async buffered channel, non-blocking writes (buffer: 1000)
- Batch insert every 5s or 100 events (whichever first)
- 30-day auto-retention (matches existing log retention)
- Query with filtering and pagination

**Key methods:**
```go
func NewSQLiteAuthEventStore(db *sql.DB) *SQLiteAuthEventStore
func (s *SQLiteAuthEventStore) LogAuthEvent(ctx, event) error        // non-blocking channel send
func (s *SQLiteAuthEventStore) GetAuthEvents(ctx, filter) ([]domain.AuthEvent, error)
func (s *SQLiteAuthEventStore) GetAuthEventsSummary(ctx, apiKeyID) (*port.AuthEventSummary, error)
func (s *SQLiteAuthEventStore) ClearAuthEvents(ctx, before) error
func (s *SQLiteAuthEventStore) flushBuffer()                         // internal batch insert
func (s *SQLiteAuthEventStore) startBackgroundWorker()               // goroutine
```

**Size:** ~180 lines

---

#### 2.3 Update JSON DB Storage
**File:** `internal/adapter/storage/json-db.go`

**Changes:**
- Implement `GetAPIKeyByID(id string) (*domain.APIKey, error)`
- Implement `GetAPIKeyByValue(key string) (*domain.APIKey, error)`
- Implement `UpdateAPIKey(key *domain.APIKey) error`
- Call `migrateAPIKeys(cfg)` inside `readFromDisk()` after unmarshal

**Migration logic:**
```go
func migrateAPIKeys(cfg *domain.AppConfig) {
    for i := range cfg.APIKeys {
        if cfg.APIKeys[i].Role == "" {
            cfg.APIKeys[i].Role = "admin" // existing keys → admin (full access)
        }
    }
}
```

**Size:** +60 lines

---

### Phase 2 Success Criteria
- [ ] SQLite schema created with correct index syntax (separate CREATE INDEX)
- [ ] Auth events logged asynchronously without blocking requests
- [ ] Batch insert works correctly
- [ ] Query filtering returns correct results
- [ ] Existing API keys auto-migrate to admin role on first `readFromDisk()`
- [ ] `GetAPIKeyByValue` returns full key object including permissions

---

## Phase 3: Authorization Core

**Duration:** 4-5 days
**Goal:** Implement authorization logic, rate limiting, quota tracking

### Tasks

#### 3.1 Add Dependency: golang.org/x/time
**Action:** `go get golang.org/x/time`

> `golang.org/x/time` is not in `go.mod` yet. Required for `golang.org/x/time/rate` token bucket used by rate limiter. Must be done before implementing Phase 3.2.

---

#### 3.2 Implement Authorizer
**File:** `internal/adapter/auth/authorizer.go` (new)

**Permission evaluation logic:**
1. `role == "admin"` → allow immediately (skip all checks)
2. Check deny lists first — deny wins over allow
3. Check allow lists hierarchically: model → connection → provider
4. No matching allow rule → deny

```go
func NewAuthorizer() port.Authorizer
func (a *authorizer) Authorize(ctx, apiKey, req) (*port.AuthDecision, error)
func (a *authorizer) evaluatePermissions(apiKey, req) (allowed bool, reason string)
func checkModelPermission(perms domain.APIKeyPermissions, model string) (allowed, denied bool)
func checkConnectionPermission(perms domain.APIKeyPermissions, connID string) (allowed, denied bool)
func checkProviderPermission(perms domain.APIKeyPermissions, provider string) (allowed, denied bool)
```

**Size:** ~120 lines

**Testing:**
- Admin bypass
- Deny overrides allow
- Hierarchical evaluation order
- Empty permissions on user key → deny all

---

#### 3.3 Implement Rate Limiter
**File:** `internal/adapter/auth/rate-limiter.go` (new)

**Implementation:**
- In-memory token bucket per `(keyID, window)` using `golang.org/x/time/rate`
- `Allow()` receives `domain.RateLimits` to know the configured limit
- Periodic cleanup of stale entries (10min TTL after last access)
- Thread-safe with `sync.RWMutex`

```go
func NewRateLimiter() port.RateLimiter
func (rl *rateLimiter) Allow(ctx, keyID, limits, window) (bool, error)
func (rl *rateLimiter) GetRemaining(ctx, keyID, window) (int, error)
func (rl *rateLimiter) GetResetTime(ctx, keyID, window) (time.Time, error)
func (rl *rateLimiter) getLimiter(keyID, window string, limit int) *rate.Limiter
func (rl *rateLimiter) startCleanup()
```

**Size:** ~130 lines

---

#### 3.4 Implement Quota Tracker
**File:** `internal/adapter/auth/quota-tracker.go` (new)

**Implementation:**
- In-memory state per keyID, periodic flush to SQLite (30s)
- `CheckQuota()` receives full `*domain.APIKey` to read `Quotas` config
- Window reset based on `Quotas.WindowType`: "daily" | "monthly" | "lifetime"
- Thread-safe with `sync.RWMutex`

```go
func NewQuotaTracker(db *sql.DB) port.QuotaTracker
func (qt *quotaTracker) CheckQuota(ctx, apiKey, tokens, cost) error
func (qt *quotaTracker) IncrementUsage(ctx, keyID, tokens, cost) error
func (qt *quotaTracker) GetUsage(ctx, keyID) (*port.QuotaUsage, error)
func (qt *quotaTracker) ResetUsage(ctx, keyID) error
func (qt *quotaTracker) flushToDB()
func (qt *quotaTracker) shouldReset(state *quotaState, windowType string) bool
```

**Size:** ~140 lines

---

### Phase 3 Success Criteria
- [ ] `golang.org/x/time` added to `go.mod`
- [ ] Authorizer correctly evaluates all permission combinations
- [ ] Rate limiter enforces per-minute/hour/day limits
- [ ] Quota tracker enforces token/cost limits with correct window reset
- [ ] All components thread-safe
- [ ] Unit test coverage >80%

---

## Phase 4: Middleware & Integration

**Duration:** 3-4 days
**Goal:** Integrate authorization into HTTP layer and services

### Tasks

#### 4.1 Rename Conflicting Function
**File:** `internal/adapter/http/router.go`

**Change:** Rename `extractAPIKey(r *http.Request) string` → `extractAPIKeyFromRequest(r *http.Request) string`

> `router.go:180` defines `extractAPIKey`. The new `auth-middleware.go` needs its own `extractAPIKey(c *gin.Context)`. Both are in package `http` — compile error without this rename. Do this first.

---

#### 4.2 Create Auth Middleware
**File:** `internal/adapter/http/auth-middleware.go` (new)

```go
const CtxAPIKey = "apiKey" // gin.Context key for *domain.APIKey

// adminKeyMiddleware: validates key exists + role=="admin", logs auth event.
// When requireApiKey==false AND no keys configured → pass through.
// When requireApiKey==false BUT keys exist → still require admin key for /api/*.
func adminKeyMiddleware(store port.CredentialStore, eventStore port.AuthEventStore) gin.HandlerFunc

// userKeyMiddleware: validates key, runs authorizer + rate limiter + quota, sets headers.
func userKeyMiddleware(
    store        port.CredentialStore,
    authorizer   port.Authorizer,
    rateLimiter  port.RateLimiter,
    quotaTracker port.QuotaTracker,
    eventStore   port.AuthEventStore,
) gin.HandlerFunc

func extractAPIKey(c *gin.Context) string
func setRateLimitHeaders(c *gin.Context, limit, remaining int, reset time.Time)
```

**Size:** ~160 lines

---

#### 4.3 Update Router
**File:** `internal/adapter/http/router.go`

**Changes:**
- Update `NewRouter` signature to accept new dependencies
- Apply `adminKeyMiddleware` to `/api/*` group
- Replace `apiKeyMiddleware` on `/v1/*` with `userKeyMiddleware`

```go
func NewRouter(
    store        port.CredentialStore,
    providers    port.ProviderRegistry,
    tunnelMgr    port.TunnelManager,
    authorizer   port.Authorizer,
    rateLimiter  port.RateLimiter,
    quotaTracker port.QuotaTracker,
    eventStore   port.AuthEventStore,
) *gin.Engine
```

Update `main.go` to construct and pass new dependencies.

**Size:** +30 lines

---

#### 4.4 Update Chat Service
**File:** `internal/service/chat-service.go`

**Strategy:** Pass `*domain.APIKey` via `context.Context` — avoids changing `port.ChatService` interface signature and breaking callers.

**Changes:**
- Update `port.ChatService` interface: add `context.Context` as first param to `HandleChat`
- Read `apiKey` from context inside `HandleChat`
- Pass `apiKey` permissions down to `executeOnProvider` → `accountSelector`

```go
// port.ChatService updated signature:
HandleChat(ctx context.Context, body []byte, modelStr string, requestID string) *ChatResult

// In chat-handler.go, pass gin context:
result := chatService.HandleChat(c.Request.Context(), body, modelStr, requestID)
```

**Size:** +25 lines

---

#### 4.5 Update Account Selector
**File:** `internal/service/account-selector.go`

**Add pure helper function** (no receiver, easy to test):
```go
// FilterConnectionsByPermissions filters connections by API key allow/deny lists.
// Called in executeOnProvider before passing allowedConnectionIDs to SelectCredentials.
func FilterConnectionsByPermissions(
    conns []domain.ProviderConnection,
    perms domain.APIKeyPermissions,
) []domain.ProviderConnection
```

**Size:** +40 lines

---

### Phase 4 Success Criteria
- [ ] No compile errors — `extractAPIKey` rename done before adding middleware
- [ ] Admin keys access `/api/*`; user keys blocked
- [ ] User keys filtered by permissions in chat requests
- [ ] Rate limit headers returned: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`
- [ ] Auth events logged for all requests
- [ ] `NewRouter` and `main.go` updated correctly

---

## Phase 5: API Endpoints

**Duration:** 3-4 days
**Goal:** Build management API for keys and auth events

### Tasks

#### 5.1 Update Key Handler
**File:** `internal/adapter/http/key-handler.go`

**New/updated endpoints:**
- `POST /api/keys` — create key with role + permissions + limits
- `GET /api/keys/:id` — get key details
- `PUT /api/keys/:id` — update name/permissions/limits
- `POST /api/keys/:id/rotate` — new secret, same ID/permissions
- `GET /api/keys/:id/usage` — quota usage from QuotaTracker

```go
type CreateKeyRequest struct {
    Name        string                   `json:"name"`
    Role        string                   `json:"role"`        // "admin" | "user"
    Permissions domain.APIKeyPermissions `json:"permissions"`
    RateLimits  domain.RateLimits        `json:"rateLimits"`
    Quotas      domain.UsageQuotas       `json:"quotas"`
    ExpiresIn   string                   `json:"expiresIn"`   // "30d", "1y", ""
}

type UpdateKeyRequest struct {
    Name        string                   `json:"name"`
    Permissions domain.APIKeyPermissions `json:"permissions"`
    RateLimits  domain.RateLimits        `json:"rateLimits"`
    Quotas      domain.UsageQuotas       `json:"quotas"`
}
```

**Size:** +130 lines

---

#### 5.2 Create Auth Event Handler
**File:** `internal/adapter/http/auth-event-handler.go` (new)

**Endpoints:**
- `GET /api/auth-events` — list with filters: `apiKeyID`, `decision`, `action`, `start`, `end`, `limit`, `offset`
- `GET /api/auth-events/summary` — aggregate stats
- `DELETE /api/auth-events` — clear events older than `?before=` (RFC3339)

**Size:** ~100 lines

---

#### 5.3 Update API Routes
**File:** `internal/adapter/http/api-handler.go`

Register new key management and auth event routes.

**Size:** +20 lines

---

### Phase 5 Success Criteria
- [ ] Create/update/rotate keys via API
- [ ] View per-key usage stats
- [ ] View/filter/clear auth events

---

## Phase 6: CLI Commands

**Duration:** 2-3 days
**Goal:** CLI parity with API for key management

### Tasks

#### 6.1 Update Key Commands

**New subcommands:**
```
dntproxy key create --name <n> --role user --allow-models "kr/model1,kr/model2"
dntproxy key list
dntproxy key get <id>
dntproxy key update <id> --allow-models "..."
dntproxy key rotate <id>
dntproxy key remove <id>
dntproxy key usage <id>
```

**Size:** ~150 lines

---

#### 6.2 Auth Event Commands
**File:** `cmd/dntproxy/auth-events-cmd.go` (new)

```
dntproxy auth-events list [--key <id>] [--decision deny] [--limit 50]
dntproxy auth-events summary [--key <id>]
dntproxy auth-events clear --before 2026-01-01
```

**Size:** ~80 lines

---

### Phase 6 Success Criteria
- [ ] All key management operations available via CLI
- [ ] Auth event listing/clearing via CLI

---

## Phase 7: Testing & Documentation

**Duration:** 3-4 days
**Goal:** Comprehensive testing and documentation

### Tasks

#### 7.1 Unit Tests
- `internal/adapter/auth/authorizer_test.go`
- `internal/adapter/auth/rate-limiter_test.go`
- `internal/adapter/auth/quota-tracker_test.go`
- `internal/adapter/storage/sqlite-auth-event-store_test.go`
- `internal/service/account-selector_test.go` — `FilterConnectionsByPermissions`

**Coverage target:** >80%

---

#### 7.2 Integration Tests
**File:** `internal/adapter/http/auth_integration_test.go` (new)

**Scenarios:**
- Admin key accesses all endpoints
- User key blocked from `/api/*`
- User key filtered by model permissions in chat
- Rate limiting enforced and headers returned
- Quota enforcement blocks over-limit requests
- Auth events logged for allow and deny

**Size:** ~200 lines

---

#### 7.3 Update Documentation
- `docs/system-architecture.md` — add auth flow
- `docs/codebase-summary.md` — update with new packages
- `README.md` — add authorization section

---

### Phase 7 Success Criteria
- [ ] Unit test coverage >80%
- [ ] Integration tests pass
- [ ] Docs reflect actual implementation

---

## Risk Mitigation

### Risk 1: Complex Permission Logic Causes Bugs
**Impact:** High
**Mitigation:** Comprehensive unit tests; deny-wins logic is explicit and simple; log all auth decisions.

### Risk 2: Rate Limiting False Positives
**Impact:** Medium
**Mitigation:** Token bucket algorithm (proven); configurable limits per key; admin keys exempt; clear rate limit headers.

### Risk 3: Auth Event Logging Fills Disk
**Impact:** Medium
**Mitigation:** 30-day auto-retention; async logging never blocks requests.

### Risk 4: Performance Degradation
**Impact:** Low
**Mitigation:** In-memory caching; async event logging; target <5ms overhead per request.

### Risk 5: Migration Breaks Existing Keys
**Impact:** High
**Mitigation:** `migrateAPIKeys()` called in `readFromDisk()` — existing keys get `role=admin` automatically; empty permissions = full access for admin; backup `db.json` before deploy.

### Risk 6: `extractAPIKey` Name Conflict (compile error)
**Impact:** High
**Mitigation:** Rename existing function in `router.go` first (Phase 4.1) before creating `auth-middleware.go`.

---

## Success Metrics

1. **Functional:** Admin can create user keys with specific model/connection/provider permissions
2. **Security:** Unauthorized access attempts logged and blocked
3. **Performance:** Authorization check adds <5ms latency
4. **Reliability:** Rate limiting prevents abuse without false positives
5. **Backward compat:** Existing keys and clients work without changes after migration

---

## Timeline Summary

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Phase 1: Domain & Port | 2-3 days | None |
| Phase 2: Storage | 3-4 days | Phase 1 |
| Phase 3: Authorization Core | 4-5 days | Phase 1, 2 |
| Phase 4: Middleware & Integration | 3-4 days | Phase 3 |
| Phase 5: API Endpoints | 3-4 days | Phase 4 |
| Phase 6: CLI Commands | 2-3 days | Phase 5 |
| Phase 7: Testing & Documentation | 3-4 days | All phases |
| **Total** | **20-27 days** | |

---

## Unresolved Questions

1. **Rate limit granularity:** Per-key or per-key-per-model?
   - **Recommendation:** Per-key (simpler, sufficient for v1)

2. **Rate limit state on restart:** In-memory resets on server restart — acceptable?
   - **Recommendation:** Yes for v1; SQLite persistence optional later

3. **Admin key rate limits:** Exempt or configurable?
   - **Recommendation:** Exempt by default

4. **Key expiration mid-request:** Fail or allow completion?
   - **Recommendation:** Allow completion (check at request start only)

5. **`requireApiKey=false` + admin middleware:** Should `/api/*` be protected even when `requireApiKey=false`?
   - **Recommendation:** Yes — if any keys exist, admin routes require a key; skip only if zero keys configured
