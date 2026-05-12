# Phase 02 - Registry Redesign

## Context Links
- `ui/src/components/screens/routing/models-tab.tsx`
- `ui/src/components/screens/routing/routing-card.tsx`
- `ui/src/components/ui/table.tsx`
- `ui/src/components/ui/badge.tsx`
- `ui/src/components/ui/button.tsx`

## Overview
- Priority: High
- Status: Complete
- Replace decorative model cards with a compact registry surface optimized for scanning and testing.

## Key Insights
- Registry is an inspection/search workflow, not a card browsing workflow.
- Model IDs are long; wrapping chips/cards create visual noise.
- Actions are hidden behind hover; poor for admin tooling.
- Provider identity should remain visible at row level, not only section header.

## Requirements
- Show a dense but readable model list.
- Keep provider grouping or filter, but do not bury row-level provider context.
- Actions must be visible and keyboard/touch usable.
- Support search by model name, ID, provider, connection name.
- Stable sort by provider then display name.

## Architecture
- `models-tab.tsx` remains tab coordinator.
- Extract:
  - `model-registry-table.tsx`
  - `model-registry-mobile-list.tsx`
  - `model-test-button.tsx`
  - `provider-filter.tsx` if filtering is useful.
- Reuse shadcn `Table`, `Button`, `Badge`, `Input`.

## Related Code Files
- Modify:
  - `ui/src/components/screens/routing/models-tab.tsx`
  - `ui/src/components/screens/routing/routing-card.tsx` only if still used elsewhere
- Create:
  - `ui/src/components/screens/routing/model-registry-table.tsx`
  - `ui/src/components/screens/routing/model-registry-mobile-list.tsx`
  - `ui/src/components/screens/routing/model-test-button.tsx`

## Implementation Steps
1. Replace provider accordion cards with table on desktop:
   - Provider
   - Model
   - Model ID
   - Connections
   - Status
   - Actions
2. Use a mobile list with the same information hierarchy.
3. Add provider filter or segmented pills only if it reduces noise.
4. Keep log and test actions visible:
   - log icon button
   - test button per model/connection, with status result.
5. Use truncation with tooltip/title for long IDs.
6. Add search result summary and clear-filter action.
7. Keep empty state specific:
   - no registry data
   - no search match
   - load error.

## Todo List
- [x] Build desktop table
- [x] Build mobile list
- [x] Make actions visible
- [x] Add stable sort/filter
- [x] Add no-results state
- [x] Verify long IDs do not break layout

## Success Criteria
- User can scan 30+ models without tag soup.
- Long model IDs are readable/copyable enough without layout overflow.
- Model test/log actions are obvious.
- Mobile layout is not horizontally broken.

## Risk Assessment
- Table can become too dense; use responsive columns and mobile list.
- Per-connection test controls can clutter rows; cap visible chips and use overflow text when needed.

## Security Considerations
- No secret values in row details or tooltips.

## Next Steps
- Redesign alias and combo workflows.

## Unresolved Questions
- None.
