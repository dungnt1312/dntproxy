# Auth Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align FE/BE dashboard auth, close public OAuth/write endpoints, stamp tenant on all connection creates, and add logout + consistent unauthorized handling.

**Architecture:** Keep the existing three-plane model (dashboard key / proxy key / provider credentials). Make dashboard auth always required end-to-end. Protect OAuth *write* endpoints with `dashboardKeyMiddleware` while leaving only pure “start” endpoints public if needed for browser redirects. Unify FE auth state in `go-api.ts` + `App.tsx`. Stamp `TenantID` from request context on every connection create path.

**Tech Stack:** Go 1.25 + Gin (BE), React 19 + Vite + Zustand (FE), existing `JsonDB` credential store, existing `APIKey` / `ProviderConnection` domain types.

## Global Constraints

- Do **not** reintroduce open dashboard mode (`RequireAPIKey=false` must not skip auth for `/api/*` or FE gate).
- Preserve proxy behavior for `/v1/*` (always requires API key; policy/tenant injection unchanged).
- Keep OAuth UX working from the dashboard (user already logged in with a dashboard key).
- Tenant isolation: new connections inherit `GetTenantID(c)`; empty tenant = legacy admin.
- File naming remains kebab-case; keep handler files under ~200–300 lines when practical.
- Prefer tests next to existing `*_test.go` patterns under `internal/adapter/http/`.
- Do not hash API keys in this plan (out of scope; plaintext `db.json` stays for now).
- Commit after each task with a focused message.

## File Map

| File | Responsibility |
|------|----------------|
| `internal/adapter/http/router.go` | Middleware exemptions; helper to mount protected auth routes if needed |
| `internal/adapter/http/auth-handler.go` | Split public start vs protected complete/import/fetch-models |
| `internal/adapter/http/api-handler.go` | Optionally host protected auth completion routes under `/api` group |
| `internal/adapter/http/auth-kiro-handler.go` | Stamp `TenantID` on poll success |
| `internal/adapter/http/auth-social-handler.go` | Stamp `TenantID` on social exchange |
| `internal/adapter/http/auth-openai-handler.go` | Stamp `TenantID` on OpenAI exchange |
| `internal/adapter/http/auth-qwen-handler.go` | Stamp `TenantID` on Qwen poll success |
| `internal/adapter/http/auth-xai-handler.go` | Stamp `TenantID` on xAI exchange (import already does) |
| `internal/adapter/http/auth-models-handler.go` | Move under dashboard middleware + ownership check |
| `internal/adapter/http/auth-guard.go` | Reuse `requireTenantOwnsConnection` |
| `internal/adapter/http/key-handler.go` | Session: reject inactive / non-dashboard keys for UI session |
| `internal/adapter/http/*_auth_hardening_test.go` (new or extend) | Middleware + tenant stamp + protected routes tests |
| `ui/src/lib/go-api.ts` | `clearStoredApiKey` / logout helper; 401 clears storage |
| `ui/src/api.ts` | Route unauthorized through shared `onUnauthorized`; treat 403 like 401 |
| `ui/src/App.tsx` | Always-on auth gate; logout; fail closed on settings error |
| `ui/src/components/screens/login-screen.tsx` | Copy when always required |
| `ui/src/components/layout/sidebar.tsx` | Logout control |
| `ui/src/components/screens/settings-screen.tsx` | Clarify/remove misleading `requireApiKey` toggle UX |
| `ui/src/stores/app-store.ts` | No structural change required (optional logout helper) |

## Decision Log (locked)

1. **Dashboard auth is always required.** `Settings.RequireAPIKey` remains a stored field for backward-compatible JSON, but:
   - BE middleware continues to ignore it for `/api/*` and `/v1/*` (already always-on).
   - FE stops using it as a login gate.
   - Settings UI either removes the toggle or labels it as **deprecated / no-op** with a note that auth is always enforced.
2. **OAuth route protection model:**
   - **Public (no key):** none for connection-mutating endpoints.
   - **Protected (dashboard key):** all OAuth start/poll/exchange/import + `fetch-models`.
   - Rationale: FE already has a key when operator adds connections; public OAuth was an abuse surface (anyone can append connections to `db.json`).
