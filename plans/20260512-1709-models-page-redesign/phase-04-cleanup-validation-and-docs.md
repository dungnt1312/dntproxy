# Phase 04 - Cleanup Validation And Docs

## Context Links
- `ui/src/App.tsx`
- `ui/src/pages/models.tsx`
- `docs/project-changelog.md`
- `docs/project-roadmap.md`
- `docs/code-standards.md`

## Overview
- Priority: Medium
- Status: Complete
- Remove old UI drift, verify behavior, and document the redesign.

## Key Insights
- Dead legacy pages create confusion during review and future edits.
- Visual redesign must be checked in browser, not only by build.
- Docs management rules require changelog/roadmap updates for significant UI work.

## Requirements
- Remove or clearly retire unused `ui/src/pages/models.tsx`.
- Remove imports/styles only when proven unused.
- Run build.
- Run browser checks for desktop and mobile.
- Update docs/changelog.

## Architecture
- Keep route at `/models`.
- Keep `ModelsScreen` as only active implementation.
- Use screenshots/manual checklist for final UX verification.

## Related Code Files
- Modify:
  - `docs/project-changelog.md`
  - `docs/project-roadmap.md` if roadmap status changes
  - `docs/ux-ui-improvement-summary.md` or create a focused note if preferred
- Delete if unused:
  - `ui/src/pages/models.tsx`
- Verify:
  - `ui/src/App.tsx`
  - `ui/src/pages/*` references

## Implementation Steps
1. Use `rg` to confirm legacy models page has no route/import.
2. Delete legacy page only if no dependency remains.
3. Run `cd ui && bun run build`.
4. Start dev server and inspect:
   - desktop width
   - mobile width
   - dark/light themes if available.
5. Manual UX cases:
   - load success
   - load failure
   - registry search
   - no search results
   - alias create/delete
   - combo create/edit/delete
   - pinned account display
   - model log modal
   - model test status.
6. Update docs with concise changelog entry and any roadmap note.

## Todo List
- [x] Remove dead legacy page
- [x] Run build
- [x] Run browser visual checks
- [x] Validate core workflows
- [x] Update changelog/docs
- [x] Request final code review

## Success Criteria
- Build passes.
- No dead active/legacy duplicate models page remains.
- UI is consistent with shadcn components and app layout.
- No mojibake text remains on redesigned surface.
- Docs reflect shipped change.

## Risk Assessment
- Removing legacy files can break hidden imports. Verify by build and `rg`.
- Browser visual checks can reveal spacing overflow not caught by build.

## Security Considerations
- Ensure screenshots/log modal do not expose secrets in docs.

## Next Steps
- After implementation, delegate code review and tester per workflow.

## Unresolved Questions
- None.
