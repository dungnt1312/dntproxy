# Phase 04 - Tests Build Docs

## Overview
- Priority: High
- Status: Complete
- Purpose: Prove fixes and document behavior.

## Key Insights
- `go test ./internal/service` currently passes.
- Full `go test ./...` currently fails in storage summary test, separate from chat path.
- New behavior touches security-sensitive routing.

## Requirements
- Add focused unit tests for all policy/fallback changes.
- Run compile checks.
- Update docs if externally visible API key semantics change.

## Related Code Files
- Modify/add tests under:
  - `internal/service/*_test.go`
  - `internal/adapter/http/*_test.go`
  - `internal/logger/*_test.go` if logging helper changes
- Docs:
  - `docs/codebase-summary.md` or relevant API key docs if present
  - `docs/project-changelog.md` if present

## Implementation Steps
1. Add service tests for model allowlist filtering and connection policy fallback.
2. Add handler tests for `/v1/messages` body limit.
3. Add stream error/log behavior tests if helper boundaries allow.
4. Run:
   - `go test ./internal/service ./internal/adapter/http ./internal/logger`
   - `go build -o $env:TEMP\dntproxy-review.exe ./cmd/dntproxy`
   - `go test ./...`
5. If full suite still fails due storage summary mismatch, document as separate pre-existing/follow-up issue.
6. Update changelog/docs with final semantics.

## Todo List
- [x] Add focused tests.
- [x] Run service/http/logger tests.
- [x] Run build.
- [x] Run full suite and record storage issue separately if still present.
- [x] Update docs/changelog if behavior changes.

## Risk Assessment
- Test fakes may need extending for API key policy and stream errors.
- Keep tests realistic; no fake pass-throughs that skip the actual routing logic.

## Security Considerations
- Add regression tests for restricted API keys.
