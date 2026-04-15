---
status: pending
priority: high
effort: 1 week
phase: 5
dependencies: [phase-1, phase-2, phase-3, phase-4]
---

# Phase 5: Features & Polish (Week 5)

## Goals

Implement OAuth flows, Cloudflare tunnel integration, token refresh scheduler, file watcher, and conduct comprehensive performance testing.

## Tasks

### 5.1 OAuth: AWS Builder ID

**File:** `internal/auth/builderid.go`

Implement AWS Builder ID device code flow:

```go
type BuilderIDAuth struct {
    clientID     string
    region       string
    httpClient   *http.Client
}

func NewBuilderIDAuth() *BuilderIDAuth
func (a *BuilderIDAuth) StartDeviceFlow() (*DeviceCodeResponse, error)
func (a *BuilderIDAuth) PollToken(deviceCode string) (*TokenResponse, error)
func (a *BuilderIDAuth) RefreshToken(refreshToken string) (*TokenResponse, error)
```

**Flow:**
1. Request device code from AWS SSO
2. Show user verification URL + code
3. Poll for token (every 5s, max 5min)
4. Return access token + refresh token

**Endpoints:**
- Device code: `https://oidc.us-east-1.amazonaws.com/device_authorization`
- Token: `https://oidc.us-east-1.amazonaws.com/token`

**Acceptance Criteria:**
- [ ] Device code flow works
- [ ] Polling with exponential backoff
- [ ] Token refresh works
- [ ] Error handling (expired, denied)
- [ ] Unit tests with mock HTTP
- [ ] Integration test with real AWS

---

### 5.2 OAuth: AWS IAM Identity Center (IDC)

**File:** `internal/auth/idc.go`

Implement AWS IDC device code flow with custom startUrl/region:

```go
type IDCAuth struct {
    startURL   string
    region     string
    httpClient *http.Client
}

func NewIDCAuth(startURL, region string) *IDCAuth
func (a *IDCAuth) StartDeviceFlow() (*DeviceCodeResponse, error)
func (a *IDCAuth) PollToken(deviceCode string) (*TokenResponse, error)
func (a *IDCAuth) RefreshToken(refreshToken string) (*TokenResponse, error)
```

**Flow:** Same as Builder ID, but with custom startUrl/region

**Endpoints:**
- Device code: `https://oidc.{region}.amazonaws.com/device_authorization`
- Token: `https://oidc.{region}.amazonaws.com/token`

**Acceptance Criteria:**
- [ ] Device code flow with custom startUrl/region
- [ ] Polling with exponential backoff
- [ ] Token refresh works
- [ ] Error handling
- [ ] Unit tests with mock HTTP
- [ ] Integration test with real AWS IDC

---

### 5.3 OAuth: Social Login (Google, GitHub)

**File:** `internal/auth/social.go`

Implement PKCE flow for Google/GitHub:

```go
type SocialAuth struct {
    provider     string // "google", "github"
    clientID     string
    redirectURI  string
    httpClient   *http.Client
}

func NewSocialAuth(provider, clientID, redirectURI string) *SocialAuth
func (a *SocialAuth) GetAuthURL(state, codeVerifier string) string
func (a *SocialAuth) ExchangeCode(code, codeVerifier string) (*TokenResponse, error)
func (a *SocialAuth) RefreshToken(refreshToken string) (*TokenResponse, error)
func (a *SocialAuth) generateCodeChallenge(verifier string) string
```

**PKCE Flow:**
1. Generate code_verifier (random 43-128 chars)
2. Generate code_challenge (SHA256 base64url of verifier)
3. Redirect user to auth URL with challenge
4. User authorizes, gets redirected with code
5. Exchange code + verifier for tokens

**Providers:**
- Google: `https://accounts.google.com/o/oauth2/v2/auth`
- GitHub: `https://github.com/login/oauth/authorize`

**Acceptance Criteria:**
- [ ] PKCE code challenge generation
- [ ] Auth URL generation
- [ ] Code exchange works
- [ ] Token refresh works
- [ ] Support Google + GitHub
- [ ] Unit tests with mock HTTP
- [ ] Integration test with real providers

---

### 5.4 OAuth: Import Token

**File:** `internal/auth/import.go`

Implement manual token import with auto-register:

```go
type ImportAuth struct {
    httpClient *http.Client
}

func NewImportAuth() *ImportAuth
func (a *ImportAuth) ImportToken(refreshToken string) (*TokenResponse, error)
func (a *ImportAuth) DetectProvider(refreshToken string) (string, error)
func (a *ImportAuth) RefreshToken(refreshToken string) (*TokenResponse, error)
```

**Flow:**
1. User provides refresh token
2. Detect provider (try AWS, Google, GitHub)
3. Refresh token to validate
4. Return access token + refresh token

**Acceptance Criteria:**
- [ ] Import refresh token
- [ ] Auto-detect provider
- [ ] Validate token by refreshing
- [ ] Error handling (invalid token)
- [ ] Unit tests with mock HTTP
- [ ] Integration test with real tokens

---

