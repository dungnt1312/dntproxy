# Phase 03 - Aliases And Combos Workflow Redesign

## Context Links
- `ui/src/components/screens/routing/aliases-tab.tsx`
- `ui/src/components/screens/routing/combos-tab.tsx`
- `ui/src/components/screens/routing/combo-step-builder.tsx`
- `ui/src/components/ui/dialog.tsx`
- `ui/src/components/ui/alert-dialog.tsx`
- `ui/src/components/ui/select.tsx`

## Overview
- Priority: High
- Status: Complete
- Treat aliases and combos as routing configuration workflows, not passive cards.

## Key Insights
- Alias target should not be raw free text when registry models exist.
- Combo builder has useful logic but too much UI density and some wrong states.
- Pinned account display/resolution is weak for `provider/model@connectionId`.
- Current colors mix object type and primary action semantics.

## Requirements
- Alias create/edit uses dialog with model picker.
- Combo create/edit uses clearer step builder with validation.
- Deletion remains confirmable.
- List rows use consistent visible actions.
- Color semantics:
  - primary action uses default primary
  - provider/type accents used only as metadata.

## Architecture
- Extract smaller components:
  - `alias-dialog.tsx`
  - `aliases-list.tsx`
  - `combo-dialog.tsx`
  - `combo-step-builder.tsx` split into form/list/helpers
  - `model-picker.tsx`
- Keep each component near or under 200 lines.

## Related Code Files
- Modify:
  - `ui/src/components/screens/routing/aliases-tab.tsx`
  - `ui/src/components/screens/routing/combos-tab.tsx`
  - `ui/src/components/screens/routing/combo-step-builder.tsx`
  - `ui/src/components/screens/routing/routing-card.tsx`
- Create:
  - `ui/src/components/screens/routing/alias-dialog.tsx`
  - `ui/src/components/screens/routing/aliases-list.tsx`
  - `ui/src/components/screens/routing/combo-dialog.tsx`
  - `ui/src/components/screens/routing/combo-step-form.tsx`
  - `ui/src/components/screens/routing/combo-step-list.tsx`
  - `ui/src/components/screens/routing/model-picker.tsx`
  - `ui/src/components/screens/routing/routing-format.ts`

## Implementation Steps
1. Alias tab:
   - Replace inline create panel with dialog.
   - Add alias name input + model picker.
   - Validate duplicate/empty alias and empty target.
   - Hide empty state while dialog open only if needed.
2. Combo tab:
   - Keep list + create/edit dialog.
   - Resolve pinned model strings for display.
   - Fix provider/model/account placeholder bugs.
   - Filter accounts by provider and supported model when data is available.
   - Avoid mutating `formSteps` during save.
3. Step builder:
   - Split form and list.
   - Replace hidden hover move/delete actions with visible compact icon buttons.
   - Scope keyboard shortcut to dialog/form.
4. Shared model picker:
   - Search by name, ID, provider.
   - Show provider icon/label and model ID.
5. Replace mojibake text with plain ASCII or valid Unicode where existing file supports it.

## Todo List
- [x] Build alias dialog with model picker
- [x] Build combo dialog cleanup
- [x] Split combo step builder
- [x] Add pinned target formatter
- [x] Fix visible actions
- [x] Fix color semantics

## Success Criteria
- Creating an alias does not require memorizing exact model IDs.
- Combo steps are understandable before saving.
- Pinned accounts display as model + account, not raw opaque strings.
- No file remains unnecessarily above size guidance unless it is a screen orchestrator.

## Risk Assessment
- Combo editing can break serialized model strings. Add focused tests or manual cases:
  - `oai/gpt-4`
  - `kr/model@conn-id`
  - repeated same model with different accounts.

## Security Considerations
- Do not display account secrets.
- Delete confirmation must describe impact without exposing sensitive config.

## Next Steps
- Cleanup, validation, docs.

## Unresolved Questions
- None.
