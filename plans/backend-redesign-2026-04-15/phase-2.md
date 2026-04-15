---
status: pending
priority: high
effort: 1 week
phase: 2
dependencies: [phase-1]
---

# Phase 2: Service Layer (Week 2)

## Goals

Build the business logic layer that orchestrates request handling, rate limiting, credit management, provider routing, and combo execution.

## Tasks

### 2.1 RateLimiter (Token Bucket)

**File:** `internal/service/rate-limiter.go`

Implement in-memory token bucket rate limiter:

```go
type RateLimiter struct {
    buckets sync.Map // keyID:type -> *TokenBucket
}

type TokenBucket struct {
    mu          sync.Mutex
    tokens      float64
    capacity    float64
    refillRate  float64
    lastRefill  time.Time
}

func NewRateLimiter() *RateLimiter
func (r *RateLimiter) AllowRPM(keyID string, rpm int) bool
func (r *RateLimiter) AllowTPM(keyID string, tpm int, estimatedTokens int) bool
func (r *RateLimiter) Allow(keyID string, rpm, tpm, estimatedTokens int) bool
func (r *RateLimiter) Reset(keyID string)
func (b *TokenBucket) Allow(tokens float64) bool
func (b *TokenBucket) refill()
```

**Key Features:**
- Token bucket algorithm (smooth rate limiting)
- Per-key buckets (isolated limits)
- Lazy initialization (buckets created on first use)
- Automatic refill based on elapsed time
- Thread-safe (sync.Map + per-bucket mutex)

**Acceptance Criteria:**
- [ ] AllowRPM enforces requests per minute
- [ ] AllowTPM enforces tokens per minute
- [ ] Allow checks both RPM and TPM
- [ ] Buckets refill smoothly over time
- [ ] Thread-safe (verified with -race)
- [ ] Unit tests with 95%+ coverage
- [ ] Benchmark: <1μs per check

---

### 2.2 CreditManager

**File:** `internal/service/credit-manager.go`

Implement credit balance tracking and cost calculation:

```go
type CreditManager struct {
    configStore  *storage.ConfigStore
    pricingTable map[string]*domain.ModelPricing
    mu           sync.RWMutex
}

func NewCreditManager(store *storage.ConfigStore) *CreditManager
func (m *CreditManager) LoadPricing() error
func (m *CreditManager) EstimateCost(model string, estimatedTokens int) float64
func (m *CreditManager) CalculateCost(providerID, model string, inputTokens, outputTokens int) float64
func (m *CreditManager) Deduct(keyID string, amount float64) error
func (m *CreditManager) GetBalance(keyID string) (float64, error)
func (m *CreditManager) AddCredits(keyID string, amount float64) error
func (m *CreditManager) matchPricing(providerID, model string) *domain.ModelPricing
```

