# Phase 05 - Tests And Docs

## Context Links

- [Plan](./plan.md)
- [Project changelog](../../docs/project-changelog.md)
- [Codebase summary](../../docs/codebase-summary.md)
- [System architecture](../../docs/system-architecture.md)
- [Roadmap](../../docs/project-roadmap.md)

## Overview

- Priority: P1
- Status: Pending
- Effort: 3h
- Goal: Lock behavior with tests and document new model access architecture.

## Key Insights

- This refactor is security-sensitive.
- Tests should prove no drift between list and execute.
- Docs must explain computed pool, not persisted pool.

## Requirements

- Add tests for central service.
- Update existing chat tests.
- Update handler tests.
- Add docs for effective pool semantics.
- Run verification commands.

## Test Matrix

Required cases:

- Unrestricted key:
  - sees all active models.
  - can execute direct model, alias, combo.
- Connection-restricted key:
  - sees only models supported by those connections.
  - cannot execute pinned disallowed connection.
- Model-restricted key:
  - sees only allowed model and aliases targeting it.
  - combos visible only if effective members exist.
- Combined restriction:
  - model allowed but no connection support -> hidden/forbidden.
  - connection allowed but model disallowed -> hidden/forbidden.
- Dynamic update behavior:
  - update connection supported models.
  - next pool build reflects it without manual pool update.
- Deleted connection referenced by key:
  - pool excludes missing connection.
  - key edit UI can later clean stale IDs.

## Related Code Files

- Create/modify: `C:\laragon\www\dntproxy\internal\service\model-access-service_test.go`
- Modify: `C:\laragon\www\dntproxy\internal\service\chat-service_test.go`
- Modify: `C:\laragon\www\dntproxy\internal\adapter\http\models-handler_test.go`
- Modify: `C:\laragon\www\dntproxy\docs\system-architecture.md`
- Modify: `C:\laragon\www\dntproxy\docs\codebase-summary.md`
- Modify: `C:\laragon\www\dntproxy\docs\project-changelog.md`
- Modify: `C:\laragon\www\dntproxy\docs\project-roadmap.md`

## Implementation Steps

1. Add model access service unit tests first.
2. Update chat tests to expect effective route attempts.
3. Update `/v1/models` tests to compare with service pool.
4. Run:
   - `go test ./...`
   - `cd ui; bun run build`
   - `go build ./cmd/dntproxy`
5. Update docs:
   - effective pool concept.
   - combo partial visibility.
   - no persisted pool/cache.
6. Update plan checkboxes and final report.

## Todo List

- [ ] Add service unit tests.
- [ ] Update chat tests.
- [ ] Update handler tests.
- [ ] Run Go tests.
- [ ] Run UI build.
- [ ] Run Go build.
- [ ] Update docs.

## Success Criteria

- All tests pass.
- Docs explain one source of truth for model access.
- No stale endpoint-local policy code remains.

## Risk Assessment

- Risk: broad refactor touches security routing.
- Mitigation: tests before and after each integration phase.

## Security Considerations

- Treat list leakage as security issue.
- Treat execute/list mismatch as security issue.
- Do not log API key values in tests or docs.

## Next Steps

- After implementation, ask whether to commit.

## Unresolved Questions

- Dashboard `/api/models` behavior for restricted dashboard key still needs final product decision.
