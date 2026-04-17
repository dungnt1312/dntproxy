# API Key Authorization System - Brainstorm Summary

**Date:** 2026-04-17  
**Status:** Agreed Solution  
**Next Step:** Implementation Planning

---

## Problem Statement

dntproxy currently has basic API key generation but lacks granular authorization. Need to add:
- Admin vs user API key roles
- Hierarchical permissions (model/connection/provider level)
- Rate limiting (requests, tokens, cost)
- Usage quotas and expiration
- Audit logging for all auth decisions
- Dashboard protection requiring admin keys

## Requirements

### Core Authorization
- **Role-based access**: Admin keys (full dashboard access) vs User keys (limited to /v1/* endpoints)
- **Hierarchical permissions**: Model-level > Connection-level > Provider-level
- **Hybrid permission model**: Support both allow/deny rules (deny overrides allow)
- **No wildcards**: Exact match only for simplicity (e.g., 'gpt-4', not 'gpt-*')

### API Key Features
- **Rate limiting**: Requests/min/hour/day, tokens/min/hour/day, cost/day/month
- **Usage quotas**: Token limits, cost budgets
- **Expiration**: Optional TTL or timestamp
- **Scopes/roles**: Predefined permission templates (admin, read-only, dev, prod)

### Integration
- **Admin authentication**: Admin API keys for dashboard access
- **Dashboard protection**: All /api/* endpoints require admin key
- **Backward compatibility**: Existing keys auto-migrate to admin role
- **Audit logging**: Log all auth decisions (allow/deny) to SQLite

### User Experience
- **Permission assignment**: Set during creation + allow editing later
- **Admin-only creation**: Only admin keys can create new keys
- **Transparent filtering**: Account selector only sees permitted connections

---

## Evaluated Approaches

### Option 1: Simple Role-Based (Admin/User)
**Pros:**
- Easy to implement and understand
- Minimal schema changes
- Fast authorization checks

**Cons:**
- Not granular enough for multi-tenant scenarios
- Can't restrict specific models per user key
- Limited flexibility for future growth

### Option 2: Full RBAC with Policies
**Pros:**
- Industry-standard approach
- Highly flexible and extensible
- Supports complex permission scenarios

**Cons:**
- Over-engineered for current needs (violates YAGNI)
- Adds significant complexity to codebase
- Slower authorization checks (policy evaluation)

### Option 3: Hierarchical Permissions with Hybrid Allow/Deny ✅ **RECOMMENDED**
**Pros:**
- Balances simplicity and flexibility
- Supports exact use case without over-engineering
- Clear permission precedence rules
- Easy to audit and debug
- Extensible for future needs

**Cons:**
- More complex than Option 1 (acceptable trade-off)
- Requires careful permission evaluation logic

---

## Final Solution Architecture

### 1. Domain Model Changes

#### Enhanced APIKey Structure
```go
type APIKey struct {
    ID          string              `json:"id"`
    Name        string              `json:"name"`
    Key         string              `json:"key"`
    IsActive    bool                `json:"isActive"`
    CreatedAt   string              `json:"createdAt"`
    
    // New fields
    Role        string              `json:"role"`        // "admin" | "user"
    Permissions APIKeyPermissions   `json:"permissions"`
    RateLimits  RateLimits          `json:"rateLimits,omitempty"`
    Quotas      UsageQuotas         `json:"quotas,omitempty"`
    ExpiresAt   *string             `json:"expiresAt,omitempty"`
    Metadata    map[string]string   `json:"metadata,omitempty"`
}

type APIKeyPermissions struct {
    // Hierarchical: model > connection > provider
    AllowModels      []string `json:"allowModels,omitempty"`      // exact model names
    DenyModels       []string `json:"denyModels,omitempty"`
    AllowConnections []string `json:"allowConnections,omitempty"` // connection IDs
    DenyConnections  []string `json:"denyConnections,omitempty"`
    AllowProviders   []string `json:"allowProviders,omitempty"`   // provider types
    DenyProviders    []string `json:"denyProviders,omitempty"`
}

type RateLimits struct {
    RequestsPerMinute int `json:"requestsPerMinute,omitempty"`
    RequestsPerHour   int `json:"requestsPerHour,omitempty"`
    RequestsPerDay    int `json:"requestsPerDay,omitempty"`
    TokensPerMinute   int `json:"tokensPerMinute,omitempty"`
    TokensPerHour     int `json:"tokensPerHour,omitempty"`
    TokensPerDay      int `json:"tokensPerDay,omitempty"`
    CostPerDay        float64 `json:"costPerDay,omitempty"`    // USD
    CostPerMonth      float64 `json:"costPerMonth,omitempty"`  // USD
}

type UsageQuotas struct {
    MaxTokens int     `json:"maxTokens,omitempty"`
    MaxCost   float64 `json:"maxCost,omitempty"`
}
```

#### Auth Event Logging
```go
type AuthEvent struct {
    ID          string `json:"id"`
    Timestamp   string `json:"timestamp"`
    APIKeyID    string `json:"apiKeyId"`
    APIKeyName  string `json:"apiKeyName"`
    Action      string `json:"action"`      // "chat", "list_models", "create_key", etc.
    Resource    string `json:"resource"`    // model/connection/provider identifier
    Decision    string `json:"decision"`    // "allow" | "deny"
    Reason      string `json:"reason"`      // why denied (if applicable)
    RequestPath string `json:"requestPath"`
    IPAddress   string `json:"ipAddress,omitempty"`
}
```

### 2. Authorization Flow

```
Request → Extract API Key → Validate Key → Check Role → Check Permissions → Check Rate Limits → Check Quotas → Allow/Deny
```

#### Permission Evaluation Logic
1. **Admin keys**: Skip permission checks, always allow (except rate limits/quotas)
2. **User keys**: Evaluate hierarchically
   - Check model-level permissions first (most specific)
   - If no model rules, check connection-level
   - If no connection rules, check provider-level
   - If no rules at any level, default deny
3. **Deny overrides allow**: If resource is in both allow and deny lists, deny wins
4. **Rate limiting**: Check current usage against limits (sliding window)
5. **Quota enforcement**: Check cumulative usage against quotas

### 3. Implementation Components

#### New Port Interfaces
```go
// port/authorizer.go
type Authorizer interface {
    Authorize(ctx context.Context, apiKey *domain.APIKey, req AuthRequest) (*AuthDecision, error)
    CheckRateLimit(apiKeyID string, tokens int) error
    CheckQuota(apiKeyID string, tokens int, cost float64) error
}

type AuthRequest struct {
    Action       string // "chat", "list_models", "create_key", etc.
    Model        string
    ConnectionID string
    Provider     string
}

type AuthDecision struct {
    Allowed bool
    Reason  string
}

// port/auth-event-store.go
type AuthEventStore interface {
    LogAuthEvent(event *domain.AuthEvent) error
    GetAuthEvents(filter AuthEventFilter) ([]domain.AuthEvent, error)
}
```

#### New Adapter Implementations
- `internal/adapter/auth/authorizer.go` - Permission evaluation logic
- `internal/adapter/auth/rate-limiter.go` - Sliding window rate limiting
- `internal/adapter/auth/quota-tracker.go` - Usage quota tracking
- `internal/adapter/storage/auth-event-store.go` - SQLite auth event persistence

#### Middleware Updates
- `internal/adapter/http/router.go` - Split middleware:
  - `adminKeyMiddleware()` - Protect /api/* endpoints
  - `userKeyMiddleware()` - Protect /v1/* endpoints
- `internal/adapter/http/auth-middleware.go` - New file with authorization logic

#### Service Layer Updates
- `internal/service/chat-service.go` - Add authorization check before model resolution
- `internal/service/account-selector.go` - Filter connections by API key permissions

### 4. Database Schema

#### SQLite Auth Events Table
```sql
CREATE TABLE auth_events (
    id TEXT PRIMARY KEY,
    timestamp TEXT NOT NULL,
    api_key_id TEXT NOT NULL,
    api_key_name TEXT NOT NULL,
    action TEXT NOT NULL,
    resource TEXT NOT NULL,
    decision TEXT NOT NULL,
    reason TEXT,
    request_path TEXT NOT NULL,
    ip_address TEXT,
    INDEX idx_timestamp (timestamp),
    INDEX idx_api_key_id (api_key_id),
    INDEX idx_decision (decision)
);
```

#### Rate Limit State (In-Memory + Periodic Flush)
```go
// In-memory sliding window counters
type RateLimitState struct {
    APIKeyID      string
    WindowStart   time.Time
    RequestCount  int
    TokenCount    int
    CostAccum     float64
}
```

### 5. API Endpoints

#### New Management Endpoints
```
POST   /api/keys                    - Create API key (admin only)
GET    /api/keys                    - List API keys (admin only)
GET    /api/keys/:id                - Get API key details (admin only)
PUT    /api/keys/:id                - Update API key permissions (admin only)
DELETE /api/keys/:id                - Delete API key (admin only)
POST   /api/keys/:id/rotate         - Rotate API key secret (admin only)
GET    /api/keys/:id/usage          - Get usage stats for key (admin only)
GET    /api/auth-events             - List auth events (admin only)
GET    /api/auth-events/stream      - SSE stream of auth events (admin only)
```

#### Updated Endpoints
```
POST /v1/chat/completions           - Add authorization check
GET  /v1/models                     - Filter by API key permissions
```

### 6. CLI Commands

```bash
# Generate admin key (first-time setup)
dntproxy key create --name "Admin Key" --role admin

# Generate user key with permissions
dntproxy key create --name "Dev Key" --role user \
  --allow-models "gpt-4,claude-3-opus-20240229" \
  --allow-providers "openai,anthropic" \
  --rate-limit-rpm 60 \
  --rate-limit-tpd 100000 \
  --expires-in 30d

# List keys
dntproxy key list

# Update permissions
dntproxy key update <id> --allow-connections "conn-123,conn-456"

# Rotate key
dntproxy key rotate <id>

# View usage
dntproxy key usage <id>

# View auth events
dntproxy auth-events --key <id> --decision deny
```

---

## Implementation Considerations

### Security
- **Key storage**: Continue using secure random generation (24 bytes hex)
- **Key prefix**: Keep `sk-dnt-` prefix for easy identification
- **Admin key protection**: First admin key created becomes "root" (cannot be deleted)
- **Audit trail**: All auth decisions logged with timestamp, key, resource, decision
- **Rate limit bypass**: Admin keys exempt from rate limits (optional config)

### Performance
- **In-memory caching**: Cache API key lookups (5min TTL)
- **Rate limit state**: In-memory sliding window, periodic flush to SQLite
- **Permission evaluation**: O(n) where n = number of rules (acceptable for <100 rules per key)
- **Auth event logging**: Async write to SQLite (buffered channel)

### Backward Compatibility
- **Migration**: On first load, existing keys get `role: "admin"` and empty permissions
- **Default behavior**: If `requireApiKey: false`, skip all auth checks (current behavior)
- **Graceful degradation**: If auth event store fails, log to stderr but allow request

### User Experience
- **Dashboard UI**: New "API Keys" page with permission builder
- **Permission templates**: Predefined templates (admin, read-only, dev, prod)
- **Usage dashboard**: Real-time usage stats per key
- **Auth event viewer**: Filterable log viewer with SSE live updates

### Monitoring
- **Metrics**: Track auth decisions (allow/deny ratio), rate limit hits, quota exhaustion
- **Alerts**: Log warnings when keys approach quota limits
- **Audit reports**: Generate daily/weekly auth event summaries

---

## Success Metrics

1. **Functional**: Admin can create user keys with specific model/connection/provider permissions
2. **Security**: Unauthorized access attempts logged and blocked
3. **Performance**: Authorization check adds <5ms latency to requests
4. **Usability**: Permission assignment takes <2 minutes via dashboard
5. **Reliability**: Rate limiting prevents abuse without false positives

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Complex permission logic causes bugs | High | Comprehensive unit tests, integration tests with real scenarios |
| Rate limiting false positives | Medium | Sliding window algorithm, configurable limits, admin bypass |
| Auth event logging fills disk | Medium | 30-day retention, configurable retention policy, log rotation |
| Performance degradation | Low | In-memory caching, async logging, benchmark tests |
| Migration breaks existing keys | High | Careful migration logic, rollback plan, version detection |

---

## Next Steps

1. **Create implementation plan** with detailed phases
2. **Design database migrations** for auth events table
3. **Implement core authorization logic** (port + adapter)
4. **Add middleware and service integration**
5. **Build dashboard UI** for key management
6. **Write comprehensive tests** (unit + integration)
7. **Update documentation** (API docs, CLI help, README)

---

## Unresolved Questions

1. Should rate limits be per-key or per-key-per-model?
2. How to handle rate limit state persistence across restarts? (In-memory only vs SQLite)
3. Should admin keys have separate rate limits or be exempt?
4. What happens when a key expires mid-request? (Fail gracefully vs allow completion)
5. Should we support IP-based rate limiting in addition to key-based?
