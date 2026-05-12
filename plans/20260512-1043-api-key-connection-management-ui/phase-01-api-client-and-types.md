# Phase 01 - API Client And Types

## Context Links

- [Plan](./plan.md)
- [README](../../README.md)
- [Code standards](../../docs/code-standards.md)
- [API key handler](../../internal/adapter/http/key-handler.go)
- [API client](../../ui/src/lib/go-api.ts)
- [Keys table type](../../ui/src/components/screens/api-keys/keys-table.tsx)

## Overview

- Priority: P1
- Status: Completed
- Effort: 1.5h
- Goal: Align frontend API key contract with backend policy fields.

## Key Insights

- Backend already accepts `allowedConnectionIds` and `allowedModels` on create/update.
- `goApi.createKey(name)` currently sends only `name`.
- No `goApi.updateKey` helper exists.
- `ApiKey` interface omits policy fields.

## Requirements

- Add typed payload shape for create/update key.
- Preserve current simple generate flow as default unrestricted key.
- Normalize `allowedConnectionIds` and `allowedModels` to arrays in `getKeys`.
- Add `goApi.updateKey(id, payload)`.

## Architecture

- Frontend-only API client change.
- Keep unified `goApi` usage.
- No new backend endpoint unless validation in Phase 03 requires it.

## Related Code Files

- Modify: `C:\laragon\www\dntproxy\ui\src\lib\go-api.ts`
- Modify: `C:\laragon\www\dntproxy\ui\src\components\screens\api-keys\keys-table.tsx`
- Optionally create: `C:\laragon\www\dntproxy\ui\src\components\screens\api-keys\types.ts`

## Implementation Steps

1. Extract `ApiKey`, `ApiKeyCreatePayload`, `ApiKeyUpdatePayload` into `api-keys/types.ts`.
2. Add optional fields:
   - `allowedConnectionIds: string[]`
   - `allowedModels: string[]`
3. Update `getKeys` mapper:
   - `allowedConnectionIds: Array.isArray(...) ? ... : []`
   - `allowedModels: Array.isArray(...) ? ... : []`
4. Change `createKey` to accept object payload, with compatibility wrapper in screen.
5. Add `updateKey(id, payload)` using `PUT /keys/:id`.

## Todo List

- [x] Add shared API key types.
- [x] Update `goApi.getKeys`.
- [x] Update `goApi.createKey`.
- [x] Add `goApi.updateKey`.
- [x] Update imports using old `ApiKey` type.

## Success Criteria

- TypeScript compiles.
- Existing key generation still works.
- API client can send create/update policy payloads.

## Risk Assessment

- Risk: breaking older `createKey(name)` caller.
- Mitigation: update all callers in same phase; grep confirms main caller is API Keys screen.

## Security Considerations

- Do not show provider connection secrets.
- Do not log full generated key except existing one-time dialog behavior.

## Next Steps

- Phase 02 consumes the updated types/client.

## Unresolved Questions

- None.