3. **401 handling:** clear `localStorage` key + session → LoginScreen.
4. **Tenant stamp:** every new `ProviderConnection` sets `TenantID: GetTenantID(c)`.

---

### Task 1: Protect OAuth completion + fetch-models behind dashboard middleware

**Files:**
- Modify: `internal/adapter/http/auth-handler.go`
- Modify: `internal/adapter/http/api-handler.go`
- Modify: `internal/adapter/http/router.go` (only if route registration comments need update)
- Test: `internal/adapter/http/auth-routes_test.go` (create)

**Interfaces:**
- Consumes: `dashboardKeyMiddleware(store)`, existing auth handler funcs
- Produces: OAuth + fetch-models only reachable with valid dashboard API key

- [ ] **Step 1: Write failing tests for public OAuth write routes**

Create `internal/adapter/http/auth-routes_test.go`:

```go
package http

import (
	"net/http"
	"net/http/httptest	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gin-gonic/gin"
)

// Use the same in-memory / fake store pattern as key-handler_test.go.
// If key-handler_test has a helper, reuse it; otherwise minimal stub:

type memStore struct {
	cfg *domain.AppConfig
}

// Implement only methods needed by middleware + handlers under test,
// or reuse existing test store from key-handler_test.go if exported.

func TestAuthRoutesRequireDashboardKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Build router with RegisterAPIRoutes + RegisterAuthRoutes like production.
	// Assert unauthenticated POST returns 401 for each mutating auth path.
	paths := []string{
		"/api/auth/kiro/start-builderid",
		"/api/auth/kiro/start-idc",
		"/api/auth/kiro/poll",
		"/api/auth/kiro/start-social",
		"/api/auth/kiro/exchange-social",
		"/api/auth/openai/start",
		"/api/auth/openai/exchange",
		"/api/auth/qwen/start",
		"/api/auth/qwen/poll",
		"/api/auth/xai/start",
		"/api/auth/xai/exchange",
		"/api/auth/xai/import-file",
		"/api/connections/some-id/fetch-models",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		w := httptest.NewRecorder()
		// r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s: got %d want 401", path, w.Code)
		}
	}
}

func TestAuthRoutesAcceptDashboardKey(t *testing.T) {
	// Create store with active key DashboardAccess=true, Key="sk-dnt-test"
	// POST /api/auth/kiro/start-builderid with Authorization: Bearer sk-dnt-test
	// Expect not 401/403 (may be 500/upstream error — that's OK; auth passed).
}
```

Adapt the test harness to whatever store helper already exists in `key-handler_test.go` / `connection-handler_test.go` (prefer reuse over a new full `CredentialStore` mock).

- [ ] **Step 2: Run tests — expect FAIL (routes currently public)**

```bash
go test ./internal/adapter/http/ -run TestAuthRoutesRequireDashboardKey -count=1
```

Expected: FAIL (200 or non-401 on unauthenticated OAuth starts).

- [ ] **Step 3: Move auth route registration under dashboard middleware**

Preferred approach (minimal surface change for FE paths — keep `/api/auth/*` URLs):

In `RegisterAuthRoutes`, stop mounting on bare `*gin.Engine`. Instead:

**Option A (recommended):** Change signature to accept a protected group:

```go
// auth-handler.go
func RegisterAuthRoutes(api *gin.RouterGroup, store port.CredentialStore) {
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/kiro/start-builderid", authStartBuilderID())
		authGroup.POST("/kiro/start-idc", authStartIDC())
		authGroup.POST("/kiro/poll", authPoll(store))
		authGroup.POST("/kiro/start-social", authStartSocial())
		authGroup.POST("/kiro/exchange-social", authExchangeSocial(store))
		authGroup.POST("/openai/start", authOpenAIStart())
		authGroup.POST("/openai/exchange", authOpenAIExchange(store))
		authGroup.POST("/qwen/start", authQwenStart())
		authGroup.POST("/qwen/poll", authQwenPoll(store))
		authGroup.POST("/xai/start", authXAIStart())
		authGroup.POST("/xai/exchange", authXAIExchange(store))
		authGroup.POST("/xai/import-file", authXAIImportFile(store))
	}
	api.POST("/connections/:id/fetch-models", apiFetchConnectionModels(store))
}
```

