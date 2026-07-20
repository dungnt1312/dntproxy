# Core Reliability P0+P1 (Learn from CLIProxyAPI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring dntproxy core routing/reliability closer to CLIProxyAPI where it matters—retry taxonomy, credential retry budget, configurable cooldown, fill-first selection, session affinity, prepare/hooks/thinking/executor options—without importing cliproxy’s protocol matrix, plugins, or multi-storage platform.

**Architecture:** Keep clean ports/adapters. Put pure policy in `internal/domain`, selection/retry orchestration in `internal/service`, optional interfaces in `internal/port`, provider-local prepare/thinking in adapters. Extend existing `CheckFallbackError` / `AccountSelector` / `executeOnProvider` rather than replacing them. P0 ships a working multi-account reliability upgrade; P1 builds protocol-depth and extension points on top.

**Tech Stack:** Go 1.25+, existing `domain`/`port`/`service`/`adapter`, Gin handlers, JSON settings in `db.json`, table-driven unit tests (`go test`).

## Global Constraints

- File naming stays kebab-case; keep new files focused (~200 lines where practical).
- Domain types stay free of external deps.
- Do **not** break: combo strategies, `@connectionId` pin, API key allowlists, tenant filtering, existing OAuth refresh-on-401.
- Do **not** import `temp/CLIProxyAPI` into go.mod; rewrite by behavior only.
- Defaults must preserve current behavior when new settings are zero/omitted (backward compatible JSON).
- Out of scope: WebSocket transport, video APIs, plugin C-ABI, Postgres/git stores, full Claude↔Gemini translator matrix.
- Execute **Phase P0 (Tasks 1–7)** fully before **Phase P1 (Tasks 8–14)** unless a task explicitly only depends on earlier P0 outputs.

## Scope map (from gap list)

| Phase | Gap | Task(s) |
|---|---|---|
| P0 | Retryable status taxonomy | Task 1 |
| P0 | Routing settings types + defaults | Task 2 |
| P0 | Credential retry budget | Task 3 |
| P0 | Configurable cooldown + disableCooling | Task 4 |
| P0 | Fill-first strategy | Task 5 |
| P0 | Session affinity | Task 6 |
| P0 | Wire settings + regression | Task 7 |
| P1 | Unified prepare helper pattern | Task 8 |
| P1 | Thinking suffix canonical | Task 9 |
| P1 | Extended ExecuteOptions | Task 10 |
| P1 | Execution hooks | Task 11 |
| P1 | Reasoning cache generalization (thin) | Task 12 |
| P1 | Quota-aware model fallback map | Task 13 |
| P1 | Docs + full regression | Task 14 |

### Locked decisions (self-review)

1. **Priority sort:** lower `Priority` value wins (matches existing `priorityFallbackSelect` in `account-selector.go`).
2. **`MaxRetryCredentials`:** max **distinct connections attempted** per model (including the successful one if any). `0` = unlimited. OAuth re-exec on same connection after refresh does **not** increment the counter.
3. **Taxonomy vs legacy `CheckFallbackError`:** body hint `"improperly formed request"` is **non-fallback** (align with current `shouldFallbackToNextAccount`, not the older cooldown branch in `CheckFallbackError`). Cooldown helper must follow classifier so the two stop disagreeing.
4. **`RequestMetadata`:** add `SessionKey string` and `APIKeyID string` in Task 6 (fields today: `Compression`, `TenantID` only).
5. **`disableCooling`:** read only from `ProviderSpecificData["disableCooling"]` bool — do not add a top-level connection struct field in P0.
6. **Task order:** settings (Task 2) before retry budget (Task 3).

## File Structure

### Create
- `internal/domain/upstream-class.go` — unified classification of upstream failures
- `internal/domain/upstream-class_test.go`
- `internal/domain/routing-settings.go` — nested routing settings types + Normalize
- `internal/domain/routing-settings_test.go`
- `internal/service/session-affinity.go` — in-memory sticky connection map
- `internal/service/session-affinity_test.go`
- `internal/service/execution-hooks.go` — hook registry (P1)
- `internal/service/execution-hooks_test.go`
- `internal/port/execute-options.go` — ExecuteOptions + ExecuteResult + ExtendedExecutor (P1)
- `internal/service/thinking/suffix.go` — parse thinking suffixes (P1)
- `internal/service/thinking/suffix_test.go`
- `internal/service/thinking/apply.go` — apply effort to body by provider hint (P1)
- `internal/service/thinking/apply_test.go`
- `internal/adapter/shared/reasoning-cache.go` — provider-keyed thin cache facade (P1)
- `internal/adapter/shared/reasoning-cache_test.go`

### Modify
- `internal/domain/config.go` — embed routing settings fields on `Settings`
- `internal/domain/fallback.go` — delegate classification to upstream-class (keep exported helpers)
- `internal/domain/provider.go` — optional `DisableCooling` field OR read from `ProviderSpecificData` (prefer PSD key first to avoid schema churn; Task 4 documents choice)
- `internal/service/account-selector.go` — fill-first, disableCooling, affinity-aware select hooks
- `internal/service/chat-service.go` — retry budget, affinity key, hooks, prepare/options type-assert
- `internal/service/chat-service-error-routing.go` — use unified classifier
- `internal/service/combo-handler.go` — only if needed for logging retry reasons (prefer no change)
- `internal/adapter/http/settings-handler.go` — accept/return new settings fields (pass-through JSON if already generic)
- `internal/adapter/xai/executor.go` — adopt ExtendedExecutor / thinking apply when P1 lands (optional type assert path)

### Reference (behavior only)
- CLIProxyAPI `config.example.yaml` routing/retry/cooldown keys
- Existing dntproxy `domain.CheckFallbackError`, `executeOnProvider` retry loop

---

# Phase P0 — Reliability core

### Task 1: Unified upstream failure taxonomy

**Files:**
- Create: `internal/domain/upstream-class.go`
- Create: `internal/domain/upstream-class_test.go`
- Modify: `internal/domain/fallback.go` (thin wrapper keeping `CheckFallbackError` API)
- Modify: `internal/service/chat-service-error-routing.go`

