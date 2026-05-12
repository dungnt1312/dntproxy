# Phase 01 - UX Contract And Data Hardening

## Context Links
- `ui/src/components/screens/models-screen.tsx`
- `ui/src/components/screens/routing/types.ts`
- `ui/src/lib/go-api.ts`
- `ui/src/lib/provider-registry.ts`
- `docs/code-standards.md`

## Overview
- Priority: Critical
- Status: Complete
- Define the page contract before changing visuals. Fix data assumptions that can crash or mislead UI.

## Key Insights
- `UiModel.name` is required by UI but `mapModel()` does not guarantee it.
- Per-request `.catch(() => [])` converts API failure into fake empty states.
- Provider labels/colors are duplicated instead of using provider registry.
- Legacy `ui/src/pages/models.tsx` creates confusion but is not active.

## Requirements
- Normalize `UiModel` so display fields are always safe.
- Use one provider metadata source.
- Expose meaningful load state: loading, partial error, full error, empty, search-empty.
- Decide final screen IA:
  - Header
  - Tabs
  - Per-tab toolbar
  - Main content
  - Dialog/sheet flows

## Architecture
- Keep `ModelsScreen` as orchestrator.
- Add focused helpers under `ui/src/components/screens/routing/`.
- Use `goApi` only; do not reintroduce legacy `api`.

## Related Code Files
- Modify:
  - `ui/src/lib/go-api.ts`
  - `ui/src/components/screens/routing/types.ts`
  - `ui/src/components/screens/models-screen.tsx`
  - `ui/src/components/screens/routing/models-tab.tsx`
  - `ui/src/components/screens/routing/aliases-tab.tsx`
  - `ui/src/components/screens/routing/combos-tab.tsx`
- Consider deleting later:
  - `ui/src/pages/models.tsx`

## Implementation Steps
1. Add a safe model display contract:
   - `name = displayName || name || modelId || id`
   - keep `id`, `modelId`, `provider`, `connectionId`, `connectionName`, `connections`.
2. Replace local provider label maps with `getProviderLabel/getProviderMeta`.
3. Replace silent per-call failure with result collection:
   - allow partial UI if one endpoint fails
   - show visible error banner with retry.
4. Define shared UI primitives needed by later phases:
   - `routing-toolbar.tsx`
   - `routing-empty-state.tsx`
   - `routing-error-state.tsx`
   - `model-display.ts`.
5. Confirm active route usage and mark legacy page for removal in Phase 04.

## Todo List
- [x] Normalize `UiModel`
- [x] Remove duplicate provider label logic
- [x] Add explicit load/error states
- [x] Document target IA in code comments only where useful
- [x] Re-run build

## Success Criteria
- Search cannot crash on missing `name`.
- API failure never looks like "no models".
- Provider labels/icons/colors match Connections/Playground.
- Phase 02 can build on stable model fields.

## Risk Assessment
- Partial data may need careful messaging so user knows what failed.
- Existing components may assume `name` always exists.

## Security Considerations
- Do not log API keys or connection secrets in error UI.
- Keep backend error messages concise.

## Next Steps
- Implement registry redesign.

## Unresolved Questions
- None.