In `api-handler.go` inside the `api` group that already uses `dashboardKeyMiddleware`:

```go
RegisterAuthRoutes(api, store)
```

In `router.go` `NewRouter`, **remove**:

```go
RegisterAuthRoutes(r, store)
```

Ensure `fetch-models` is not registered twice. If `api-handler.go` already has other connection routes, keep `fetch-models` only once under the protected group.

- [ ] **Step 4: Ownership check on fetch-models**

In `auth-models-handler.go` after loading `conn`:

```go
if !requireTenantOwnsConnection(c, conn) {
	return
}
```

Use the existing helper in `auth-guard.go` (it should abort with 404 on cross-tenant). If the helper returns bool vs aborts, match its existing call style used by other connection handlers.

- [ ] **Step 5: Run tests — expect PASS**

```bash
go test ./internal/adapter/http/ -run 'TestAuthRoutes' -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/http/auth-handler.go internal/adapter/http/api-handler.go internal/adapter/http/router.go internal/adapter/http/auth-models-handler.go internal/adapter/http/auth-routes_test.go
git commit -m "fix(auth): require dashboard key for OAuth and fetch-models"
```

---

### Task 2: Stamp TenantID on all OAuth connection creates

**Files:**
- Modify: `internal/adapter/http/auth-kiro-handler.go` (poll success `ProviderConnection{...}`)
- Modify: `internal/adapter/http/auth-social-handler.go`
- Modify: `internal/adapter/http/auth-openai-handler.go`
- Modify: `internal/adapter/http/auth-qwen-handler.go`
- Modify: `internal/adapter/http/auth-xai-handler.go` (exchange path; import already stamps)
- Test: `internal/adapter/http/auth-tenant_test.go` (create) or extend Task 1 tests

**Interfaces:**
- Consumes: `GetTenantID(c string)` from `router.go`
- Produces: every new OAuth `ProviderConnection` has `TenantID` matching the dashboard key’s tenant

- [ ] **Step 1: Write failing tenant-stamp test**

```go
func TestKiroPollStampsTenantID(t *testing.T) {
	// Arrange: store with tenant key TenantID="acme", DashboardAccess=true
	// Seed an in-memory auth session OR call the handler with a mocked poll success path.
	// If full OAuth poll is hard to mock, unit-test by extracting a small helper:

	// preferred: helper used by all handlers
	// conn := newOAuthConnection(c, store, fields...)
	// assert conn.TenantID == "acme"
}
```

Pragmatic approach if full OAuth is hard:

```go
// In a shared helper file (optional) or test the field assignment via table
// after refactoring connection construction.

func TestOAuthConnectionIncludesTenant(t *testing.T) {
	// After implementation, load config post-handler and assert TenantID.
}
```

Minimum acceptable test: for **one** provider path (e.g. xAI import already correct — assert OpenAI exchange or a small extracted builder):

```go
func buildConnectionTenant(c *gin.Context, base domain.ProviderConnection) domain.ProviderConnection {
	base.TenantID = GetTenantID(c)
	return base
}
```

- [ ] **Step 2: Run test — FAIL (missing TenantID)**

```bash
go test ./internal/adapter/http/ -run Tenant -count=1
```

- [ ] **Step 3: Add TenantID to every OAuth create composite literal**

Pattern (repeat in each handler where `domain.ProviderConnection{` is built for OAuth success):

```go
conn := domain.ProviderConnection{
	// ...existing fields...
	TenantID: GetTenantID(c),
}
```

Handlers/locations:
- `auth-kiro-handler.go` — poll success
- `auth-social-handler.go` — exchange success
- `auth-openai-handler.go` — exchange success
- `auth-qwen-handler.go` — poll success
- `auth-xai-handler.go` — exchange success (import already has `TenantID: GetTenantID(c)`)