**Interfaces:**
- Consumes: status code + error text + current backoff level
- Produces:
  - `type UpstreamClass string` with values: `non_fallback`, `retryable`, `quota`, `auth`, `transient`, `model_entitlement`
  - `func ClassifyUpstream(status int, errorText string) UpstreamClass`
  - `func IsRetryableUpstream(status int, errorText string) bool`
  - `func IsQuotaExceeded(status int, errorText string) bool`
  - Existing `CheckFallbackError` remains and uses classifier for cooldown decisions

- [ ] **Step 1: Write failing tests**

```go
package domain

import "testing"

func TestClassifyUpstream(t *testing.T) {
	tests := []struct {
		status int
		body   string
		want   UpstreamClass
	}{
		{400, "invalid request", UpstreamNonFallback},
		{400, "improperly formed request", UpstreamNonFallback},
		{429, "rate limit exceeded", UpstreamQuota},
		// Body "overloaded" is a quota keyword and wins over bare status class.
		{503, "overloaded", UpstreamQuota},
		{503, "", UpstreamTransient},
		{401, "unauthorized", UpstreamAuth},
		{403, "model_not_entitled", UpstreamModelEntitlement},
		{500, "internal", UpstreamRetryable},
		{408, "timeout", UpstreamRetryable},
	}
	for _, tt := range tests {
		if got := ClassifyUpstream(tt.status, tt.body); got != tt.want {
			t.Fatalf("status=%d body=%q got=%s want=%s", tt.status, tt.body, got, tt.want)
		}
	}
}

func TestIsRetryableUpstream(t *testing.T) {
	if !IsRetryableUpstream(503, "") {
		t.Fatal("503 should be retryable")
	}
	if IsRetryableUpstream(400, "invalid json") {
		t.Fatal("400 invalid should not be retryable")
	}
	if !IsRetryableUpstream(429, "quota exceeded") {
		t.Fatal("quota should still allow account/model failover")
	}
}
```

- [ ] **Step 2: Run tests (expect FAIL)**

Run: `go test ./internal/domain -run "TestClassifyUpstream|TestIsRetryableUpstream" -count=1`

Expected: FAIL undefined symbols

- [ ] **Step 3: Implement `upstream-class.go`**

```go
package domain

import "strings"

type UpstreamClass string

const (
	UpstreamNonFallback      UpstreamClass = "non_fallback"
	UpstreamRetryable        UpstreamClass = "retryable"
	UpstreamQuota            UpstreamClass = "quota"
	UpstreamAuth             UpstreamClass = "auth"
	UpstreamTransient        UpstreamClass = "transient"
	UpstreamModelEntitlement UpstreamClass = "model_entitlement"
)

func ClassifyUpstream(status int, errorText string) UpstreamClass {
	lower := strings.ToLower(errorText)
	if IsNonFallbackStatus(status) {
		return UpstreamNonFallback
	}
	// client-error body hints (mirror chat-service-error-routing)
	for _, hint := range []string{"invalid request", "improperly formed request", "malformed", "invalid json", "missing required", "unsupported parameter", "tool schema"} {
		if strings.Contains(lower, hint) {
			return UpstreamNonFallback
		}
	}
	if status == 403 && isModelEntitlementError(lower) {
		return UpstreamModelEntitlement
	}
	for _, kw := range []string{"rate limit", "too many requests", "quota exceeded", "capacity", "overloaded"} {
		if strings.Contains(lower, kw) {
			return UpstreamQuota
		}
	}
	switch status {
	case 401:
		return UpstreamAuth
	case 429:
		return UpstreamQuota
	case 402, 403:
		return UpstreamQuota
	case 408, 500, 502, 503, 504:
		return UpstreamTransient
	case 404:
		// Keep legacy CheckFallbackError behavior: still allow account/model failover.
		return UpstreamRetryable
	default:
		if status >= 500 {
			return UpstreamTransient
		}
		// Unknown non-4xx-client statuses remain retryable for multi-account HA.
		return UpstreamRetryable
	}
}

// Note for CheckFallbackError rewrite: remove the special-case that treated
// "improperly formed request" as transient cooldown. Classifier marks it
// non_fallback; CheckFallbackError must return ShouldFallback=false, CooldownMs=0.

func IsRetryableUpstream(status int, errorText string) bool {
	switch ClassifyUpstream(status, errorText) {
	case UpstreamNonFallback:
		return false
	default:
		return true
	}
}

func IsQuotaExceeded(status int, errorText string) bool {
	return ClassifyUpstream(status, errorText) == UpstreamQuota ||
		ClassifyUpstream(status, errorText) == UpstreamModelEntitlement
}
```

Update `CheckFallbackError` to branch on `ClassifyUpstream` while preserving cooldown numbers already in `fallback.go`.

Update `shouldFallbackToNextAccount` to:

```go
func shouldFallbackToNextAccount(status int, errorText string) bool {
	return domain.IsRetryableUpstream(status, errorText)
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/domain -count=1
go test ./internal/service -run "Fallback|Error|Chat" -count=1
```

Expected: PASS (fix any tests that assumed old private hint lists)

- [ ] **Step 5: Commit**

```bash
git add internal/domain/upstream-class.go internal/domain/upstream-class_test.go internal/domain/fallback.go internal/service/chat-service-error-routing.go
git commit -m "$(cat <<'EOF'
feat(domain): unify upstream failure classification for retries

EOF
)"
```

---

### Task 2: Routing settings types + defaults

**Files:**
- Create: `internal/domain/routing-settings.go`
- Create: `internal/domain/routing-settings_test.go`
- Modify: `internal/domain/config.go` — add fields on `Settings`; call normalize from `DefaultConfig`
- Modify: `internal/adapter/storage/json-db.go` — after unmarshal / in `GetSettings`, call `settings.NormalizeRouting()` so zero JSON still gets safe TTL defaults when affinity enabled

**Interfaces / fields on `Settings`:**

