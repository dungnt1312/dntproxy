# Phase 02 - API Key Permissions UI

## Context Links

- [Plan](./plan.md)
- [API Keys screen](../../ui/src/components/screens/api-keys-screen.tsx)
- [Generate dialog](../../ui/src/components/screens/api-keys/generate-dialog.tsx)
- [Keys table](../../ui/src/components/screens/api-keys/keys-table.tsx)
- [Keys mobile](../../ui/src/components/screens/api-keys/keys-mobile.tsx)
- [Connection types](../../ui/src/types/connections.ts)

## Overview

- Priority: P1
- Status: Completed
- Effort: 3.5h
- Goal: Let admin create/edit connection/model allowlists per API key.

## Key Insights

- API Keys screen is 300+ lines; keep it as orchestrator.
- Existing shadcn/ui has Checkbox, Switch, Dialog, Badge, Table, ScrollArea.
- Connections/models are already fetchable via `goApi.getConnections()` and `goApi.getModels()`.
- Empty allowlist is unrestricted. UI wording must prevent accidental lockout confusion.

## Requirements

- Create key dialog supports:
  - name
  - active defaults true (backend creates active)
  - access mode: unrestricted or restricted
  - selected connections
  - selected models
- Edit key dialog supports:
  - rename
  - active/inactive switch
  - connection/model allowlist updates
- Table/mobile cards show:
  - unrestricted vs restricted
  - connection count
  - model count
- Search should include permission summary where useful.
- Empty connection/model lists mean unrestricted, not "none".

## Architecture

- Keep `api-keys-screen.tsx` as data loader and handler host.
- Extract UI modules under `ui/src/components/screens/api-keys/`:
  - `permissions-editor.tsx`: reusable selection editor
  - `edit-key-dialog.tsx`: edit existing key
  - optionally `permission-summary.tsx`: badge/count renderer
- `GenerateDialog` can reuse `PermissionsEditor` instead of becoming large.

## Related Code Files

- Modify: `C:\laragon\www\dntproxy\ui\src\components\screens\api-keys-screen.tsx`
- Modify: `C:\laragon\www\dntproxy\ui\src\components\screens\api-keys\generate-dialog.tsx`
- Modify: `C:\laragon\www\dntproxy\ui\src\components\screens\api-keys\keys-table.tsx`
- Modify: `C:\laragon\www\dntproxy\ui\src\components\screens\api-keys\keys-mobile.tsx`
- Create: `C:\laragon\www\dntproxy\ui\src\components\screens\api-keys\permissions-editor.tsx`
- Create: `C:\laragon\www\dntproxy\ui\src\components\screens\api-keys\edit-key-dialog.tsx`
- Optional create: `C:\laragon\www\dntproxy\ui\src\components\screens\api-keys\permission-summary.tsx`

## Implementation Steps

1. In `api-keys-screen.tsx`, fetch keys, connections, models in parallel.
2. Add states:
   - `editTarget`
   - `saving`
   - `connections`
   - `models`
3. Update `GenerateDialog` props:
   - `connections`
   - `models`
   - `onGenerate(payload)`
4. Build `PermissionsEditor`:
   - radio/toggle: unrestricted vs restricted
   - provider-grouped connection checkboxes
   - model selector grouped by provider/model display
   - selected count + clear all
5. Build `EditKeyDialog`:
   - initial values from selected `ApiKey`
   - save through `goApi.updateKey`
   - refresh keys on success
6. Add edit action button to table/mobile.
7. Add permission summary column/card section.
8. Error handling:
   - toast on load/save failure
   - disable save while saving
   - keep dialog open on error.

## Todo List

- [x] Fetch connections/models on API Keys screen.
- [x] Create reusable permissions editor.
- [x] Extend generate dialog with restrictions.
- [x] Add edit dialog.
- [x] Add table/mobile edit action.
- [x] Add permission summary badges.
- [x] Keep responsive layout clean.

## Success Criteria

- Create unrestricted key works.
- Create restricted key stores selected `allowedConnectionIds` and/or `allowedModels`.
- Edit key updates restrictions and active state.
- Mobile and desktop both expose same core controls.
- No screen file exceeds local guideline without extraction.

## Risk Assessment

- Risk: too many models makes dialog long.
- Mitigation: ScrollArea, provider grouping, search/filter if needed.
- Risk: user selects restricted mode but no items.
- Mitigation: treat no items as unrestricted or block save with clear message. Recommended: block save when restricted mode and both lists empty.

## Security Considerations

- Only admin/API-authenticated dashboard users can access `/api/keys`.
- Do not reveal provider credential material in connection labels.
- Consider masking generated dntproxy key except existing one-time show.

## Next Steps

- Phase 03 validates server behavior and tests UI build.

## Unresolved Questions

- Should UI allow model-only restriction without connection restriction? Recommended yes.