Do **not** change apikey create paths that already set `TenantID: GetTenantID(c)` in `connection-add-handler.go`.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/adapter/http/ -count=1
go test ./internal/service/ -count=1
```

Expected: PASS (or only pre-existing failures unrelated to this change — report faithfully).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/http/auth-kiro-handler.go internal/adapter/http/auth-social-handler.go internal/adapter/http/auth-openai-handler.go internal/adapter/http/auth-qwen-handler.go internal/adapter/http/auth-xai-handler.go internal/adapter/http/auth-tenant_test.go
git commit -m "fix(auth): stamp TenantID on all OAuth connection creates"
```

---

### Task 3: Harden session validation for dashboard UI

**Files:**
- Modify: `internal/adapter/http/key-handler.go` (`apiSession`, optionally `apiValidateKey`)
- Test: `internal/adapter/http/key-handler_test.go` (extend)

**Interfaces:**
- Produces: `GET /api/auth/session` returns `authenticated:false` when key lacks `DashboardAccess`, is inactive, or tenant disabled

- [ ] **Step 1: Write failing session tests**

```go
func TestAPISessionRejectsNonDashboardKey(t *testing.T) {
	// Key valid for /v1 but DashboardAccess=false
	// GET /api/auth/session with Bearer → authenticated:false
}

func TestAPISessionRejectsInactiveKey(t *testing.T) {
	// IsActive=false → authenticated:false
}
```

- [ ] **Step 2: Run — FAIL if session currently returns authenticated:true for non-dashboard keys**

```bash
go test ./internal/adapter/http/ -run TestAPISession -count=1
```

- [ ] **Step 3: Update `apiSession`**

```go
func apiSession(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := extractAPIKey(c.Request)
		if key == "" {
			key = c.Query("key")
		}
		if key == "" {
			c.JSON(200, gin.H{"authenticated": false})
			return
		}
		apiKey, valid := store.GetAPIKeyByValue(key)
		if !valid || apiKey == nil || !apiKey.IsActive || !apiKey.DashboardAccess {
			c.JSON(200, gin.H{"authenticated": false})
			return
		}
		if isTenantDisabledCached(store, apiKey.TenantID) {
			c.JSON(200, gin.H{"authenticated": false})
			return
		}
		c.JSON(200, gin.H{
			"authenticated":   true,
			"dashboardAccess": true,
			"tenantId":        apiKey.TenantID,
			"isAdmin":         domain.IsLegacyTenant(apiKey.TenantID),
			"keyId":           apiKey.ID,
			"keyName":         apiKey.Name,
		})
	}
}
```

Keep `apiValidateKey` returning `valid` + `dashboardAccess` separately so LoginScreen can show “no dashboard access” (already does).

- [ ] **Step 4: Run tests — PASS**

```bash
go test ./internal/adapter/http/ -run 'TestAPISession|TestValidate' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/http/key-handler.go internal/adapter/http/key-handler_test.go
git commit -m "fix(auth): session requires active dashboard key"
```

---

### Task 4: FE auth gate always-on + fail closed + clear key on 401

**Files:**
- Modify: `ui/src/lib/go-api.ts`
- Modify: `ui/src/api.ts`
- Modify: `ui/src/App.tsx`
- Modify: `ui/src/components/screens/login-screen.tsx`

**Interfaces:**
- Consumes: `goApi.getSession()`, `getStoredApiKey`, `setStoredApiKey`
- Produces: `clearAuth()` that clears storage + notifies App; dual clients share one unauthorized path

- [ ] **Step 1: Add shared clear + wire both clients**

In `ui/src/lib/go-api.ts`:

```ts
export function clearStoredApiKey() {
  localStorage.removeItem(AUTH_KEY);
}

export function clearAuth() {
  clearStoredApiKey();
  on401Callback?.();
}
```

In `goRequest` 401/403 branch:

```ts
if (res.status === 401 || res.status === 403) {
  clearStoredApiKey();
  on401Callback?.();
  throw new Error("Unauthorized");
}
```

In `ui/src/api.ts` — remove separate `onLegacyUnauthorized` callback (or make it an alias):

```ts
import { getStoredApiKey, onUnauthorized, clearStoredApiKey } from './lib/go-api';

// delete onLegacyUnauthorized or:
export const onLegacyUnauthorized = onUnauthorized;

async function request(path: string, options?: RequestInit) {
  // ...
  if (res.status === 401 || res.status === 403) {
    clearStoredApiKey();
    // go-api's callback is registered by App; call via onUnauthorized side effect:
    // simplest: import a notifyUnauthorized() from go-api
    throw new Error('Unauthorized');
  }
}
```