```go
// MaxRetryCredentials caps distinct connections attempted per model.
// 0 means unlimited (legacy behavior).
MaxRetryCredentials int `json:"maxRetryCredentials,omitempty"`

// CooldownEnabled nil/true = on; pointer so JSON omit keeps default-on.
CooldownEnabled *bool `json:"cooldownEnabled,omitempty"`
// TransientCooldownSeconds overrides CooldownTransient when > 0.
TransientCooldownSeconds int `json:"transientCooldownSeconds,omitempty"`
// MaxCooldownSeconds clamps any computed cooldown when > 0.
MaxCooldownSeconds int `json:"maxCooldownSeconds,omitempty"`
// ModelLockEnabled nil/true = on.
ModelLockEnabled *bool `json:"modelLockEnabled,omitempty"`

SessionAffinityEnabled    bool `json:"sessionAffinityEnabled,omitempty"`
SessionAffinityTTLSeconds int  `json:"sessionAffinityTTLSeconds,omitempty"`

// QuotaModelFallback used in P1 Task 13; add field now so JSON shape is stable.
QuotaModelFallback map[string]string `json:"quotaModelFallback,omitempty"`
```

Also document allowed `ConnectionStrategy` values: `weighted-random`, `priority-fallback`, `round-robin`, `fill-first`.

```go
func (s *Settings) CooldownOn() bool {
	return s.CooldownEnabled == nil || *s.CooldownEnabled
}
func (s *Settings) ModelLockOn() bool {
	return s.ModelLockEnabled == nil || *s.ModelLockEnabled
}
func (s *Settings) NormalizeRouting() {
	if s.SessionAffinityEnabled && s.SessionAffinityTTLSeconds <= 0 {
		s.SessionAffinityTTLSeconds = 1800
	}
	if s.MaxCooldownSeconds < 0 {
		s.MaxCooldownSeconds = 0
	}
	if s.MaxRetryCredentials < 0 {
		s.MaxRetryCredentials = 0
	}
}
```

- [ ] **Step 1: Tests for defaults/normalize**

```go
func TestSettingsNormalizeRoutingDefaults(t *testing.T) {
	s := Settings{SessionAffinityEnabled: true}
	s.NormalizeRouting()
	if s.SessionAffinityTTLSeconds != 1800 {
		t.Fatalf("ttl=%d", s.SessionAffinityTTLSeconds)
	}
	if !s.CooldownOn() || !s.ModelLockOn() {
		t.Fatal("cooldown/model lock should default on")
	}
}

func TestSettingsMaxRetryCredentialsNegativeNormalized(t *testing.T) {
	s := Settings{MaxRetryCredentials: -3}
	s.NormalizeRouting()
	if s.MaxRetryCredentials != 0 {
		t.Fatalf("got %d", s.MaxRetryCredentials)
	}
}
```

- [ ] **Step 2: Run FAIL**

Run: `go test ./internal/domain -run "TestSettingsNormalize|TestSettingsMaxRetry" -count=1`

- [ ] **Step 3: Implement fields + helpers; wire NormalizeRouting in DefaultConfig and GetSettings path**

- [ ] **Step 4: PASS**

Run: `go test ./internal/domain -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/domain/config.go internal/domain/routing-settings.go internal/domain/routing-settings_test.go internal/adapter/storage/json-db.go
git commit -m "$(cat <<'EOF'
feat(domain): add routing reliability settings with backward-compatible defaults

EOF
)"
```

---

### Task 3: Credential retry budget in `executeOnProvider`

**Files:**
- Modify: `internal/service/chat-service.go`
- Create: `internal/service/chat-service-retry_test.go`

**Interfaces:**
- Consumes: `Settings.MaxRetryCredentials` from Task 2
- Helper: `func shouldStopCredentialRetry(attempted int, max int) bool`
- Semantics (locked):
  - `max == 0` → never stop for budget (unlimited)
  - `max == N` → stop after **N distinct connections attempted**
  - Increment `attempted` once per selected connection **after** select succeeds
  - OAuth refresh + second `Execute` on the **same** connection does not increment again
  - Pinned `@connectionId` still single attempt (budget irrelevant)
  - Non-retryable error returns immediately without consuming remaining budget on other conns
  - When budget exhausted: `AllowFallback: true` so combo can move to next model

```go
func shouldStopCredentialRetry(attempted int, max int) bool {
	return max > 0 && attempted >= max
}
```

- [ ] **Step 1: Write failing tests**

```go
func TestShouldStopCredentialRetry(t *testing.T) {
	if shouldStopCredentialRetry(1, 0) {
		t.Fatal("0 means unlimited")
	}
	if shouldStopCredentialRetry(1, 2) {
		t.Fatal("should continue")
	}
	if !shouldStopCredentialRetry(2, 2) {
		t.Fatal("should stop at max")
	}
}

func TestExecuteOnProviderRespectsMaxRetryCredentials(t *testing.T) {
	// Use NewChatServiceWithDeps + fake store/executor from chat-service-test-helpers_test.go.
	// Settings.MaxRetryCredentials = 2
	// Three connections conn-a, conn-b, conn-c for provider under test
	// Executor returns 503 for every connection
	// Assert: exactly 2 MarkUnavailable (or 2 Execute calls), result AllowFallback=true,
	// status 503/503-family or 503 mapped, and conn-c never selected.
}
```

If the full harness cannot assert selection order easily, still require:
1. helper unit test above (mandatory)
2. one integration test that with `max=1` and two failing conns, only one Execute is observed

- [ ] **Step 2: Run FAIL**

Run: `go test ./internal/service -run "TestShouldStopCredentialRetry|TestExecuteOnProviderRespectsMaxRetryCredentials" -count=1`

- [ ] **Step 3: Implement in `executeOnProvider` loop**

```go
attempted := 0
for {
	creds, err := s.accountSelector.SelectCredentialsForModel(...)
	// map selection errors...
	attempted++

	// execute once; on 401+refreshToken, re-Execute same creds without attempted++

	if success { ... }

	if !shouldFallbackToNextAccount(status, errMsg) {
		// end + return non-fallback
	}
	_ = s.accountSelector.MarkUnavailable(creds.ConnectionID, status, errMsg, model)
	excludeIDs[creds.ConnectionID] = true
	if parsed.ConnectionID != "" {
		// pinned fail return
	}
	max := 0
	if settings, err := s.store.GetSettings(); err == nil && settings != nil {
		max = settings.MaxRetryCredentials
	}
	if shouldStopCredentialRetry(attempted, max) {
		msg := fmt.Sprintf("credential retry budget exhausted (%d)", max)
		reqlog.End(http.StatusServiceUnavailable, msg)
		return &ComboResult{OK: false, StatusCode: http.StatusServiceUnavailable, Error: msg, AllowFallback: true}, nil
	}
}
```