### 5.5 Token Refresh Scheduler

**File:** `internal/service/token-refresh-scheduler.go`

Implement background token refresh:

```go
type TokenRefreshScheduler struct {
    configStore *storage.ConfigStore
    authClients map[string]TokenRefresher
    stopCh      chan struct{}
    wg          sync.WaitGroup
}

type TokenRefresher interface {
    RefreshToken(refreshToken string) (*TokenResponse, error)
}

func NewTokenRefreshScheduler(store *storage.ConfigStore) *TokenRefreshScheduler
func (s *TokenRefreshScheduler) RegisterRefresher(authType string, refresher TokenRefresher)
func (s *TokenRefreshScheduler) Start()
func (s *TokenRefreshScheduler) Stop()
func (s *TokenRefreshScheduler) refreshLoop()
func (s *TokenRefreshScheduler) refreshAccount(account *domain.ProviderAccount) error
```

**Key Features:**
- Check all accounts every 5 minutes
- Refresh tokens expiring in <5 minutes
- Update credentials in ConfigStore
- Log refresh success/failure
- Retry on failure (3 attempts)

**Acceptance Criteria:**
- [ ] Periodic check (every 5min)
- [ ] Refresh tokens expiring soon (<5min)
- [ ] Update credentials in ConfigStore
- [ ] Retry on failure (3 attempts)
- [ ] Log refresh events
- [ ] Graceful shutdown
- [ ] Unit tests with mock refreshers
- [ ] Integration test with real tokens

---

### 5.6 Cloudflare Tunnel Integration

**File:** `internal/tunnel/cloudflare.go`

Implement Cloudflare tunnel management:

```go
type CloudflareTunnel struct {
    binaryPath  string
    configPath  string
    statePath   string
    cmd         *exec.Cmd
    mu          sync.Mutex
}

func NewCloudfareTunnel(dataDir string) *CloudflareTunnel
func (t *CloudflareTunnel) Download() error
func (t *CloudflareTunnel) Start(port int) (string, error)
func (t *CloudflareTunnel) Stop() error
func (t *CloudflareTunnel) Status() (*TunnelStatus, error)
func (t *CloudflareTunnel) IsRunning() bool
func (t *CloudflareTunnel) GetURL() string
```

**Key Features:**
- Auto-download cloudflared binary (v2026.3.0)
- Cross-platform (Windows, macOS, Linux amd64/arm64)
- Persistent state (save URL to state.json)
- Auto-restart on server boot (if previously enabled)
- Graceful shutdown

**Binary URLs:**
- Windows: `https://github.com/cloudflare/cloudflared/releases/download/2026.3.0/cloudflared-windows-amd64.exe`
- macOS: `https://github.com/cloudflare/cloudflared/releases/download/2026.3.0/cloudflared-darwin-amd64`
- Linux: `https://github.com/cloudflare/cloudflared/releases/download/2026.3.0/cloudflared-linux-amd64`

**Acceptance Criteria:**
- [ ] Auto-download binary on first use
- [ ] Start tunnel with `cloudflared tunnel --url http://localhost:{port}`
- [ ] Parse tunnel URL from stdout
- [ ] Save state to state.json
- [ ] Stop tunnel gracefully
- [ ] Status check (running, URL)
- [ ] Auto-restart on server boot
- [ ] Unit tests with mock exec
- [ ] Integration test with real cloudflared

---

### 5.7 File Watcher Integration

**File:** `cmd/dntproxy/main.go` (extend)

Integrate file watcher with ConfigStore:

```go
func main() {
    // ... setup ...
    
    // Enable auto-reload
    if err := configStore.EnableAutoReload(); err != nil {
        log.Fatalf("Failed to enable auto-reload: %v", err)
    }
    defer configStore.Close()
    
    // ... start server ...
}
```

**Acceptance Criteria:**
- [ ] ConfigStore auto-reloads on file change
- [ ] Logs reload success/failure
- [ ] No downtime during reload
- [ ] Integration test with real file changes

---

### 5.8 Performance Testing

**File:** `test/performance/load_test.go`

Implement load testing with concurrent requests:

```go
func TestConcurrentRequests(t *testing.T)
func TestRateLimiting(t *testing.T)
func TestCreditDeduction(t *testing.T)
func TestProviderFallback(t *testing.T)
func TestComboExecution(t *testing.T)
```

**Test Scenarios:**
- 100 concurrent requests (measure p50, p95, p99 latency)
- 1000 requests with rate limiting (verify 429 responses)
- 100 requests with credit deduction (verify balances)
- 50 requests with provider fallback (verify account selection)
- 50 requests with combo execution (verify strategy)

**Performance Targets:**
- p50: <20ms
- p95: <40ms
- p99: <50ms
- Throughput: 100+ req/s

**Acceptance Criteria:**
- [ ] 100 concurrent requests without errors
- [ ] p99 latency <50ms
- [ ] Rate limiting works under load
- [ ] Credit deduction accurate under load
- [ ] Provider fallback works under load
- [ ] Combo execution works under load
- [ ] No memory leaks (verified with pprof)
- [ ] No race conditions (verified with -race)