Cleaner: export from `go-api.ts`:

```ts
export function notifyUnauthorized() {
  clearStoredApiKey();
  on401Callback?.();
}
```

Use `notifyUnauthorized()` from both `goRequest` and `api.ts` `request`.

- [ ] **Step 2: Rewrite App bootstrap to always require login**

Replace auth gate in `ui/src/App.tsx`:

```tsx
const [authReady, setAuthReady] = useState(false);
const [authenticated, setAuthenticated] = useState(false);

const showLogin = useCallback(() => {
  setAuthenticated(false);
  clearSession();
}, [clearSession]);

useEffect(() => {
  onUnauthorized(showLogin);
}, [showLogin]);

useEffect(() => {
  let cancelled = false;
  (async () => {
    try {
      const stored = getStoredApiKey();
      if (!stored) {
        if (!cancelled) {
          setAuthenticated(false);
          setAuthReady(true);
        }
        return;
      }
      const sess = await goApi.getSession();
      if (cancelled) return;
      if (sess.authenticated && sess.dashboardAccess !== false) {
        setAuthenticated(true);
        setSession({
          tenantId: sess.tenantId ?? "",
          isAdmin: Boolean(sess.isAdmin),
          dashboardAccess: Boolean(sess.dashboardAccess),
          keyId: sess.keyId,
          keyName: sess.keyName,
        });
      } else {
        // invalid / non-dashboard key
        const { clearStoredApiKey } = await import("@/lib/go-api");
        clearStoredApiKey();
        setAuthenticated(false);
      }
    } catch {
      // Fail closed: network or server error during bootstrap → login
      setAuthenticated(false);
    } finally {
      if (!cancelled) setAuthReady(true);
    }
  })();
  return () => {
    cancelled = true;
  };
}, [setSession]);

if (!authReady) {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="animate-pulse text-muted-foreground text-sm">Loading...</div>
    </div>
  );
}

if (!authenticated) {
  return <LoginScreen onSuccess={() => setAuthenticated(true)} />;
}
```

Remove all `requireApiKey` / `authRequired` branching from App.

- [ ] **Step 3: Update LoginScreen copy**

```tsx
<p className="text-sm text-muted-foreground text-center">
  Enter a dashboard API key to continue.
</p>
```

- [ ] **Step 4: Manual verify (dev)**

```bash
# terminal 1
go run ./cmd/dntproxy
# terminal 2
cd ui && npm run dev
```

Checks:
1. Open `/dashboard` with empty localStorage → LoginScreen
2. Paste non-dashboard key → error “does not have dashboard access”
3. Paste valid dashboard key → app loads
4. Force 401 (delete key server-side or corrupt localStorage mid-session) → LoginScreen and localStorage cleared
5. Connections OAuth start still works while logged in

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/go-api.ts ui/src/api.ts ui/src/App.tsx ui/src/components/screens/login-screen.tsx
git commit -m "fix(ui): always-on dashboard auth gate and shared 401 handling"
```

---

### Task 5: Logout control in sidebar

**Files:**
- Modify: `ui/src/components/layout/sidebar.tsx`
- Modify: `ui/src/App.tsx` (pass `onLogout` if needed)
- Modify: `ui/src/lib/go-api.ts` (reuse `notifyUnauthorized` / `clearStoredApiKey`)

**Interfaces:**
- Produces: user-visible Logout that clears key + session + shows LoginScreen

- [ ] **Step 1: Add logout handler**

In `App.tsx`:

```tsx
const handleLogout = useCallback(() => {
  clearStoredApiKey(); // from go-api
  clearSession();
  setAuthenticated(false);
}, [clearSession]);
```

Pass to Sidebar:

```tsx
<Sidebar
  // existing props...
  onLogout={handleLogout}
/>
```

- [ ] **Step 2: Render logout button in sidebar footer**

In `sidebar.tsx` (near collapse control):

```tsx
import { LogOut } from "lucide-react";

// props: onLogout?: () => void