- [ ] **Step 4: PASS**

Run: `go test ./internal/service -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/service/chat-service.go internal/service/chat-service-retry_test.go
git commit -m "$(cat <<'EOF'
feat(service): cap per-model credential retry budget

EOF
)"
```

---

### Task 4: Configurable cooldown + per-connection disableCooling

**Files:**
- Modify: `internal/service/account-selector.go` (`MarkUnavailable`)
- Create: `internal/service/account-selector-cooldown_test.go`

**Behavior (locked):**
1. If `!settings.CooldownOn()` → `MarkUnavailable` returns nil without writing `RateLimitedUntil` / backoff / model locks (in-request `excludeIDs` still work)
2. If `ProviderSpecificData["disableCooling"] == true` → same no-persist behavior for that connection only
3. If `TransientCooldownSeconds > 0` and class is `UpstreamTransient` → `CooldownMs = TransientCooldownSeconds * 1000` (override legacy 2000ms)
4. If `MaxCooldownSeconds > 0` → `CooldownMs = min(CooldownMs, MaxCooldownSeconds*1000)`
5. If `!settings.ModelLockOn()` → never write `ModelLocks` even when `ModelOnly` would apply
6. Still update `LastError` / `LastErrorAt` when persisting a real cooldown (keep ops visibility); when cooling disabled, still may set last error if existing code already does—match current `MarkUnavailable` fields except cooldown/lock

```go
func connectionDisableCooling(conn *domain.ProviderConnection) bool {
	if conn == nil || conn.ProviderSpecificData == nil {
		return false
	}
	v, ok := conn.ProviderSpecificData["disableCooling"].(bool)
	return ok && v
}

func clampCooldownMs(ms int, maxSeconds int) int {
	if maxSeconds <= 0 || ms <= 0 {
		return ms
	}
	maxMs := maxSeconds * 1000
	if ms > maxMs {
		return maxMs
	}
	return ms
}
```

- [ ] **Step 1: Failing tests**

```go
func TestMarkUnavailableHonorsDisableCooling(t *testing.T) {
	// store with one conn, PSD disableCooling=true, backoffLevel=0
	// MarkUnavailable(id, 429, "rate limit", "gpt")
	// reload conn: RateLimitedUntil == "" and ModelLocks empty
}

func TestMarkUnavailableHonorsCooldownDisabled(t *testing.T) {
	// settings.CooldownEnabled = boolPtr(false)
	// MarkUnavailable 503
	// RateLimitedUntil remains ""
}

func TestMarkUnavailableClampsMaxCooldown(t *testing.T) {
	// settings.MaxCooldownSeconds = 1
	// MarkUnavailable 429 "quota exceeded" with high backoff level
	// parse RateLimitedUntil; must be <= now+1s+slack (250ms)
}

func TestMarkUnavailableTransientOverrideSeconds(t *testing.T) {
	// settings.TransientCooldownSeconds = 5
	// MarkUnavailable 503 ""
	// cooldown duration ~= 5s (not 2s legacy)
}
```

- [ ] **Step 2: Run FAIL**

Run: `go test ./internal/service -run TestMarkUnavailable -count=1`

- [ ] **Step 3: Implement in `MarkUnavailable`**

Order inside MarkUnavailable after loading conn:
1. Load settings; if `!CooldownOn()` or `connectionDisableCooling(conn)` → optional last-error only / return nil
2. `result := domain.CheckFallbackError(status, errorText, conn.BackoffLevel)`
3. If class transient and TransientCooldownSeconds > 0 → replace CooldownMs
4. Clamp with MaxCooldownSeconds
5. Apply RateLimitedUntil / backoff / model lock using ModelLockOn()

- [ ] **Step 4: PASS**

Run: `go test ./internal/service -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/service/account-selector.go internal/service/account-selector-cooldown_test.go
git commit -m "$(cat <<'EOF'
feat(service): honor cooldown settings and per-connection disableCooling

EOF
)"
```

---

### Task 5: Fill-first connection strategy

**Files:**
- Modify: `internal/service/account-selector.go`
- Create: `internal/service/account-selector-strategy_test.go`

**Interfaces:**
- Add `ConnectionStrategyFillFirst = "fill-first"`
- Include it in `connectionStrategy()` allowed switch (today only weighted/priority/rr)
- **Priority semantics locked:** lower `Priority` wins (same as `priorityFallbackSelect`)
- Sort: Priority ASC → Weight DESC (via `normalizedWeight`) → ID ASC for stability
- On success, **do not** call rotation advance for fill-first (`AdvanceConnectionRotation` already no-ops unless RR; keep that)
- Fill-first equals “always pick current best available candidate” — sticky as long as that candidate remains available

```go
const ConnectionStrategyFillFirst = "fill-first"

func fillFirstSelect(available []domain.ProviderConnection) *domain.ProviderConnection {
	if len(available) == 1 {
		return &available[0]
	}
	sorted := append([]domain.ProviderConnection(nil), available...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority < sorted[j].Priority
		}
		if normalizedWeight(sorted[i].Weight) != normalizedWeight(sorted[j].Weight) {
			return normalizedWeight(sorted[i].Weight) > normalizedWeight(sorted[j].Weight)
		}
		return sorted[i].ID < sorted[j].ID
	})
	return &sorted[0]
}
```

Wire in the same place priority/rr branches are chosen inside `SelectCredentials`.

- [ ] **Step 1: Failing tests**

```go
func TestFillFirstSelectsLowestPriority(t *testing.T) {
	// settings connectionStrategy=fill-first
	// conns: id=a Priority=2, id=b Priority=1, id=c Priority=3; all active
	// SelectCredentials 20 times → always b
}

func TestFillFirstSkipsExcludedAndCooldown(t *testing.T) {
	// exclude b; expect a (Priority 2) next
}
```

