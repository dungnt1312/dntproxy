# Backend Architecture Redesign — DNTProxy

**Date:** 2026-04-15
**Type:** Full Rewrite (Simplified)
**Context:** Admin-managed API key gateway (OpenRouter-like model)

---

## Requirements Summary

### Core Model
- **Single admin** manages system via dashboard
- **End-users** consume API via keys (`/v1/chat/completions`)
- **NO multi-tenant, NO orgs, NO user accounts**
- **Simple deployment** — JSON config + SQLite logs

### API Key Capabilities
- Model whitelist (e.g., key only allows `kr/sonnet`, `oai/gpt-4`)
- Provider whitelist (e.g., key only allows `kiro`, `openai`)
- Rate limits per key (RPM, TPM)
- Credit balance per key (deduct on usage)
- Expiry date
- Auto load-balancing across provider accounts

### Must-Have Features
- Multi-provider routing (Kiro, OpenAI, Anthropic, GLM, MiniMax, Qwen)
- Multi-account per provider with fallback/cooldown
- Combo strategies (fallback, round-robin)
- OpenAI-compatible API
- OAuth flows (Builder ID, IDC, Social, PKCE)
- Structured logging + usage tracking
- Cloudflare tunnel
- Web dashboard

### Performance Target
- 100+ concurrent requests
- <100ms p99 latency

---

## Proposed Architecture

### Persistence Strategy — SIMPLIFIED

**JSON + In-Memory + SQLite logs**

```
config.json (hot-reloaded on change)
├── api_keys[]
├── provider_accounts[]
├── combos[]
├── model_pricing[]
└── settings{}

logs.db (SQLite, append-only)
└── request_logs table
```

**Rationale:**
- **Config changes are rare** — admin adds keys/accounts occasionally
- **No concurrent writes** — single admin, no race conditions
- **Fast reads** — everything in RAM, zero DB latency
- **Simple deployment** — no Postgres setup
- **Logs separate** — SQLite for append-only logs (no hot path)

---

## Core Domain Models

### 1. APIKey (Central Entity)

```go
type APIKey struct {
    ID              string    `json:"id"`
    Name            string    `json:"name"`
    KeyHash         string    `json:"key_hash"` // bcrypt hash
    
    // Permissions
    AllowedModels   []string  `json:"allowed_models"`   // ["kr/sonnet", "*"]
    AllowedProviders []string `json:"allowed_providers"` // ["kiro", "*"]
    
    // Rate Limits
    RPM             int       `json:"rpm"` // 0 = unlimited
    TPM             int       `json:"tpm"` // 0 = unlimited
    
    // Credits
    CreditBalance   float64   `json:"credit_balance"` // USD
    CreditLimit     float64   `json:"credit_limit"`   // max balance
    
    // Lifecycle
    ExpiresAt       *time.Time `json:"expires_at,omitempty"`
    IsActive        bool       `json:"is_active"`
    
    // Metadata
    Tags            []string  `json:"tags,omitempty"`
    Notes           string    `json:"notes,omitempty"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}
