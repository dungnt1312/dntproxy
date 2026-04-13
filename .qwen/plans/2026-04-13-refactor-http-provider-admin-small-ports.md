---
title: "Refactor: HTTP Provider Admin Handlers (Small Ports)"
status: pending
created: 2026-04-13
updated: 2026-04-13
labels: [refactor, architecture, http-handlers, provider-admin]
---

# Plan: Refactor HTTP Provider Admin Handlers (Small Ports)

## Overview

Refactor oversized HTTP admin handlers without introducing a composite `ProviderOperations` interface.

Current pressure points:
- `internal/adapter/http/connection-handler.go` - 912 lines
- `internal/adapter/http/auth-handler.go` - 999 lines
- `internal/adapter/http/quota-handler.go` - 507 lines
- `/api/connections/:id/fetch-models` currently lives in auth routing even though it is a connection admin action

This plan keeps boundaries small and honest:
- add one new narrow port: `ConnectionTester`
- keep existing `ModelFetcher` and `QuotaChecker` ports
- keep `ProviderExecutor` responsible for `/api/connections/:id/test-model`
- split handlers by action so new files stay under the size target

Baseline before refactor:
- `go build ./cmd/dntproxy` passes
- `go test ./...` passes

## Scope

### In Scope
- `internal/adapter/http/*`
- `internal/port/connection-tester.go`
- provider admin adapters under `internal/adapter/{kiro,openai,shared,custom}/`
- wiring in `cmd/dntproxy/main.go`

### Out of Scope
- chat request flow in `internal/service/chat-service.go`
- combo/alias/key/settings handlers
- UI behavior and endpoint contracts
- `usage-handler.go` functional changes

## Success Criteria

1. New or rewritten HTTP handler files stay under 200 lines where practical, with helpers extracted before exceeding the limit.
2. No composite provider admin interface is introduced.
3. `openai-compatible` remains fully supported for test, fetch-models, and quota paths.
4. `qwen` OAuth remains supported and is not downgraded to API-key-only behavior.
5. `/api/connections/:id/fetch-models` moves out of auth routing into connection admin routing.
6. `/api/connections/:id/test-model` continues to use `ProviderExecutor`, not the new admin ports.
7. `go build ./cmd/dntproxy` and `go test ./...` still pass after refactor.

## Architecture Decisions

### Decision 1: Add only one new port

Create:

```go
// internal/port/connection-tester.go
type ConnectionTester interface {
    TestConnection(conn *domain.ProviderConnection) (*ConnectionTestResult, error)
}
```

Reason:
- `TestConnection` is the only missing narrow concern.
- `ModelFetcher` and `QuotaChecker` already exist.
- no forced no-op methods, no "fat interface", no fake polymorphism.

### Decision 2: Bundle admin dependencies in HTTP adapter, not port layer

Create a small dependency bundle for router wiring:

```go
type ProviderAdminDeps struct {
    Testers       map[string]port.ConnectionTester
    ModelFetchers map[string]port.ModelFetcher
    QuotaCheckers map[string]port.QuotaChecker
}
```

Reason:
- one extra router param instead of three parallel maps everywhere
- keeps port layer interface-only
- immutable after startup, no mutex needed

### Decision 3: Keep `ProviderExecutor` for model execution probes

`apiTestModel` currently validates real model execution through `ProviderExecutor`. That is correct and must stay separate from admin probes.

Reason:
- `TestConnection` answers "are credentials/connectivity usable?"
- `TestModel` answers "can this exact model execute?"
- these are different failure domains

### Decision 4: Move fetch-models out of auth layer

`/api/connections/:id/fetch-models` is a connection admin action, not an auth action.

Reason:
- removes misplaced logic from `auth-handler.go`
- lets auth router focus only on interactive login/session flows

### Decision 5: Prefer shared standard adapters plus explicit special cases

Use shared adapters where behavior is actually shared:
- standard bearer-token/API-key connection tester for OpenAI-like chat APIs
- standard `/v1/models` fetcher for OpenAI-style APIs
- existing no-op adapters for unsupported quota/model-fetch paths

Keep explicit special cases:
- `kiro` tester
- `kiro` quota checker
- `openai` OAuth tester
- `openai` OAuth model fetcher (`/backend-api/models`)
- `openai` quota checker

Reason:
- reduces file count
- keeps `openai-compatible` covered
- avoids fake per-provider wrappers with no real behavior

## Target File Layout

### HTTP Layer