**Key Features:**
- In-memory pricing table (fast lookup)
- Pattern matching for model pricing (e.g., "claude-sonnet*")
- Async deduction (don't block response)
- Pre-flight cost estimation
- Thread-safe operations

**Acceptance Criteria:**
- [ ] LoadPricing builds in-memory table
- [ ] EstimateCost returns reasonable estimate
- [ ] CalculateCost uses actual token counts
- [ ] Deduct updates balance and persists async
- [ ] Pattern matching works (e.g., "claude-*")
- [ ] Thread-safe (verified with -race)
- [ ] Unit tests with 95%+ coverage

---

### 2.3 ProviderRouter

**File:** `internal/service/provider-router.go`

Implement provider account selection with fallback and cooldown:

```go
type ProviderRouter struct {
    configStore *storage.ConfigStore
    executors   map[string]ProviderExecutor
    mu          sync.RWMutex
}

type ProviderExecutor interface {
    Execute(ctx context.Context, account *domain.ProviderAccount, req *ChatRequest) (*ChatResponse, error)
}

func NewProviderRouter(store *storage.ConfigStore) *ProviderRouter
func (r *ProviderRouter) RegisterExecutor(providerID string, executor ProviderExecutor)
func (r *ProviderRouter) Execute(ctx context.Context, providerID, model string) (*ChatResponse, error)
func (r *ProviderRouter) selectAccount(providerID, model string) (*domain.ProviderAccount, error)
func (r *ProviderRouter) handleSuccess(accountID string)
func (r *ProviderRouter) handleError(accountID string, err error)
func (r *ProviderRouter) calculateCooldown(backoffLevel int) time.Duration
```

**Key Features:**
- Priority-based account selection
- Cooldown on error (exponential backoff: 1s → 2s → 4s → ... → 2min max)
- Model locks (prevent retrying failed model on same account)
- Clear cooldown on success
- Fallback to next account on error

**Acceptance Criteria:**
- [ ] selectAccount returns highest priority healthy account
- [ ] selectAccount skips accounts in cooldown
- [ ] selectAccount skips accounts with model locks
- [ ] handleError sets cooldown with exponential backoff
- [ ] handleError adds model lock
- [ ] handleSuccess clears cooldown and backoff
- [ ] Fallback tries all accounts before failing
- [ ] Thread-safe (verified with -race)
- [ ] Unit tests with 95%+ coverage

---

### 2.4 ComboExecutor

**File:** `internal/service/combo-executor.go`

Implement combo strategy execution (fallback, round-robin):

```go
type ComboExecutor struct {
    configStore    *storage.ConfigStore
    providerRouter *ProviderRouter
    roundRobinIdx  sync.Map // comboName -> int
}

func NewComboExecutor(store *storage.ConfigStore, router *ProviderRouter) *ComboExecutor
func (e *ComboExecutor) Execute(ctx context.Context, comboName string, req *ChatRequest) (*ChatResponse, error)
func (e *ComboExecutor) executeFallback(ctx context.Context, models []string, req *ChatRequest) (*ChatResponse, error)
func (e *ComboExecutor) executeRoundRobin(ctx context.Context, models []string, req *ChatRequest) (*ChatResponse, error)
func (e *ComboExecutor) parseModel(model string) (providerID, modelName string, err error)
```

**Key Features:**
- Fallback strategy: try models in order until success
- Round-robin strategy: rotate starting model each request
- Atomic round-robin counter (sync.Map)
- Parse model format (e.g., "kr/sonnet" → "kiro", "sonnet")

**Acceptance Criteria:**
- [ ] executeFallback tries models in order
- [ ] executeFallback stops on first success
- [ ] executeFallback returns last error if all fail
- [ ] executeRoundRobin rotates starting model
- [ ] executeRoundRobin falls back to next on error
- [ ] parseModel handles all formats (kr/, oai/, etc.)
- [ ] Thread-safe (verified with -race)
- [ ] Unit tests with 95%+ coverage

---

### 2.5 RequestOrchestrator

**File:** `internal/service/request-orchestrator.go`

Implement main request flow orchestration:

```go
type RequestOrchestrator struct {
    configStore   *storage.ConfigStore
    logStore      *storage.LogStore
    rateLimiter   *RateLimiter
    creditManager *CreditManager
    providerRouter *ProviderRouter
    comboExecutor  *ComboExecutor
}

type ChatRequest struct {
    Model       string
    Messages    []Message
    Stream      bool
    Temperature float64
    MaxTokens   int
    Tools       []Tool
}

type ChatResponse struct {
    ID      string
    Model   string
    Choices []Choice
    Usage   Usage
}

type Usage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
}

func NewRequestOrchestrator(
    configStore *storage.ConfigStore,
    logStore *storage.LogStore,
    rateLimiter *RateLimiter,
    creditManager *CreditManager,
    providerRouter *ProviderRouter,
    comboExecutor *ComboExecutor,
) *RequestOrchestrator

func (o *RequestOrchestrator) HandleChatCompletion(
    ctx context.Context,
    apiKey string,
    req *ChatRequest,
) (*ChatResponse, error)

func (o *RequestOrchestrator) validateKey(apiKey string) (*domain.APIKey, error)
func (o *RequestOrchestrator) checkPermissions(key *domain.APIKey, model string) error
func (o *RequestOrchestrator) checkRateLimit(key *domain.APIKey, estimatedTokens int) error
func (o *RequestOrchestrator) checkCredits(key *domain.APIKey, model string, estimatedTokens int) error
func (o *RequestOrchestrator) routeRequest(ctx context.Context, model string, req *ChatRequest) (*ChatResponse, error)
func (o *RequestOrchestrator) postProcess(key *domain.APIKey, req *ChatRequest, resp *ChatResponse) error
```

**Request Flow:**
```
1. Validate API key (hash lookup)
2. Check expiry
3. Check permissions (model/provider whitelist)
4. Check rate limits (RPM, TPM)
5. Pre-flight credit check (estimate cost)
6. Route request (combo or direct provider)
7. Deduct credits (async)
8. Log request (async)
9. Return response
```

**Acceptance Criteria:**
- [ ] validateKey checks hash, active, expiry
- [ ] checkPermissions validates model/provider whitelist
- [ ] checkRateLimit enforces RPM/TPM
- [ ] checkCredits validates sufficient balance
- [ ] routeRequest handles combos and direct models
- [ ] postProcess deducts credits async
- [ ] postProcess logs request async
- [ ] Returns proper HTTP status codes (401, 403, 429, 402, 500)
- [ ] Thread-safe (verified with -race)
- [ ] Unit tests with 95%+ coverage

---

### 2.6 Integration Tests

**File:** `internal/service/integration_test.go`

End-to-end service layer tests:

```go
func TestRequestOrchestrator_FullFlow(t *testing.T)
func TestRequestOrchestrator_RateLimitExceeded(t *testing.T)
func TestRequestOrchestrator_InsufficientCredits(t *testing.T)
func TestRequestOrchestrator_ModelNotAllowed(t *testing.T)
func TestRequestOrchestrator_ComboFallback(t *testing.T)
func TestRequestOrchestrator_ComboRoundRobin(t *testing.T)
func TestRequestOrchestrator_MultiAccountFallback(t *testing.T)
func TestRequestOrchestrator_CooldownBackoff(t *testing.T)
```

**Test Scenarios:**
- [ ] Successful request with credit deduction
- [ ] Rate limit exceeded (429)
- [ ] Insufficient credits (402)
- [ ] Model not allowed (403)
- [ ] Combo fallback (first fails, second succeeds)
- [ ] Combo round-robin (rotates starting model)
- [ ] Multi-account fallback (first account fails, second succeeds)
- [ ] Cooldown backoff (exponential backoff on repeated errors)

**Acceptance Criteria:**
- [ ] All scenarios pass
- [ ] Mock provider executors
- [ ] Real ConfigStore + LogStore (in-memory)
- [ ] Verify async operations complete
- [ ] Verify logs written correctly

---

## Dependencies

- Phase 1 (ConfigStore, LogStore)
- `golang.org/x/crypto/bcrypt` — key hashing
- `github.com/google/uuid` — request IDs

---

## Testing Strategy

### Unit Tests
- Mock dependencies (ConfigStore, LogStore)
- Test each service in isolation
- Race detector enabled
- 95%+ coverage per file

### Integration Tests
- Real ConfigStore + LogStore (in-memory)
- Mock provider executors
- Test full request flow
- Verify async operations

### Performance Tests
- 1000 concurrent requests
- Measure p99 latency (<50ms target)
- Verify no memory leaks (pprof)

---

## Deliverables

- [ ] RateLimiter with token bucket
- [ ] CreditManager with pricing table
- [ ] ProviderRouter with fallback
- [ ] ComboExecutor with strategies
- [ ] RequestOrchestrator with full flow
- [ ] 95%+ test coverage
- [ ] Integration tests
- [ ] Performance benchmarks
- [ ] Documentation (godoc)

---

## Estimated Effort

- RateLimiter: 6 hours
- CreditManager: 6 hours
- ProviderRouter: 8 hours
- ComboExecutor: 6 hours
- RequestOrchestrator: 10 hours
- Integration tests: 8 hours
- Performance tests: 4 hours
- Documentation: 2 hours

**Total:** 50 hours (1 week)