- [ ] **Step 2: Run FAIL**

Run: `go test ./internal/service -run TestFillFirst -count=1`

- [ ] **Step 3: Implement constant + branch + helper**

- [ ] **Step 4: PASS**

Run: `go test ./internal/service -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/service/account-selector.go internal/service/account-selector-strategy_test.go
git commit -m "$(cat <<'EOF'
feat(service): add fill-first connection selection strategy

EOF
)"
```

---

### Task 6: Session affinity (sticky connection)

**Files:**
- Create: `internal/service/session-affinity.go`
- Create: `internal/service/session-affinity_test.go`
- Modify: `internal/port/chat-service.go` — extend `RequestMetadata`
- Modify: `internal/service/chat-service.go` — soft-pin + Put on success
- Modify: `internal/adapter/http/chat-handler.go` — populate metadata SessionKey/APIKeyID from request

**Interfaces:**

```go
// port.RequestMetadata (extend existing struct; keep Compression + TenantID)
type RequestMetadata struct {
	Compression *domain.CompressionLogMetadata
	TenantID    string
	SessionKey  string // raw client session header value (optional)
	APIKeyID    string // authenticated API key id if any
}

// service/session-affinity.go
type SessionAffinityStore struct {
	mu      sync.Mutex
	entries map[string]affinityEntry
}
type affinityEntry struct {
	ConnectionID string
	ExpiresAt    time.Time
}

func NewSessionAffinityStore() *SessionAffinityStore
func (s *SessionAffinityStore) Get(key string) (connectionID string, ok bool)
func (s *SessionAffinityStore) Put(key, connectionID string, ttl time.Duration)
func (s *SessionAffinityStore) Delete(key string)

// AffinityKey builds storage key; empty means affinity off for this request.
// headerSession non-empty wins; else apiKeyID+provider+model; else "".
func AffinityKey(apiKeyID, provider, model, headerSession string) string
```

Header names (chat-handler): read first non-empty of `X-Session-Id`, `X-Dntproxy-Session`.

Wire in `executeOnProvider` (only when settings.SessionAffinityEnabled):
1. Build key via `AffinityKey(meta.APIKeyID, provider, model, meta.SessionKey)`
2. If key != "" and hard pin empty: `Get(key)` → if hit, try that connection first by calling pinned select path OR by pre-seeding exclude logic:
   - Preferred: attempt `SelectCredentialsForModel` with a temporary qualified model `provider/model@stickyID` only for the first try; on retryable fail `Delete(key)` and continue normal loop (sticky id stays in excludeIDs)
3. On success: `Put(key, creds.ConnectionID, ttl)` with ttl from settings (after NormalizeRouting)
4. Hard `@connectionId` pin always wins; do not rewrite pin

- [ ] **Step 1: Unit tests for store TTL + key builder**

```go
func TestSessionAffinityStoreTTL(t *testing.T) {
	s := NewSessionAffinityStore()
	s.Put("k", "c1", 40*time.Millisecond)
	if id, ok := s.Get("k"); !ok || id != "c1" {
		t.Fatalf("got %s %v", id, ok)
	}
	time.Sleep(50 * time.Millisecond)
	if _, ok := s.Get("k"); ok {
		t.Fatal("expected expire")
	}
}

func TestAffinityKeyPrecedence(t *testing.T) {
	if AffinityKey("ak", "openai", "gpt", "sess") != "sess" && !strings.Contains(AffinityKey("ak", "openai", "gpt", "sess"), "sess") {
		// Exact format: prefer key == "hdr:sess" or raw "sess"; pick one and lock it:
	}
	// Locked format:
	// header -> "h:"+headerSession
	// else api key -> "k:"+apiKeyID+"|"+provider+"|"+model
	// else ""
}
```

Lock key formats:
- header present → `"h:" + headerSession`
- else apiKeyID present → `"k:" + apiKeyID + "|" + provider + "|" + model`
- else `""`

- [ ] **Step 2: Run FAIL**

Run: `go test ./internal/service -run "TestSessionAffinity|TestAffinityKey" -count=1`

- [ ] **Step 3: Implement store, metadata fields, chat-handler header read, executeOnProvider soft-pin**

- [ ] **Step 4: PASS**

Run:
```bash
go test ./internal/service -count=1
go test ./internal/adapter/http -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/service/session-affinity.go internal/service/session-affinity_test.go internal/service/chat-service.go internal/port/chat-service.go internal/adapter/http/chat-handler.go
git commit -m "$(cat <<'EOF'
feat(service): add optional session affinity for sticky connections

EOF
)"
```

---

### Task 7: P0 regression gate + settings surface

**Files:**
- Modify: `docs/project-changelog.md` (short P0 note)
- Modify: UI settings only if project already has connection strategy dropdown—add `fill-first` option and optional advanced fields **only if low-cost**; otherwise API-only JSON settings is enough for P0
- Tests: full service/domain

- [ ] **Step 1: Run regression**

```bash
go test ./internal/domain -count=1
go test ./internal/service -count=1
go test ./internal/adapter/http -count=1
go build -o dntproxy ./cmd/dntproxy/
```

Expected: all PASS

- [ ] **Step 2: Manual checklist (document in commit body / Task 7 report)**
- Default settings → behavior identical to pre-P0 on happy path
- `connectionStrategy=fill-first` always picks lowest Priority available
- `maxRetryCredentials=1` with 2 failing conns → only 1 Execute, combo can continue
- `providerSpecificData.disableCooling=true` → no `RateLimitedUntil`
- Affinity header `X-Session-Id` sticks connection across two successful requests

- [ ] **Step 3: Changelog + commit**

```bash
git add docs/project-changelog.md
git commit -m "$(cat <<'EOF'
docs: note P0 multi-account reliability upgrades

EOF
)"
```

**P0 exit criteria**
- [ ] Taxonomy unified; `shouldFallbackToNextAccount` uses `IsRetryableUpstream`
- [ ] `CheckFallbackError` agrees with classifier on `improperly formed request` (non-fallback)
- [ ] Retry budget = distinct connections attempted; OAuth re-exec not double-counted
- [ ] Cooldown configurable + PSD `disableCooling`
- [ ] `fill-first` lower Priority wins
- [ ] Session affinity optional with locked key formats
- [ ] Defaults backward compatible (zero settings)
- [ ] `go test ./internal/domain ./internal/service ./internal/adapter/http` green + build OK

