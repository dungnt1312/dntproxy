# Models Page Redesign Plan

## Context
- Active route `/models` renders `ui/src/components/screens/models-screen.tsx`, not legacy `ui/src/pages/models.tsx`.
- Current design has useful tabs but weak hierarchy, hidden actions, mixed color semantics, weak error states, and model registry scan issues.
- Follow shadcn/ui, Tailwind tokens, `goApi`, provider registry, file size limits.

## Phases

| Phase | Status | Goal |
|---|---:|---|
| [Phase 01 - UX Contract](phase-01-ux-contract-and-data-hardening.md) | Complete | Defined final IA and fixed data/error assumptions before visual work |
| [Phase 02 - Registry Redesign](phase-02-registry-redesign.md) | Complete | Made model registry compact, searchable, sortable, provider-aware |
| [Phase 03 - Routing Workflow Redesign](phase-03-aliases-and-combos-workflow-redesign.md) | Complete | Redesigned aliases/combos as clear config workflows with validation |
| [Phase 04 - Cleanup Validation Docs](phase-04-cleanup-validation-and-docs.md) | Complete | Removed legacy drift, verified build/UI, updated docs/changelog |

## Key Dependencies
- Phase 02 depends on Phase 01 model normalization.
- Phase 03 depends on shared picker/toolbar decisions from Phase 01.
- Phase 04 depends on all UI changes being complete.

## Target UX
- Keep `Registry / Aliases / Combos` tabs.
- Registry is browse/inspect/test, not decorative cards.
- Aliases and combos are configuration surfaces with explicit create/edit/delete flows.
- Actions are discoverable, keyboard/touch usable, and consistent.
- Empty, loading, error, and search-no-result states must differ clearly.

## Non-Goals
- No backend API redesign unless a UI blocker is found.
- No new design system beyond existing shadcn/ui components.
- No large dashboard/playground redesign in this plan.

## Validation
- `cd ui && .\node_modules\.bin\tsc.exe -b --pretty false` passed.
- `cd ui && .\node_modules\.bin\vite.exe build` passed.
- Manual desktop check from user screenshot led to visual polish of table rows, connection pills, status, and actions.
- Core workflows covered in implementation: registry search/test/log, alias create/delete, combo create/edit/delete.

## Unresolved Questions
- None.