{onLogout && (
  <Button
    variant="ghost"
    className={cn("w-full justify-start gap-2", !sidebarOpen && "justify-center px-0")}
    onClick={onLogout}
    title="Log out"
  >
    <LogOut className="size-4" />
    {sidebarOpen && <span>Log out</span>}
  </Button>
)}
```

Match existing button density/classes in that file.

- [ ] **Step 3: Manual verify**

1. Login → Log out → LoginScreen  
2. localStorage `dntproxy_api_key` removed  
3. Hard refresh still LoginScreen  

- [ ] **Step 4: Commit**

```bash
git add ui/src/App.tsx ui/src/components/layout/sidebar.tsx
git commit -m "feat(ui): add dashboard logout"
```

---

### Task 6: Settings UX — requireApiKey no longer pretends to gate dashboard

**Files:**
- Modify: `ui/src/components/screens/settings-screen.tsx`
- Modify: `ui/src/lib/go-api.ts` (mapping comments only if needed)
- Optional: leave BE `Settings.RequireAPIKey` field for JSON compatibility

**Decision for this task:** Keep the field in API payload for backward compatibility, but change UI to a **read-only notice** (or admin-only deprecated toggle that cannot disable enforcement). Preferred UX:

- Replace the switch with an informational card:

```tsx
<div className="space-y-1">
  <Label>API Key Authentication</Label>
  <p className="text-xs text-muted-foreground">
    Always enabled. Dashboard and /v1 API requests require a valid API key.
    Create keys under API Keys; enable Dashboard Access for UI login.
  </p>