```

**Key Design Decisions:**
- **KeyHash instead of plaintext** — store bcrypt hash, show full key only on creation
- **Flexible permissions** — wildcard `["*"]` or specific whitelist
- **Credit balance per key** — deduct on each request
- **Rate limit per key** — enforce RPM/TPM independently

---

### 2. ProviderAccount (Dynamic, Admin-Managed)

```go
type ProviderAccount struct {
    ID              string    `json:"id"`
    ProviderID      string    `json:"provider_id"` // "kiro", "openai"
    Name            string    `json:"name"`
    
    // Auth
    AuthType        string    `json:"auth_type"` // "oauth", "apikey"
    Credentials     Credentials `json:"credentials"` // encrypted
    
    // Routing
    Priority        int       `json:"priority"` // lower = higher priority
    IsActive        bool      `json:"is_active"`
    
    // Health
    Status          string    `json:"status"` // "healthy", "cooldown", "unauthorized"
    LastError       string    `json:"last_error,omitempty"`
    LastErrorAt     *time.Time `json:"last_error_at,omitempty"`
    
    // Rate Limiting (per account)
    CooldownUntil   *time.Time `json:"cooldown_until,omitempty"`
    BackoffLevel    int        `json:"backoff_level"`
    
    // Model Restrictions
    SupportedModels []string              `json:"supported_models"` // ["*"] or specific
    ModelLocks      map[string]time.Time  `json:"model_locks,omitempty"`
    
    // Metadata
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

type Credentials struct {
    AccessToken  string     `json:"access_token,omitempty"`
    RefreshToken string     `json:"refresh_token,omitempty"`
    APIKey       string     `json:"api_key,omitempty"`
    ExpiresAt    *time.Time `json:"expires_at,omitempty"`
    Extra        map[string]interface{} `json:"extra,omitempty"`
}
```

**Key Design Decisions:**
- **Credentials as JSON** — flexible for different auth types
- **Priority-based routing** — admin controls fallback order
- **Per-account cooldown** — independent backoff state
- **Model locks** — prevent retrying failed models on same account

---

### 3. Combo (Model Chain)

```go
type Combo struct {
    ID        string   `json:"id"`
    Name      string   `json:"name"`
    Models    []string `json:"models"` // ["kr/sonnet", "oai/gpt-4"]
    Strategy  string   `json:"strategy"` // "fallback", "round-robin"
    IsActive  bool     `json:"is_active"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

---

### 4. ModelPricing

```go
type ModelPricing struct {
    ID           string  `json:"id"`
    ProviderID   string  `json:"provider_id"`
    ModelPattern string  `json:"model_pattern"` // "claude-sonnet*"
    InputPrice   float64 `json:"input_price"`   // per 1M tokens
    OutputPrice  float64 `json:"output_price"`
    Currency     string  `json:"currency"` // "USD"
}
```

---

### 5. Config Structure (config.json)

```json
{
  "api_keys": [
    {
      "id": "pk-abc123",
      "name": "Production Key",
      "key_hash": "$2a$10$...",
      "allowed_models": ["kr/sonnet", "oai/gpt-4"],
      "allowed_providers": ["*"],
      "rpm": 60,
      "tpm": 100000,
      "credit_balance": 100.00,
      "credit_limit": 1000.00,
      "expires_at": "2027-01-01T00:00:00Z",
      "is_active": true,
      "tags": ["customer-a"],
      "created_at": "2026-04-15T00:00:00Z",
      "updated_at": "2026-04-15T00:00:00Z"
    }
  ],
  "provider_accounts": [
    {
      "id": "acc-xyz789",
      "provider_id": "kiro",
      "name": "Kiro Main",
      "auth_type": "oauth",
      "credentials": {
        "access_token": "encrypted...",
        "refresh_token": "encrypted...",
        "expires_at": "2026-04-16T00:00:00Z"
      },
      "priority": 1,
      "is_active": true,
      "status": "healthy",
      "cooldown_until": null,
      "backoff_level": 0,
      "supported_models": ["*"],
      "model_locks": {},
      "created_at": "2026-04-15T00:00:00Z",
      "updated_at": "2026-04-15T00:00:00Z"
    }
  ],
  "combos": [
    {
      "id": "combo-001",
      "name": "fast-fallback",
      "models": ["kr/sonnet", "oai/gpt-4"],
      "strategy": "fallback",
      "is_active": true,
      "created_at": "2026-04-15T00:00:00Z",
      "updated_at": "2026-04-15T00:00:00Z"
    }
  ],
  "model_pricing": [
    {
      "id": "price-001",
      "provider_id": "kiro",
      "model_pattern": "claude-sonnet*",
      "input_price": 3.00,
      "output_price": 15.00,
      "currency": "USD"
    }
  ],
  "settings": {
    "port": 20199,
    "require_api_key": true,
    "tunnel_enabled": false,
    "tunnel_url": ""
  }
}
```

---

### 6. SQLite Schema (logs.db)

```sql
CREATE TABLE request_logs (
    id              TEXT PRIMARY KEY,
    api_key_id      TEXT NOT NULL,
    
    model           TEXT NOT NULL,
    combo_name      TEXT,
    
    provider_id     TEXT,
    account_id      TEXT,
    resolved_model  TEXT,
    
    started_at      INTEGER NOT NULL, -- unix timestamp ms
    duration_ms     INTEGER,
    
    input_tokens    INTEGER DEFAULT 0,
    output_tokens   INTEGER DEFAULT 0,
    total_tokens    INTEGER DEFAULT 0,
    
    input_cost      REAL DEFAULT 0,
    output_cost     REAL DEFAULT 0,
    total_cost      REAL DEFAULT 0,
    
    status_code     INTEGER,
    error           TEXT,
    
    request_id      TEXT,
    user_agent      TEXT
);

CREATE INDEX idx_request_logs_api_key ON request_logs(api_key_id, started_at DESC);
CREATE INDEX idx_request_logs_started_at ON request_logs(started_at DESC);
```

**Key Design Decisions:**
- **REAL for costs** — SQLite doesn't have NUMERIC, but logs are read-only so acceptable
- **INTEGER for timestamps** — unix ms, simpler than TEXT dates
- **No foreign keys** — logs are append-only, no referential integrity needed

---

## Layer Structure

```
┌─────────────────────────────────────────────────────────┐
│                     HTTP Layer                          │
│  - API routes (/v1/chat/completions)                   │
│  - Admin routes (/admin/*)                             │
│  - Middleware (auth, rate limit, logging)              │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                   Service Layer                         │
│  - RequestOrchestrator (main flow)                     │
│  - KeyValidator                                         │
│  - CreditManager                                        │
│  - ProviderRouter                                       │
│  - ComboExecutor                                        │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                   Storage Layer                         │
│  - ConfigStore (in-memory + JSON file)                 │
│  - LogStore (SQLite)                                    │
│  - FileWatcher (auto-reload config)                    │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                Infrastructure Layer                     │
│  - Provider adapters (Kiro, OpenAI, Anthropic)        │
│  - Token bucket rate limiter (in-memory)              │
│  - Tunnel manager                                       │
└─────────────────────────────────────────────────────────┘
```

---

## Service Layer Design

### ConfigStore (In-Memory + File)

```go
type ConfigStore struct {
    mu          sync.RWMutex
    config      *Config
    filePath    string
    watcher     *fsnotify.Watcher
    
    // In-memory indexes for fast lookup
    keysByHash  map[string]*APIKey
    accountsByProvider map[string][]*ProviderAccount
    combosByName map[string]*Combo
}

func (s *ConfigStore) Load() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    data, err := os.ReadFile(s.filePath)
    if err != nil {
        return err
    }
    
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return err
    }
    
    // Build indexes
    s.config = &cfg
    s.rebuildIndexes()
    return nil
}

func (s *ConfigStore) Save() error {
    s.mu.RLock()
    data, err := json.MarshalIndent(s.config, "", "  ")
    s.mu.RUnlock()
    
    if err != nil {
        return err
    }
    
    // Atomic write: .tmp -> rename
    tmpPath := s.filePath + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0600); err != nil {
        return err
    }
    return os.Rename(tmpPath, s.filePath)
}

func (s *ConfigStore) GetKeyByHash(hash string) (*APIKey, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    key, ok := s.keysByHash[hash]
    return key, ok
}

func (s *ConfigStore) GetHealthyAccounts(providerID string) []*ProviderAccount {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    accounts := s.accountsByProvider[providerID]
    healthy := make([]*ProviderAccount, 0, len(accounts))
    
    now := time.Now()
    for _, acc := range accounts {
        if !acc.IsActive {
            continue
        }
        if acc.CooldownUntil != nil && now.Before(*acc.CooldownUntil) {
            continue
        }
        healthy = append(healthy, acc)
    }
    
    // Sort by priority
    sort.Slice(healthy, func(i, j int) bool {
        return healthy[i].Priority < healthy[j].Priority
    })
    
    return healthy
}

func (s *ConfigStore) UpdateAccountStatus(accountID string, status string, err error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    for _, acc := range s.config.ProviderAccounts {
        if acc.ID == accountID {
            acc.Status = status
            if err != nil {
                acc.LastError = err.Error()
                now := time.Now()
                acc.LastErrorAt = &now
            }
            acc.UpdatedAt = time.Now()
            break
        }
    }
    
    // Auto-save on status change
    go s.Save()
}
```

**Key Design Decisions:**
- **RWMutex** — many concurrent reads, rare writes
- **In-memory indexes** — O(1) lookup for hot paths
- **Auto-reload on file change** — fsnotify watches config.json
- **Async save** — don't block on status updates

---

### RequestOrchestrator (Main Flow)

```go
type RequestOrchestrator struct {
    configStore   *ConfigStore
    providerRouter *ProviderRouter
    comboExecutor  *ComboExecutor
    rateLimiter    *RateLimiter
    creditManager  *CreditManager
    logger         *AsyncLogger
}

func (o *RequestOrchestrator) HandleChatCompletion(
    ctx context.Context,
    apiKey string,
    req *ChatRequest,
) (*ChatResponse, error) {
    // 1. Validate API key
    keyHash := bcrypt.Hash(apiKey)
    key, ok := o.configStore.GetKeyByHash(keyHash)
    if !ok || !key.IsActive {
        return nil, ErrUnauthorized // 401
    }
    
    // 2. Check expiry
    if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
        return nil, ErrKeyExpired // 403
    }
    
    // 3. Check permissions
    if !o.isModelAllowed(key, req.Model) {
        return nil, ErrModelNotAllowed // 403
    }
    
    // 4. Check rate limits
    if !o.rateLimiter.Allow(key.ID, key.RPM, key.TPM) {
        return nil, ErrRateLimitExceeded // 429
    }
    
    // 5. Pre-flight credit check
    estimatedCost := o.creditManager.EstimateCost(req)
    if key.CreditBalance < estimatedCost {
        return nil, ErrInsufficientCredits // 402
    }
    
    // 6. Route request
    var resp *ChatResponse
    var err error
    
    if o.isCombo(req.Model) {
        resp, err = o.comboExecutor.Execute(ctx, key, req)
    } else {
        resp, err = o.providerRouter.Execute(ctx, key, req)
    }
    
    if err != nil {
        return nil, err
    }
    
    // 7. Deduct credits (async)
    actualCost := o.creditManager.CalculateCost(resp.Usage)
    go o.creditManager.Deduct(key.ID, actualCost)
    
    // 8. Log request (async)
    go o.logger.LogRequest(ctx, key.ID, req, resp)
    
    return resp, nil
}
```

**Key Design Decisions:**
- **Pre-flight checks** — fail fast before expensive operations
- **Async credit deduction** — don't block response
- **Async logging** — don't block response
- **Simple error handling** — return typed errors with HTTP status

---

### RateLimiter (Token Bucket, In-Memory)

```go
type RateLimiter struct {
    buckets sync.Map // keyID -> *TokenBucket
}

type TokenBucket struct {
    mu          sync.Mutex
    tokens      float64
    capacity    float64
    refillRate  float64 // tokens per second
    lastRefill  time.Time
}

func (r *RateLimiter) Allow(keyID string, rpm, tpm int) bool {
    if rpm == 0 && tpm == 0 {
        return true // unlimited
    }
    
    // Check RPM
    if rpm > 0 {
        bucket := r.getBucket(keyID+":rpm", float64(rpm), float64(rpm)/60.0)
        if !bucket.Allow() {
            return false
        }
    }
    
    // Check TPM (estimated, actual tokens unknown at this point)
    if tpm > 0 {
        bucket := r.getBucket(keyID+":tpm", float64(tpm), float64(tpm)/60.0)
        if !bucket.Allow() {
            return false
        }
    }
    
    return true
}

func (r *RateLimiter) getBucket(key string, capacity, refillRate float64) *TokenBucket {
    val, _ := r.buckets.LoadOrStore(key, &TokenBucket{
        tokens:     capacity,
        capacity:   capacity,
        refillRate: refillRate,
        lastRefill: time.Now(),
    })
    return val.(*TokenBucket)
}

func (b *TokenBucket) Allow() bool {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    // Refill tokens
    now := time.Now()
    elapsed := now.Sub(b.lastRefill).Seconds()
    b.tokens = math.Min(b.capacity, b.tokens + elapsed*b.refillRate)
    b.lastRefill = now
    
    // Check if token available
    if b.tokens >= 1 {
        b.tokens -= 1
        return true
    }
    return false
}
```

**Key Design Decisions:**
- **In-memory** — no DB hit on every request
- **Token bucket** — smooth rate limiting, allows bursts
- **Per-key buckets** — isolated limits
- **Lazy initialization** — buckets created on first use

---

### CreditManager (Balance Tracking)

```go
type CreditManager struct {
    configStore *ConfigStore
    pricingTable map[string]*ModelPricing // provider/model -> pricing
}

func (m *CreditManager) Deduct(keyID string, amount float64) error {
    // Update in-memory balance
    m.configStore.mu.Lock()
    defer m.configStore.mu.Unlock()
    
    for _, key := range m.configStore.config.APIKeys {
        if key.ID == keyID {
            key.CreditBalance -= amount
            key.UpdatedAt = time.Now()
            break
        }
    }
    
    // Async save to disk
    go m.configStore.Save()
    
    return nil
}

func (m *CreditManager) CalculateCost(usage *Usage) float64 {
    pricing := m.pricingTable[usage.Provider+"/"+usage.Model]
    if pricing == nil {
        return 0
    }
    
    inputCost := float64(usage.InputTokens) / 1_000_000 * pricing.InputPrice
    outputCost := float64(usage.OutputTokens) / 1_000_000 * pricing.OutputPrice
    
    return inputCost + outputCost
}
```

**Key Design Decisions:**
- **No transactions** — single admin, no concurrent deductions
- **Async save** — don't block response
- **In-memory pricing table** — fast cost calculation

---

## Performance Optimizations

### 1. In-Memory Everything (Config)
- Zero DB latency on key/account lookup
- RWMutex for concurrent reads
- Indexes for O(1) lookup

### 2. Async Operations
- Credit deduction (don't block response)
- Logging (don't block response)
- Config save (don't block status updates)

### 3. Token Bucket Rate Limiter
- In-memory, no DB hit
- Smooth rate limiting
- Per-key isolation

### 4. Batch Logging
- Queue 100 logs or 1s flush
- Single SQLite transaction per batch
- Backpressure: drop logs if queue full

**Expected:** <50ms p99 for 100+ concurrent requests (no DB on hot path)

---

## Migration Strategy

### Phase 1: Core Refactor (Week 1)
- [ ] New domain models (APIKey, ProviderAccount, Combo)
- [ ] ConfigStore (in-memory + JSON)
- [ ] LogStore (SQLite)
- [ ] Unit tests

### Phase 2: Service Layer (Week 2)
- [ ] RequestOrchestrator
- [ ] RateLimiter (token bucket)
- [ ] CreditManager
- [ ] ProviderRouter (with fallback)
- [ ] ComboExecutor
- [ ] Integration tests

### Phase 3: Provider Adapters (Week 3)
- [ ] Refactor Kiro adapter (fix EventStream CRC)
- [ ] Refactor OpenAI adapter (fix Codex translation)
- [ ] Refactor Anthropic adapter (fix SSE format)
- [ ] Add GLM, MiniMax, Qwen adapters

### Phase 4: HTTP Layer (Week 4)
- [ ] New HTTP handlers (chat, admin)
- [ ] Middleware (auth, rate limit, logging)
- [ ] API key authentication
- [ ] Admin dashboard API

### Phase 5: Features (Week 5)
- [ ] OAuth flows
- [ ] Tunnel integration
- [ ] File watcher (auto-reload config)
- [ ] Performance testing

### Phase 6: Dashboard (Week 6)
- [ ] React UI refactor
- [ ] API key management
- [ ] Provider account management
- [ ] Usage analytics
- [ ] Credit top-up UI

---

## Trade-offs & Risks

### ✅ Pros
- **Simple** — no DB setup, just JSON + SQLite
- **Fast** — everything in RAM, <50ms p99
- **Easy deployment** — single binary + 2 files
- **Git-friendly** — config.json can be version-controlled
- **Type-safe** — proper Go structs, no stringly-typed
- **Maintainable** — clean separation of concerns

### ⚠️ Cons
- **Single-instance only** — can't scale horizontally (but not needed)
- **No ACID for credits** — deduction is async (acceptable for single admin)
- **Config reload delay** — fsnotify has ~100ms latency
- **Memory usage** — all config in RAM (but tiny for single admin)

### 🔴 Risks
- **Config corruption** — if JSON is manually edited incorrectly
- **Credit drift** — if process crashes during deduction (rare, acceptable)
- **Rate limit reset** — if process restarts, buckets reset (acceptable)
- **Log loss** — if queue full, logs dropped (acceptable)

---

## Comparison with Current Architecture

| Aspect | Current | Proposed |
|--------|---------|----------|
| **Persistence** | JSON file + SQLite | Same (but cleaner) |
| **API Keys** | Simple auth, no credits | Full-featured (credits, limits, expiry) |
| **Rate Limiting** | Per-account cooldown only | Per-key + per-account |
| **Logging** | Async writer with ring buffer | Async batch writer |
| **Caching** | None | In-memory indexes |
| **Type Safety** | Strings everywhere | Proper structs + enums |
| **Concurrency** | Many race conditions | RWMutex + proper locking |
| **Testing** | 1 test file | Full unit + integration tests |
| **Performance** | ~100ms p99 | <50ms p99 (no DB on hot path) |

---

## Next Steps

1. **Review & Approve** — discuss trade-offs, adjust design
2. **Create Implementation Plan** — break down into tasks with `/ck:plan`
3. **Set up new repo branch** — `feature/backend-redesign`
4. **Start Phase 1** — domain models + ConfigStore

---

## Answers to Questions

1. **Postgres vs SQLite?** — **SQLite for logs only**, JSON for config
2. **Credit precision?** — **float64** (acceptable for single admin, no concurrent deductions)
3. **Rate limit algorithm?** — **Token bucket** (smooth, allows bursts)
4. **Cache TTL?** — **No cache needed** (everything in RAM)
5. **Async logging backpressure?** — **Drop logs** (better than OOM)

---

**Status:** Ready for implementation plan
