---
title: "Diff: Small Ports Plan vs Provider Operations Plan"
status: pending
created: 2026-04-13
updated: 2026-04-13
labels: [diff, refactor, architecture, review]
---

# Diff: Small Ports Plan vs Provider Operations Plan

## Compared Files

- old: `./.qwen/plans/2026-04-13-refactor-provider-operations-interface.md`
- new: `./.qwen/plans/2026-04-13-refactor-http-provider-admin-small-ports.md`

## High-Level Shift

Old plan:
- introduce one composite `ProviderOperations` interface
- make each provider implement `test`, `quota`, `fetch-models`
- route more HTTP admin behavior through that composite interface

New plan:
- do not introduce composite interface
- add only one narrow port: `ConnectionTester`
- keep existing `ModelFetcher`, `QuotaChecker`, `ProviderExecutor`
- split handlers by action, not by forced provider abstraction

## Semantic Diff

| Topic | Old Plan | New Plan | Why It Changed |
|------|----------|----------|----------------|
| Core abstraction | `ProviderOperations` with `TestConnection`, `CheckQuota`, `FetchModels` | `ConnectionTester` only, keep `ModelFetcher` and `QuotaChecker` | avoid fat interface and forced no-op implementations |
| Existing ports | effectively superseded by composite interface | reused and kept alive | codebase already has these ports; less churn |
| `apiTestModel` boundary | moved under `ProviderOperations` | stays on `ProviderExecutor` | test-model is execution-path validation, not admin probe |
| `fetch-models` endpoint | conceptually grouped with auth split, but route remained blurry | explicitly moved from auth router to connection admin router | endpoint is not part of auth flow |
| `openai-compatible` support | missing in proposed ops map | explicitly covered in provider matrix and startup wiring | prevent regression |
| `qwen` handling | treated as simple API-key provider in Phase 4 | keeps both API key and OAuth behavior | old plan under-specified Qwen OAuth |
| No-op adapters | planned for deletion | reused for unsupported quota/model-fetch flows | still useful, less unnecessary churn |
| Provider file strategy | one file per provider ops wrapper | shared standard adapters + provider-specific special cases | reduces boilerplate |
| Auth split | 4 files, with Kiro still large | split Kiro into device and social files | better chance to keep files under 200 lines |
| Quota refactor | hidden inside provider ops migration | explicit phase to thin `quota-handler.go` | makes the 507-line file a direct target |
| Execution order | more parallel, more moving parts | more linear, lower coordination cost | simpler to implement safely |
| Testing plan | mock `ProviderOperations`, manual regression focus | test standard tester, OAuth tester, model fetcher, thin handlers | aligns tests to real boundaries |

## Added in New Plan

- baseline statement that current `go build` and `go test` are green
- `ProviderAdminDeps` bundle for HTTP wiring
- explicit provider coverage matrix
- explicit route move for `/api/connections/:id/fetch-models`
- explicit statement that `openai-compatible` must remain supported
- explicit statement that `qwen` OAuth must not regress
- explicit recommendation section

## Removed from New Plan

- `ProviderOperations` interface
- per-provider `provider_ops.go` sprawl for `glm`, `minimax`, `qwen`, `anthropic`, `gemini`
- deletion plan for existing no-op adapters
- claim that adding a new provider only needs `provider_ops.go` + one register line
- assumption that all simple providers can share identical behavior without auth nuance

## Risk Profile Change

Old plan increased risk of:
- wrong abstraction boundary
- dropping `openai-compatible`
- downgrading `qwen` OAuth handling
- mixing `test-model` with connectivity probes
- deleting reusable adapters too early

New plan reduces risk by:
- keeping boundaries aligned with existing code
- reusing current ports instead of replacing them
- isolating route moves from auth logic
- preserving execution-path behavior for `test-model`
- making provider coverage explicit

## Recommendation

Use the new plan.

Reason:
- less architectural churn
- fewer fake abstractions
- better fit with current codebase
- lower regression risk on provider support
- cleaner path to shrink `connection-handler.go`, `auth-handler.go`, `quota-handler.go`

## Raw Diff Note

A raw `git diff --no-index` was generated locally between the two plan files.
This document is the semantic summary, not a line-by-line patch dump.

## Unresolved Questions

1. `anthropic` should get a real minimal connection tester now, or stay explicit unsupported until adapter work lands?
2. `StandardModelFetcher` should stay where it is for this refactor, or move out of `internal/port/` in a follow-up cleanup?