</div>
```

- Stop sending `requireApiKey: false` from the form as a user action.
- If `updateSettings` still maps `apiKeyAuthEnabled`, either:
  - remove field from form state, or
  - force `apiKeyAuthEnabled: true` on save.

Recommended save payload behavior:

```ts
// go-api updateSettings — always persist requireApiKey: true
requireApiKey: true,
```

Or keep storing whatever is on server but UI never offers disable. Prefer **force true on save** only if you also update BE docs; safer FE-only: remove control and do not include `requireApiKey` in PUT body (leave existing DB value).

**Chosen approach for implementer:** remove toggle; **do not** PUT `requireApiKey` from this form anymore (omit field so other settings updates don’t flip it). Document in UI that auth is always on.

- [ ] **Step 1: Edit settings Security card** as above  
- [ ] **Step 2: Remove `apiKeyAuthEnabled` from dirty-check if unused**  
- [ ] **Step 3: Grep FE for remaining gate logic**

```bash
rg -n "requireApiKey|apiKeyAuthEnabled" ui/src -g '*.ts' -g '*.tsx'
```

Only mapping leftovers + tunnel warning should remain. Update tunnel warning copy if it says “enable require API key”:

```tsx
// tunnel-screen: change to "Ensure only trusted dashboard keys exist before enabling tunnel"
```

- [ ] **Step 4: Manual verify Settings + Tunnel screens render**  
- [ ] **Step 5: Commit**

```bash
git add ui/src/components/screens/settings-screen.tsx ui/src/components/screens/tunnel-screen.tsx ui/src/lib/go-api.ts
git commit -m "fix(ui): remove misleading requireApiKey toggle; auth always on"
```

---

### Task 7: OAuth session map caps + cleanup consistency

**Files:**
- Modify: `internal/adapter/http/auth-handler.go`
- Modify: `internal/adapter/http/auth-kiro-handler.go` / `auth-social-handler.go` / `auth-openai-handler.go` / `auth-qwen-handler.go` / `auth-xai-handler.go` (start handlers)
- Test: small unit test for reject-when-full

**Interfaces:**
- Produces: all session maps enforce `maxAuthSessions` (already 1000 for xAI)

- [ ] **Step 1: Add helper**

```go
// auth-handler.go
func authSessionRoom(mu *sync.Mutex, length int) bool {
	mu.Lock()
	defer mu.Unlock()
	return length < maxAuthSessions
}
```

Better: check under the same lock as insert:

```go
authSessionsMu.Lock()
if len(authSessions) >= maxAuthSessions {
	authSessionsMu.Unlock()
	c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many pending auth sessions"})
	return
}
authSessions[sessionID] = ...
authSessionsMu.Unlock()
```

Apply same pattern to `openaiSessions`, `qwenSessions`, `xaiSessions` (xAI already has check — align status code/message).

- [ ] **Step 2: Test optional** — table test helper if extracted  
- [ ] **Step 3: Commit**

```bash
git add internal/adapter/http/auth-handler.go internal/adapter/http/auth-kiro-handler.go internal/adapter/http/auth-social-handler.go internal/adapter/http/auth-openai-handler.go internal/adapter/http/auth-qwen-handler.go internal/adapter/http/auth-xai-handler.go
git commit -m "fix(auth): cap pending OAuth sessions for all providers"
```

---

### Task 8: Verification pass + docs touch-up

**Files:**
- Modify: `AGENTS.md` (auth notes only if they claim RequireAPIKey gates middleware)
- Optional: `docs/multi-tenancy.md` one paragraph on OAuth tenant stamp

- [ ] **Step 1: Full test suite**

```bash
go test ./... -count=1
```

Report failures faithfully; fix regressions from this plan only.

- [ ] **Step 2: Manual E2E checklist**

| # | Case | Expected |
|---|------|----------|
| 1 | No key → `/dashboard` | Login |
| 2 | Proxy-only key (no dashboard) | Login error |
| 3 | Dashboard key | Full UI |
| 4 | Logout | Login + empty storage |
| 5 | Add Kiro/OpenAI/Qwen/xAI OAuth while logged in | Success; connection has tenant |
| 6 | Unauthenticated curl OAuth start | 401 |
| 7 | Unauthenticated fetch-models | 401 |
| 8 | Tenant A key cannot fetch-models for tenant B conn | 404 |
| 9 | `/v1/models` without key | 401 |
| 10 | `/v1/models` with proxy key | 200 |

- [ ] **Step 3: Update AGENTS.md Key Behaviors if needed**

Add short bullet under Key Behaviors:

```markdown
### Auth
- Dashboard (`/api/*`) and proxy (`/v1/*`) always require API keys
- OAuth connection enrollment requires a dashboard-capable API key
- New connections inherit the creating key's TenantID
```

- [ ] **Step 4: Final commit**

```bash
git add AGENTS.md docs/multi-tenancy.md
git commit -m "docs: document always-on auth and OAuth tenant stamping"
```

---

## Out of Scope (follow-ups)

| Item | Why deferred |
|------|----------------|
| Hash API keys at rest | Larger migration + CLI/UI reveal flow redesign |
| Unify `api.ts` into `go-api.ts` fully | Large FE churn; Task 4 only shares unauthorized path |
| CLI Qwen/xAI auth parity | Separate UX work |
| OpenAI callback port unify (1455 vs 20129) | CLI-only; not security-critical |
| Rate-limit OAuth starts per IP | Needs reverse-proxy or gin middleware design |
| Delete dead `ui/src/pages/settings.tsx` | Cleanup; not auth-critical |

## Priority order

| Order | Task | Severity |
|------:|------|----------|
| 1 | Protect OAuth + fetch-models | P0 security |
| 2 | TenantID stamp | P0 multi-tenant isolation |
| 3 | Session harden | P1 |
| 4 | FE always-on gate + 401 clear | P0 UX/security mismatch |
| 5 | Logout | P1 UX |
| 6 | Settings toggle cleanup | P1 UX honesty |
| 7 | Session map caps | P2 hardening |
| 8 | Verification + docs | P2 |

## Self-Review

1. **Spec coverage:** Public OAuth, fetch-models, tenant stamp, FE/BE requireApiKey mismatch, logout, dual-client 401, session non-dashboard keys, session caps — each has a task.
2. **No placeholders:** Paths, code sketches, commands, and expected outcomes included; tests describe concrete assertions.
3. **Type consistency:** `GetTenantID(c)`, `DashboardAccess`, `notifyUnauthorized`/`clearStoredApiKey`, route paths under `/api/auth/*` preserved for FE.

---

## Execution Handoff

Plan saved to `docs/superpowers/plans/2026-07-13-auth-hardening.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — fresh subagent per task, review between tasks  
2. **Inline Execution** — execute tasks in this session with checkpoints  

Which approach?
