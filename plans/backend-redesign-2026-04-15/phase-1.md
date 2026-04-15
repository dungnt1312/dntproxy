---
status: pending
priority: high
effort: 1 week
phase: 1
dependencies: []
---

# Phase 1: Core Foundation (Week 1)

## Goals

Build the foundational data models and storage layer with proper type safety, in-memory caching, and file persistence.

## Tasks

### 1.1 Domain Models

**File:** `internal/domain/models.go`

Create core domain types:

```go
type APIKey struct {
    ID              string
    Name            string
    KeyHash         string
    AllowedModels   []string
    AllowedProviders []string
    RPM             int
    TPM             int
    CreditBalance   float64
    CreditLimit     float64
    ExpiresAt       *time.Time
    IsActive        bool
    Tags            []string
    Notes           string
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type ProviderAccount struct {
    ID              string
    ProviderID      string
    Name            string
    AuthType        string
    Credentials     Credentials
    Priority        int
    IsActive        bool
    Status          string
    LastError       string
    LastErrorAt     *time.Time
    CooldownUntil   *time.Time
    BackoffLevel    int
    SupportedModels []string
    ModelLocks      map[string]time.Time
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type Credentials struct {
    AccessToken  string
    RefreshToken string
    APIKey       string
    ExpiresAt    *time.Time
    Extra        map[string]interface{}
}

type Combo struct {
    ID        string
    Name      string
    Models    []string
    Strategy  string
    IsActive  bool
    CreatedAt time.Time
    UpdatedAt time.Time
}

type ModelPricing struct {
    ID           string
    ProviderID   string
    ModelPattern string
    InputPrice   float64
    OutputPrice  float64
    Currency     string
}

type Config struct {
    APIKeys          []*APIKey
    ProviderAccounts []*ProviderAccount
    Combos           []*Combo
    ModelPricing     []*ModelPricing
    Settings         Settings
}

type Settings struct {
    Port            int
    RequireAPIKey   bool
    TunnelEnabled   bool
    TunnelURL       string
    TunnelProvider  string
}
```

**Acceptance Criteria:**
- [ ] All structs have proper JSON tags
- [ ] Pointer fields for optional values
- [ ] Time fields use time.Time
- [ ] No external dependencies
- [ ] Godoc comments for all types

---

### 1.2 ConfigStore (In-Memory + JSON)

**File:** `internal/storage/config-store.go`

Implement in-memory config store with JSON persistence:

```go
type ConfigStore struct {
    mu          sync.RWMutex
    config      *Config
    filePath    string
    
    // In-memory indexes
    keysByHash           map[string]*APIKey
    keysById             map[string]*APIKey
    accountsByProvider   map[string][]*ProviderAccount
    accountsById         map[string]*ProviderAccount
    combosByName         map[string]*Combo
    pricingByPattern     map[string]*ModelPricing
}

func NewConfigStore(filePath string) (*ConfigStore, error)
func (s *ConfigStore) Load() error
func (s *ConfigStore) Save() error
func (s *ConfigStore) GetKeyByHash(hash string) (*APIKey, bool)
func (s *ConfigStore) GetKeyByID(id string) (*APIKey, bool)
func (s *ConfigStore) GetHealthyAccounts(providerID string) []*ProviderAccount
func (s *ConfigStore) GetAccountByID(id string) (*ProviderAccount, bool)
func (s *ConfigStore) GetComboByName(name string) (*Combo, bool)
func (s *ConfigStore) UpdateAccountStatus(accountID, status string, err error)
func (s *ConfigStore) UpdateAccountCooldown(accountID string, until time.Time, backoffLevel int)
func (s *ConfigStore) UpdateKeyBalance(keyID string, newBalance float64)
func (s *ConfigStore) rebuildIndexes()
```

**Key Features:**
- RWMutex for concurrent reads
- Atomic writes (.tmp → rename)
- In-memory indexes for O(1) lookup
- Auto-rebuild indexes on load
- Thread-safe operations

**Acceptance Criteria:**
- [ ] Load/Save with atomic writes
- [ ] All indexes built correctly
- [ ] GetHealthyAccounts filters cooldown + inactive
- [ ] GetHealthyAccounts sorts by priority
- [ ] UpdateAccountStatus persists async
- [ ] UpdateKeyBalance persists async
- [ ] Thread-safe (verified with -race)
- [ ] Unit tests with 90%+ coverage

---

### 1.3 LogStore (SQLite)

**File:** `internal/storage/log-store.go`

Implement SQLite log store with batch writer:

```go
type LogStore struct {
    db       *sql.DB
    queue    chan *RequestLog
    stopCh   chan struct{}
    wg       sync.WaitGroup
}

type RequestLog struct {
    ID            string
    APIKeyID      string
    Model         string
    ComboName     string
    ProviderID    string
    AccountID     string
    ResolvedModel string
    StartedAt     time.Time
    DurationMs    int64
    InputTokens   int
    OutputTokens  int
    TotalTokens   int
    InputCost     float64
    OutputCost    float64
    TotalCost     float64
    StatusCode    int
    Error         string
    RequestID     string
    UserAgent     string
}

func NewLogStore(dbPath string) (*LogStore, error)
func (s *LogStore) Start()
func (s *LogStore) Stop()
func (s *LogStore) Log(log *RequestLog)
func (s *LogStore) Query(filter LogFilter) ([]*RequestLog, error)
func (s *LogStore) CleanOldLogs(retentionDays int) error
```