---

# Phase P1 — Protocol depth & extension points

### Task 8: Shared request prepare helper pattern

**Files:**
- Create: `internal/adapter/shared/prepare.go`
- Create: `internal/adapter/shared/prepare_test.go`

**Interfaces:**

```go
package shared

type PrepareFuncs struct {
	ParseModel  func(model string) (base string, effort string)
	ApplyEffort func(body []byte, effort string) []byte
	Normalize   func(body []byte) []byte
	Sanitize    func(body []byte, base string) []byte
}

// PrepareBody runs ParseModel → ApplyEffort → Normalize → Sanitize.
// Nil func slots are skipped. ParseModel nil => base=model, effort="".
func PrepareBody(model string, body []byte, fns PrepareFuncs) (base string, out []byte, err error)
```

Do **not** force xAI rewrite in this task (avoid scope creep). xAI may adopt in Task 9/10.

- [ ] **Step 1: Failing test**

```go
func TestPrepareBodyOrder(t *testing.T) {
	var steps []string
	fns := PrepareFuncs{
		ParseModel: func(m string) (string, string) { steps = append(steps, "parse"); return "base", "high" },
		ApplyEffort: func(b []byte, e string) []byte { steps = append(steps, "effort:"+e); return b },
		Normalize: func(b []byte) []byte { steps = append(steps, "norm"); return b },
		Sanitize: func(b []byte, base string) []byte { steps = append(steps, "san:"+base); return []byte("ok") },
	}
	base, out, err := PrepareBody("m-high", []byte(`{}`), fns)
	if err != nil || base != "base" || string(out) != "ok" {
		t.Fatalf("base=%s out=%s err=%v", base, out, err)
	}
	want := []string{"parse", "effort:high", "norm", "san:base"}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps=%v", steps)
	}
}
```

- [ ] **Step 2: FAIL** → **Step 3: implement** → **Step 4: PASS**

Run: `go test ./internal/adapter/shared -run TestPrepareBodyOrder -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/shared/prepare.go internal/adapter/shared/prepare_test.go
git commit -m "$(cat <<'EOF'
feat(shared): add composable request prepare pipeline helper

EOF
)"
```

---

### Task 9: Canonical thinking suffix module

**Files:**
- Create: `internal/service/thinking/suffix.go`
- Create: `internal/service/thinking/suffix_test.go`
- Create: `internal/service/thinking/apply.go`
- Create: `internal/service/thinking/apply_test.go`
- Modify: `internal/adapter/xai/model.go` — `ParseModel` delegates suffix split to `thinking.ParseSuffix` after prefix strip (keep `CanonicalModel` local)

**Interfaces:**

```go
package thinking

// ParseSuffix strips a trailing effort token: xhigh|high|medium|low|minimal|none
// Longer tokens first (xhigh before high).
func ParseSuffix(model string) (base string, effort string)

// ApplyXAIReasoningEffort sets JSON "reasoning":{"effort":effort} when effort != "".
func ApplyXAIReasoningEffort(body []byte, effort string) []byte

// ApplyOpenAIReasoningEffort sets "reasoning_effort" when effort != "" (chat-completions style).
func ApplyOpenAIReasoningEffort(body []byte, effort string) []byte
```

Do not wire into `executeOnProvider` (avoids double-apply with executor-side logic). xAI executor continues to call its ApplyReasoningEffort; optionally make that function call `thinking.ApplyXAIReasoningEffort`.

- [ ] **Step 1: Failing tests** (port cases from xai model tests)

```go
func TestParseSuffix(t *testing.T) {
	base, effort := ParseSuffix("grok-4.3-high")
	if base != "grok-4.3" || effort != "high" {
		t.Fatalf("%s %s", base, effort)
	}
	base, effort = ParseSuffix("grok-4.20-0309-reasoning")
	if base != "grok-4.20-0309-reasoning" || effort != "" {
		t.Fatalf("must not strip -reasoning model id")
	}
}
```

Note: `-reasoning` is **not** in the effort token list; only explicit effort suffixes strip.

- [ ] **Step 2: FAIL** → **Step 3: implement + xAI delegate** → **Step 4:**

Run: `go test ./internal/service/thinking ./internal/adapter/xai -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/service/thinking internal/adapter/xai/model.go
git commit -m "$(cat <<'EOF'
feat(thinking): centralize model thinking suffix parsing

EOF
)"
```

---

### Task 10: Extended executor options (backward compatible)

**Files:**
- Create: `internal/port/execute-options.go`
- Modify: `internal/adapter/xai/executor.go` — implement `ExecuteWithOptions`
- Create: `internal/adapter/xai/executor_options_test.go`
- Do **not** require chat-service changes in this task

**Interfaces:**

```go
package port

type ExecuteOptions struct {
	Stream    bool
	Alt       string // "responses/compact" reserved
	SessionID string
}

type ExecuteResult struct {
	Stream     io.ReadCloser
	Body       []byte
	StatusCode int
}

type ExtendedExecutor interface {
	ProviderExecutor
	ExecuteWithOptions(model string, body []byte, credentials *domain.Credentials, opts ExecuteOptions, reqlog RequestLogger) (*ExecuteResult, error)
}
```

xAI `ExecuteWithOptions`:
- `Stream==true` (default for chat path): call existing `Execute`, wrap as `ExecuteResult{Stream, StatusCode}`
- `Stream==false`: still may use stream internally and buffer until done **or** return error `status=501` if non-stream not implemented yet — **locked for P1:** implement stream passthrough only; non-stream returns `ExecuteResult` by reading full stream into memory only if trivial, else document TODO. Prefer: if `!opts.Stream`, call same Execute and read all bytes into Body, StatusCode=200 path.

Keep `Execute` method unchanged for ProviderExecutor.

- [ ] **Step 1: Failing test** — type assert `var _ port.ExtendedExecutor = (*xai.Executor)(nil)` compile test + runtime ExecuteWithOptions stream

