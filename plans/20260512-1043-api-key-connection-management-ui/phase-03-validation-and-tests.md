# Phase 03 - Validation And Tests

## Context Links

- [Plan](./plan.md)
- [Key handler](../../internal/adapter/http/key-handler.go)
- [Router middleware](../../internal/adapter/http/router.go)
- [Chat service](../../internal/service/chat-service.go)
- [Chat service tests](../../internal/service/chat-service_test.go)
- [UI package](../../ui/package.json)

## Overview

- Priority: P1
- Status: Completed
- Effort: 2h
- Goal: Ensure policy persistence and enforcement remain correct after UI integration.

## Key Insights

- Backend already enforces policies from API key middleware.
- Handler allows update but has minimal validation.
- Current create response omits allowlists; list response includes full key objects.
- Existing service tests cover allowed connection behavior.

## Requirements

- Backend should reject invalid policy references if feasible:
  - unknown connection IDs
  - malformed model names if strict enough
- Tests should cover create/update persistence of allowlists.
- UI build must pass.
- Existing Go tests must pass.

## Architecture

- Prefer small validation helper in `key-handler.go` or new focused file if handler exceeds practical size.
- Keep domain unchanged unless new metadata required.
- Do not add database migration; config JSON already stores arrays.

## Related Code Files

- Modify: `C:\laragon\www\dntproxy\internal\adapter\http\key-handler.go`
- Optional create: `C:\laragon\www\dntproxy\internal\adapter\http\key-handler_test.go`
- Optional modify: `C:\laragon\www\dntproxy\internal\service\chat-service_test.go`
- Modify as needed: `C:\laragon\www\dntproxy\ui\src\components\screens\api-keys\*.tsx`

## Implementation Steps

1. Add handler tests for:
   - create key with allowlists persists arrays
   - update key changes name, active state, connection/model arrays
   - delete still works
2. If adding validation:
   - load config
   - build connection ID set
   - reject unknown IDs with 400
   - dedupe arrays before save
3. Keep empty arrays as unrestricted.
4. Run backend tests:
   - `go test ./...`
5. Run frontend checks:
   - `cd ui; bun install` if dependencies missing
   - `bun run build`
6. Fix compile/type errors.

## Todo List

- [x] Add/adjust backend handler tests.
- [x] Add dedupe/validation helper if needed.
- [x] Run `go test ./...`.
- [x] Run `bun run build`.
- [x] Fix failures without fake data or bypass.

## Success Criteria

- `go test ./...` passes.
- `bun run build` passes.
- Manual payloads through `/api/keys` store/retrieve policy arrays.
- Restricted key cannot reach disallowed connection/model.

## Risk Assessment

- Risk: strict validation blocks existing configs with deleted connections.
- Mitigation: validate on new create/update only; list existing data as-is.
- Risk: frontend build exposes type debt in unrelated files.
- Mitigation: document unrelated failures if not introduced; fix introduced errors.

## Security Considerations

- Do not weaken `apiKeyMiddleware`.
- Keep require API key behavior unchanged.
- Avoid returning extra secret data from new endpoints.

## Next Steps

- Phase 04 updates docs and changelog.

## Unresolved Questions

- Need exact policy for deleted connections referenced by old keys. Recommended: display as "missing" and allow admin to remove on edit.