**Key Features:**
- Buffered channel (1000 capacity)
- Batch insert (100 logs or 1s flush)
- Single transaction per batch
- Backpressure: drop logs if queue full
- Auto-cleanup (30-day retention)

**Schema:**
```sql
CREATE TABLE request_logs (
    id              TEXT PRIMARY KEY,
    api_key_id      TEXT NOT NULL,
    model           TEXT NOT NULL,
    combo_name      TEXT,
    provider_id     TEXT,
    account_id      TEXT,
    resolved_model  TEXT,
    started_at      INTEGER NOT NULL,
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

**Acceptance Criteria:**
- [ ] Schema created on init
- [ ] Batch writer with 100 logs or 1s flush
- [ ] Backpressure drops logs if queue full
- [ ] Query with filters (api_key_id, date range)
- [ ] CleanOldLogs removes logs older than N days
- [ ] Graceful shutdown (flush queue)
- [ ] Unit tests with 90%+ coverage

---

### 1.4 File Watcher

**File:** `internal/storage/file-watcher.go`

Implement config file watcher with fsnotify:

```go
type FileWatcher struct {
    watcher   *fsnotify.Watcher
    filePath  string
    onChange  func()
    stopCh    chan struct{}
    wg        sync.WaitGroup
}

func NewFileWatcher(filePath string, onChange func()) (*FileWatcher, error)
func (w *FileWatcher) Start() error
func (w *FileWatcher) Stop()
```

**Key Features:**
- Watch config.json for changes
- Debounce (100ms) to avoid multiple reloads
- Call onChange callback on file write
- Graceful shutdown

**Acceptance Criteria:**
- [ ] Detects file write events
- [ ] Debounces multiple events
- [ ] Calls onChange callback
- [ ] Handles file deletion gracefully
- [ ] Graceful shutdown
- [ ] Unit tests with mock filesystem

---

### 1.5 Integration: ConfigStore + FileWatcher

**File:** `internal/storage/config-store.go` (extend)

Add auto-reload to ConfigStore:

```go
func (s *ConfigStore) EnableAutoReload() error {
    watcher, err := NewFileWatcher(s.filePath, func() {
        if err := s.Load(); err != nil {
            log.Printf("Failed to reload config: %v", err)
        } else {
            log.Println("Config reloaded successfully")
        }
    })
    if err != nil {
        return err
    }
    
    s.watcher = watcher
    return watcher.Start()
}

func (s *ConfigStore) Close() error {
    if s.watcher != nil {
        s.watcher.Stop()
    }
    return nil
}
```

**Acceptance Criteria:**
- [ ] Auto-reload on config.json change
- [ ] Logs reload success/failure
- [ ] Graceful shutdown
- [ ] Integration test with real file

---

### 1.6 Unit Tests

**Files:**
- `internal/domain/models_test.go`
- `internal/storage/config-store_test.go`
- `internal/storage/log-store_test.go`
- `internal/storage/file-watcher_test.go`

**Test Coverage:**
- [ ] ConfigStore: Load, Save, all getters, updates
- [ ] ConfigStore: Index rebuilding
- [ ] ConfigStore: Concurrent reads/writes (-race)
- [ ] LogStore: Batch insert, query, cleanup
- [ ] LogStore: Backpressure (queue full)
- [ ] FileWatcher: Detect changes, debounce
- [ ] Integration: ConfigStore + FileWatcher

**Target:** 90%+ coverage

---

## Dependencies

- `github.com/fsnotify/fsnotify` — file watcher
- `modernc.org/sqlite` — pure Go SQLite
- `golang.org/x/crypto/bcrypt` — key hashing

---

## Testing Strategy

### Unit Tests
- Mock filesystem for FileWatcher
- In-memory SQLite for LogStore
- Temp files for ConfigStore
- Race detector enabled

### Integration Tests
- Real file operations
- ConfigStore + FileWatcher
- LogStore batch writer

### Performance Tests
- 1000 concurrent reads (ConfigStore)
- 10000 log writes (LogStore)
- Measure p99 latency

---

## Deliverables

- [ ] Domain models with godoc
- [ ] ConfigStore with in-memory indexes
- [ ] LogStore with batch writer
- [ ] FileWatcher with debounce
- [ ] 90%+ test coverage
- [ ] Performance benchmarks
- [ ] Documentation (README.md)

---

## Estimated Effort

- Domain models: 4 hours
- ConfigStore: 8 hours
- LogStore: 8 hours
- FileWatcher: 4 hours
- Integration: 4 hours
- Unit tests: 12 hours
- Documentation: 2 hours

**Total:** 42 hours (1 week)