```go
func TestXAIExecutorImplementsExtended(t *testing.T) {
	var _ port.ExtendedExecutor = NewExecutor()
}
```

- [ ] **Step 2–4:** implement + `go test ./internal/adapter/xai -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/port/execute-options.go internal/adapter/xai/executor.go internal/adapter/xai/executor_options_test.go
git commit -m "$(cat <<'EOF'
feat(port): add optional ExtendedExecutor with ExecuteOptions

EOF
)"
```

---

### Task 11: Execution hooks

**Files:**
- Create: `internal/service/execution-hooks.go`
- Create: `internal/service/execution-hooks_test.go`
- Modify: `internal/service/chat-service.go` — field `hooks *HookRegistry`; Before before Execute; After on success/fail (not when waiting on stream close)

**Interfaces:**

```go
package service

type ExecContext struct {
	RequestID    string
	Provider     string
	Model        string
	ConnectionID string
	StatusCode   int
	Err          error
	Duration     time.Duration
}

type ExecutionHook interface {
	BeforeExecute(ctx *ExecContext)
	AfterExecute(ctx *ExecContext)
}

type HookRegistry struct {
	mu    sync.RWMutex
	hooks []ExecutionHook
}

func NewHookRegistry() *HookRegistry
func (r *HookRegistry) Add(h ExecutionHook)
func (r *HookRegistry) Before(ctx *ExecContext) // no-op if r==nil
func (r *HookRegistry) After(ctx *ExecContext)
```

`NewChatService` creates empty registry (non-nil) so Add works later; methods nil-safe anyway.

- [ ] **Step 1: Registry order test**

```go
func TestHookRegistryOrder(t *testing.T) {
	var seq []string
	r := NewHookRegistry()
	r.Add(hookFunc{before: func(*ExecContext) { seq = append(seq, "b1") }, after: func(*ExecContext) { seq = append(seq, "a1") }})
	r.Add(hookFunc{before: func(*ExecContext) { seq = append(seq, "b2") }, after: func(*ExecContext) { seq = append(seq, "a2") }})
	ctx := &ExecContext{Provider: "xai"}
	r.Before(ctx)
	r.After(ctx)
	// expect b1,b2,a1,a2
}
```

Define unexported `hookFunc` test double in test file.

- [ ] **Step 2: Wire chat-service** Before after SelectAccount; After after success wrap or failure return (include status)

- [ ] **Step 3: PASS** `go test ./internal/service -count=1`

- [ ] **Step 4: Commit**

```bash
git add internal/service/execution-hooks.go internal/service/execution-hooks_test.go internal/service/chat-service.go
git commit -m "$(cat <<'EOF'
feat(service): add execution hooks for observability extensions

EOF
)"
```

---

### Task 12: Reasoning cache facade (provider-keyed)

**Files:**
- Create: `internal/adapter/shared/reasoning-cache.go`
- Create: `internal/adapter/shared/reasoning-cache_test.go`
- Do **not** rewrite xAI cache in this task unless a 5-line delegate is free; default is independent facade for future providers

**Interfaces:**

```go
package shared

type ReasoningCache interface {
	Put(provider, model, sessionKey string, items [][]byte)
	Get(provider, model, sessionKey string) ([][]byte, bool)
}

type MemoryReasoningCache struct { /* mu, ttl, max, map */ }

func NewMemoryReasoningCache(ttl time.Duration, maxEntries int) *MemoryReasoningCache
func (c *MemoryReasoningCache) Put(provider, model, sessionKey string, items [][]byte)
func (c *MemoryReasoningCache) Get(provider, model, sessionKey string) ([][]byte, bool)
```

Key: `provider + "\x00" + model + "\x00" + sessionKey`. Empty sessionKey → Put/Get no-op false.

- [ ] **Step 1: Tests** Put/Get roundtrip + empty session no-op + eviction maxEntries

- [ ] **Step 2–4:** implement + PASS `go test ./internal/adapter/shared -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/shared/reasoning-cache.go internal/adapter/shared/reasoning-cache_test.go
git commit -m "$(cat <<'EOF'
feat(shared): provider-keyed reasoning cache facade

EOF
)"
```

---

### Task 13: Quota-aware model fallback map

**Files:**
- Settings field already added in Task 2: `QuotaModelFallback map[string]string`
- Modify: `internal/service/chat-service.go` `HandleChat` combo callback wrapper
- Create: `internal/service/chat-service-quota-fallback_test.go`

**Behavior (locked):**
1. Lookup key = attempt’s qualified model string exactly as planned (`provider/model` without @pin unless present — use `provider/model` base from ParseModelString)
2. On attempt failure where `domain.IsQuotaExceeded(status, err)` is true AND map has fallback F AND F not in `tried` set → run one extra `executeOnProvider(F)` before combo advances
3. Prevent loops: `triedFallback` set includes original and F
4. If fallback missing or empty, no change to combo order

```go
func (s *ChatService) lookupQuotaFallback(qualifiedModel string) string {
	settings, err := s.store.GetSettings()
	if err != nil || settings == nil || len(settings.QuotaModelFallback) == 0 {
		return ""
	}
	// normalize: strip @pin for lookup
	base := qualifiedModel
	if i := strings.IndexByte(base, '@'); i >= 0 {
		base = base[:i]
	}
	return strings.TrimSpace(settings.QuotaModelFallback[base])
}
```

- [ ] **Step 1: Unit tests**

```go
func TestLookupQuotaFallbackStripsPin(t *testing.T) { /* map["xai/grok-4.3"]= "xai/grok-3-mini"; input xai/grok-4.3@c1 → mini */ }
func TestQuotaFallbackRunsOnce(t *testing.T) {
	// mock execute sequence: primary quota fail, fallback fail quota pointing back → ensure no infinite loop
}
```

- [ ] **Step 2: Implement wrapper in HandleChat around combo execute function**

- [ ] **Step 3: PASS** `go test ./internal/service -count=1`

- [ ] **Step 4: Commit**

```bash
git add internal/service/chat-service.go internal/service/chat-service-quota-fallback_test.go
git commit -m "$(cat <<'EOF'
feat(service): optional quota model fallback map

EOF
)"
```