---

### 5.9 Memory Profiling

**File:** `test/performance/memory_test.go`

Implement memory profiling:

```go
func TestMemoryUsage(t *testing.T)
func TestMemoryLeaks(t *testing.T)
func TestGoroutineLeaks(t *testing.T)
```

**Test Scenarios:**
- Measure baseline memory usage
- Run 10000 requests, measure memory growth
- Check for goroutine leaks
- Check for memory leaks (pprof heap)

**Acceptance Criteria:**
- [ ] Baseline memory <50MB
- [ ] Memory growth <100MB after 10000 requests
- [ ] No goroutine leaks
- [ ] No memory leaks (verified with pprof)

---

### 5.10 Stress Testing

**File:** `test/performance/stress_test.go`

Implement stress testing:

```go
func TestHighConcurrency(t *testing.T)
func TestLongRunning(t *testing.T)
func TestBurstTraffic(t *testing.T)
```

**Test Scenarios:**
- 500 concurrent requests (measure degradation)
- 1 hour continuous load (measure stability)
- Burst traffic (0 → 100 req/s → 0)

**Acceptance Criteria:**
- [ ] 500 concurrent requests without crashes
- [ ] 1 hour continuous load without degradation
- [ ] Burst traffic handled gracefully
- [ ] No memory leaks during long run
- [ ] No goroutine leaks during long run

---

### 5.11 CLI Commands

**File:** `cmd/dntproxy/cmd/auth.go`

Implement auth CLI commands:

```go
func authAddCmd() *cobra.Command
func authListCmd() *cobra.Command
func authRemoveCmd() *cobra.Command
func authTestCmd() *cobra.Command
func authRefreshCmd() *cobra.Command
```

**Commands:**
- `dntproxy auth add` — Interactive auth flow (choose method)
- `dntproxy auth list` — List all accounts
- `dntproxy auth remove <id>` — Remove account
- `dntproxy auth test <id>` — Test account
- `dntproxy auth refresh <id>` — Refresh token

**Acceptance Criteria:**
- [ ] Interactive auth flow (choose method)
- [ ] Builder ID flow
- [ ] IDC flow (prompt for startUrl/region)
- [ ] Social flow (prompt for provider)
- [ ] Import flow (prompt for refresh token)
- [ ] List accounts with status
- [ ] Remove account
- [ ] Test account (call provider)
- [ ] Refresh token

---

### 5.12 CLI: Tunnel Commands

**File:** `cmd/dntproxy/cmd/tunnel.go`

Implement tunnel CLI commands:

```go
func tunnelEnableCmd() *cobra.Command
func tunnelDisableCmd() *cobra.Command
func tunnelStatusCmd() *cobra.Command
```

**Commands:**
- `dntproxy tunnel enable` — Enable tunnel
- `dntproxy tunnel disable` — Disable tunnel
- `dntproxy tunnel status` — Show tunnel status

**Acceptance Criteria:**
- [ ] Enable tunnel (download binary, start, save state)
- [ ] Disable tunnel (stop, clear state)
- [ ] Status (show running, URL)
- [ ] Integration test with real cloudflared

---

## Dependencies

- Phase 1 (ConfigStore)
- Phase 2 (RequestOrchestrator)
- Phase 3 (Provider Registry)
- Phase 4 (HTTP Server)
- `golang.org/x/oauth2` — OAuth flows
- `github.com/spf13/cobra` — CLI
- `github.com/cloudflare/cloudflared` — Tunnel binary

---

## Testing Strategy

### Unit Tests
- Mock HTTP clients
- Mock exec commands
- Test each auth flow in isolation
- 95%+ coverage per file

### Integration Tests
- Real OAuth flows (with test accounts)
- Real tunnel (with cloudflared)
- Real file watcher (with temp files)

### Performance Tests
- Load testing (100+ concurrent)
- Memory profiling (pprof)
- Stress testing (500+ concurrent)
- Long-running tests (1 hour)

---

## Deliverables

- [ ] OAuth flows (Builder ID, IDC, Social, Import)
- [ ] Token refresh scheduler
- [ ] Cloudflare tunnel integration
- [ ] File watcher integration
- [ ] Performance testing (load, memory, stress)
- [ ] CLI commands (auth, tunnel)
- [ ] 95%+ test coverage
- [ ] Performance benchmarks
- [ ] Documentation (godoc + CLI help)

---

## Estimated Effort

- OAuth Builder ID: 6 hours
- OAuth IDC: 4 hours
- OAuth Social: 6 hours
- OAuth Import: 4 hours
- Token refresh scheduler: 6 hours
- Cloudflare tunnel: 8 hours
- File watcher integration: 2 hours
- Performance testing: 8 hours
- Memory profiling: 4 hours
- Stress testing: 4 hours
- CLI auth commands: 6 hours
- CLI tunnel commands: 3 hours
- Unit tests: 8 hours
- Integration tests: 6 hours
- Documentation: 3 hours

**Total:** 78 hours (2 weeks)