```text
internal/adapter/http/
|- api-handler.go                              # route wiring only
|- provider-admin-deps.go                      # ProviderAdminDeps
|- connection-handler-helpers.go               # shared lookup/refresh/save helpers
|- connection-list-handler.go
|- connection-add-handler.go
|- connection-crud-handler.go
|- connection-test-connection-handler.go
|- connection-test-model-handler.go
|- connection-fetch-models-handler.go
|- connection-detect-kiro-handler.go
|- auth-handler.go                             # auth route wiring only
|- auth-session-store.go
|- auth-kiro-device-handler.go
|- auth-kiro-social-handler.go
|- auth-openai-handler.go
|- auth-qwen-handler.go
`- quota-handler.go                            # thin dispatcher only
```

### Provider/Admin Adapters

```text
internal/port/connection-tester.go
internal/adapter/shared/standard-connection-tester.go
internal/adapter/shared/standard-model-fetcher.go
internal/adapter/kiro/connection-tester.go
internal/adapter/kiro/quota-checker.go
internal/adapter/openai/oauth-connection-tester.go
internal/adapter/openai/oauth-model-fetcher.go
internal/adapter/openai/quota-checker.go
```

Existing adapters to reuse:
- `internal/adapter/custom/model-fetcher.go`
- `internal/adapter/custom/quota-checker.go`

## Provider Coverage Matrix

| Provider | ConnectionTester | ModelFetcher | QuotaChecker |
|----------|------------------|--------------|--------------|
| `kiro` | custom | config-default/no-op | custom |
| `openai` API key | standard | standard | custom |
| `openai` OAuth | custom Codex probe | custom ChatGPT backend fetcher | custom |
| `openai-compatible` | standard | standard | no-op |
| `glm` | standard | no-op/config-default | no-op |
| `minimax` | standard | no-op/config-default | no-op |
| `qwen` API key | standard | no-op/config-default | no-op |
| `qwen` OAuth | standard bearer-token path | no-op/config-default | no-op |
| `gemini` | standard | no-op/config-default | no-op |
| `anthropic` | explicit unsupported or provider-specific minimal tester | no-op/config-default | no-op |

## Implementation Phases

### Phase 1: Introduce narrow admin abstractions

Files:
- new `internal/port/connection-tester.go`
- new `internal/adapter/http/provider-admin-deps.go`

Steps:
1. add `ConnectionTester` and `ConnectionTestResult`
2. add immutable `ProviderAdminDeps`
3. update router signatures only after deps compile locally

Definition of done:
- no behavior change yet
- compile still green

### Phase 2: Implement shared and provider-specific admin adapters

Files:
- new `internal/adapter/shared/standard-connection-tester.go`
- new `internal/adapter/shared/standard-model-fetcher.go`
- new `internal/adapter/kiro/connection-tester.go`
- new `internal/adapter/kiro/quota-checker.go`
- new `internal/adapter/openai/oauth-connection-tester.go`
- new `internal/adapter/openai/oauth-model-fetcher.go`
- new `internal/adapter/openai/quota-checker.go`

Steps:
1. extract generic API-key/bearer probe from `testProviderAPI()`
2. extract OpenAI OAuth Codex probe from `probeCodexAPI()` and `checkCodexQuota()`
3. extract Kiro quota logic from `handleKiroQuota()`
4. leave unsupported quota/model-fetch paths on existing no-op adapters
5. map `openai-compatible` explicitly to standard adapters

Definition of done:
- no handler calls provider-specific inline logic anymore for these concerns

### Phase 3: Wire admin deps in startup path

Files:
- `cmd/dntproxy/main.go`
- `internal/adapter/http/router.go`
- `internal/adapter/http/api-handler.go`

Steps:
1. build `ProviderAdminDeps` once in `main.go`
2. pass deps into `NewRouter()`
3. pass deps into API route registration
4. keep existing `ProviderRegistry` wiring unchanged for chat/test-model

Definition of done:
- startup path compiles
- provider coverage includes `openai-compatible`

### Phase 4: Split connection handler by action

Files:
- new `internal/adapter/http/connection-handler-helpers.go`
- new `internal/adapter/http/connection-list-handler.go`
- new `internal/adapter/http/connection-add-handler.go`
- new `internal/adapter/http/connection-crud-handler.go`
- new `internal/adapter/http/connection-test-connection-handler.go`
- new `internal/adapter/http/connection-test-model-handler.go`
- new `internal/adapter/http/connection-fetch-models-handler.go`
- new `internal/adapter/http/connection-detect-kiro-handler.go`
- delete `internal/adapter/http/connection-handler.go`

Steps:
1. extract shared config lookup, refresh, save, masking helpers
2. move `apiTestConnection` to `ConnectionTester`
3. move `apiFetchConnectionModels` here and bind it under `/api/connections/:id/fetch-models`
4. keep `apiTestModel` on `ProviderExecutor`
5. keep response JSON shapes unchanged

Definition of done:
- connection admin actions no longer live in one giant file
- `/fetch-models` no longer hangs off auth routing

### Phase 5: Split auth handler by flow

Files:
- new `internal/adapter/http/auth-session-store.go`
- new `internal/adapter/http/auth-kiro-device-handler.go`
- new `internal/adapter/http/auth-kiro-social-handler.go`
- new `internal/adapter/http/auth-openai-handler.go`
- new `internal/adapter/http/auth-qwen-handler.go`
- rewrite `internal/adapter/http/auth-handler.go`

Steps:
1. move session maps and cleanup into session store file
2. separate Kiro device flow from Kiro social flow
3. keep OpenAI and Qwen flows isolated
4. make `auth-handler.go` route wiring only
5. remove fetch-models route from auth router

Definition of done:
- auth router contains only auth endpoints
- each auth flow has one focused file

### Phase 6: Thin quota handler

Files:
- rewrite `internal/adapter/http/quota-handler.go`

Steps:
1. replace provider-specific inline branches with `QuotaChecker` dispatch
2. preserve current unsupported-provider JSON shape
3. keep OpenAI refresh behavior inside the OpenAI checker, not HTTP handler

Definition of done:
- `quota-handler.go` becomes a dispatcher, not a 500-line logic bucket

### Phase 7: Cleanup and verification

Files:
- remove dead helpers left behind by extraction
- update comments in `api-handler.go`

Checks:
1. `go build ./cmd/dntproxy`
2. `go test ./...`
3. verify route list still includes:
   - `POST /api/connections/:id/test`
   - `POST /api/connections/:id/test-model`
   - `POST /api/connections/:id/check-quota`
   - `POST /api/connections/:id/fetch-models`
4. smoke test:
   - OpenAI API key connection test
   - OpenAI OAuth connection test
   - Qwen OAuth connection test
   - GLM connection test
   - Kiro quota check

## Related Code Files

Primary:
- `cmd/dntproxy/main.go`
- `internal/adapter/http/api-handler.go`
- `internal/adapter/http/router.go`
- `internal/adapter/http/connection-handler.go`
- `internal/adapter/http/auth-handler.go`
- `internal/adapter/http/quota-handler.go`

Supporting:
- `internal/port/model-fetcher.go`
- `internal/port/quota-checker.go`
- `internal/domain/provider-config.go`
- `internal/adapter/custom/model-fetcher.go`
- `internal/adapter/custom/quota-checker.go`

## Testing Strategy

### Unit

1. test `standard-connection-tester` with `httptest.Server`
2. test `openai/oauth-connection-tester` on 200, 400, 401, 429 paths
3. test `openai/oauth-model-fetcher` parsing ChatGPT backend payload
4. test thin HTTP handlers with fake deps maps

### Integration

1. add connection -> test connection -> fetch models -> update -> delete
2. OpenAI OAuth pending/complete path
3. Qwen OAuth pending/complete path

### Regression

1. existing `go test ./...`
2. confirm no route or JSON contract drift for UI-facing admin endpoints

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| `openai-compatible` accidentally dropped during wiring | High | explicit mapping in startup deps |
| `qwen` OAuth broken by API-key assumptions | High | standard tester must support bearer access token path |
| auth/file split still exceeds 200 lines | Medium | split Kiro into device/social files from the start |
| quota refresh logic changes behavior | Medium | keep refresh semantics inside provider checker and reuse current code |
| route drift for `/fetch-models` | Medium | move route in same phase as handler extraction and smoke test it |
| duplicate quota logic remains in `usage-handler.go` | Low | accept for now, schedule later extraction after this refactor lands |

## Execution Order

1. Phase 1 - abstractions
2. Phase 2 - provider admin adapters
3. Phase 3 - wiring
4. Phase 4 - connection split
5. Phase 5 - auth split
6. Phase 6 - quota thinning
7. Phase 7 - cleanup and verification

## Recommendation

Proceed with this plan instead of the old `ProviderOperations` hybrid plan.

Why:
- smaller interfaces
- less boilerplate
- correct boundary for `test-model`
- covers `openai-compatible`
- does not regress `qwen` OAuth
- fixes the misplaced `/fetch-models` endpoint

## Unresolved Questions

1. For `anthropic`, do we want a real minimal tester now, or return an explicit "not supported yet" response until the provider adapter is completed?
2. Do we want to move the concrete `StandardModelFetcher` implementation out of `internal/port/` in this refactor, or leave that cleanup to a follow-up if it starts widening scope?