---

### Task 14: P1 regression + docs

**Files:**
- Modify: `docs/project-changelog.md`

- [ ] **Step 1: Full test/build**

```bash
go test ./internal/domain -count=1
go test ./internal/service -count=1
go test ./internal/adapter/xai -count=1
go test ./internal/adapter/http -count=1
go test ./internal/adapter/shared -count=1
go build -o dntproxy ./cmd/dntproxy/
```

Expected: all PASS / BUILD_OK

- [ ] **Step 2: Acceptance checklist**

P1:
- [ ] `thinking.ParseSuffix` used by xAI model helper
- [ ] `var _ port.ExtendedExecutor = (*xai.Executor)(nil)` compiles
- [ ] Hook registry order test + chat-service calls Before/After
- [ ] shared ReasoningCache Put/Get tested
- [ ] quota fallback lookup + one-shot behavior tested
- [ ] P0 tests still green

- [ ] **Step 3: Changelog commit**

```bash
git add docs/project-changelog.md
git commit -m "$(cat <<'EOF'
docs: note P1 core extension points and thinking/executor options

EOF
)"
```

---

## Settings reference (final JSON shape)

```json
{
  "settings": {
    "comboStrategy": "fallback",
    "connectionStrategy": "fill-first",
    "maxRetryCredentials": 3,
    "cooldownEnabled": true,
    "transientCooldownSeconds": 2,
    "maxCooldownSeconds": 120,
    "modelLockEnabled": true,
    "sessionAffinityEnabled": true,
    "sessionAffinityTTLSeconds": 1800,
    "quotaModelFallback": {
      "xai/grok-4.3": "xai/grok-3-mini"
    }
  }
}
```

Connection opt-out:

```json
{
  "providerSpecificData": {
    "disableCooling": true,
    "sessionId": "optional-explicit-session"
  }
}
```

---

## Risk notes

1. **Priority sort:** locked lower-wins; fill-first reuses same ordering as `priorityFallbackSelect`.
2. **Retry budget:** count distinct connections attempted; OAuth re-exec same conn does not increment; pinned failures count as 1 then stop.
3. **Affinity vs pin:** hard `@connectionId` always wins over sticky soft-pin.
4. **Cooldown disable:** must not clear in-request `excludeIDs`; retry budget prevents hot loops when cooling is off.
5. **Classifier vs legacy cooldown:** `improperly formed request` becomes non-fallback everywhere (behavior change vs old `CheckFallbackError` transient path—intentional, matches live chat routing).
6. **P1 ExtendedExecutor:** optional type assert only; providers that omit it keep working.
7. **Task 2 before Task 3:** settings fields must exist before retry budget reads them.

---

## Self-review (post-edit)

### 1. Spec coverage (gap list → tasks)
| Gap | Task | Status |
|---|---|---|
| Retry taxonomy | 1 | covered |
| Settings/defaults | 2 | covered (was ordered after budget—fixed) |
| Credential retry budget | 3 | covered |
| Cooldown + disableCooling | 4 | covered |
| Fill-first | 5 | covered |
| Session affinity | 6 | covered |
| P0 regression | 7 | covered |
| Prepare helper | 8 | covered |
| Thinking suffix | 9 | covered |
| ExtendedExecutor | 10 | covered |
| Hooks | 11 | covered |
| Reasoning facade | 12 | covered |
| Quota model map | 13 | covered |
| P1 regression | 14 | covered |
| Hot-reload / WS / plugins | — | out of scope (explicit) |

### 2. Issues found and fixed in this review
1. **Task order:** settings moved before retry budget (Task 2 ↔ old Task 3).
2. **Taxonomy test bug:** `503+"overloaded"` expected `UpstreamTransient` but keyword path returns quota — tests corrected; empty-body 503 stays transient.
3. **Legacy conflict:** `CheckFallbackError` treated `improperly formed request` as transient; plan now forces non-fallback alignment with `shouldFallbackToNextAccount`.
4. **Retry semantics ambiguity:** locked to distinct connections attempted; OAuth double-exec excluded.
5. **Placeholder tests:** Task 4/5/6/8/9/11/12/13 rewritten with concrete cases (no “/* comment only */” steps).
6. **Priority inspect-later:** locked to lower-wins from live `priorityFallbackSelect`.
7. **RequestMetadata:** explicit new fields `SessionKey`, `APIKeyID` + affinity key format locked.
8. **disableCooling:** PSD-only (no struct field churn).
9. **QuotaModelFallback:** field introduced in Task 2 so Task 13 does not re-declare.
10. **combo-handler “prefer no change”:** Task 13 wires fallback in `HandleChat` wrapper instead of ambiguous combo-handler edits.

### 3. Remaining residual risks (acceptable)
- Full `TestExecuteOnProviderRespectsMaxRetryCredentials` needs test harness fidelity; helper test is mandatory fallback.
- Affinity soft-pin via temporary `@stickyID` may interact with API key allowlists—must ensure sticky id is in allowlist or skip sticky.
- Non-stream `ExecuteWithOptions` buffering may be heavy for huge responses—acceptable for P1.
- UI settings form may not expose new fields in P0 (API/JSON only)—documented.

### 4. Type/name consistency check
Stable names: `UpstreamClass`, `MaxRetryCredentials`, `ConnectionStrategyFillFirst`, `SessionAffinityStore`, `AffinityKey`, `ExecuteOptions`, `ExtendedExecutor`, `HookRegistry`, `ExecutionHook`, `QuotaModelFallback`, `shouldStopCredentialRetry`, `CooldownOn`, `ModelLockOn`.

### 5. Verdict
**Plan ready for execution** after self-review fixes. Prefer branch `feat/core-reliability-p0` for Tasks 1–7, then `feat/core-reliability-p1` for 8–14.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-09-core-reliability-p0-p1.md` (self-reviewed).

**Two execution options:**

1. **Subagent-Driven (recommended)** — fresh subagent per task, review between tasks  
2. **Inline Execution** — execute in this session with checkpoints  

**Recommendation:** run **P0 Tasks 1–7 first** on a dedicated worktree, merge, then P1.

Which approach?
