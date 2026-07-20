---
title: "MiniMax Image MVP Completion"
type: project-status-report
status: completed
created: 2026-07-20T09:51:00+07:00
scope: minimax-image
---

# Plan Complete: MiniMax Image MVP

## Summary

| Item | Result |
|---|---|
| Public API | `POST /v1/images/generations` |
| Provider model | `minimax/image-01` |
| Native upstream | `POST /v1/image_generation` |
| Output formats | URL, Base64 |
| Review | 9/10, no blockers |
| Full Go tests | Pass |
| Race tests | Pass for changed packages |
| Go vet/build | Pass |
| Frontend production build | Pass |

## Work Completed

- [x] MiniMax OpenAI-to-native request translator.
- [x] Prompt, count, aspect-ratio, dimension, and response-format validation.
- [x] URL/Base64 native response conversion.
- [x] `base_resp` business-error validation and HTTP mapping.
- [x] Partial batch rejection and response-size bound.
- [x] Client cancellation propagation to upstream MiniMax request.
- [x] `minimax/image-01` registry capability and new-connection default.
- [x] Tenant, model, connection, and pinned-account policy enforcement.
- [x] Focused translator, transport, policy, registry, race, and regression tests.

## Security and Reliability Fixes

- Closed pinned-connection API-key allowlist bypass in the shared image credential helper.
- Reject missing MiniMax response envelope and partial batch failures.
- Preserve real upstream HTTP status for unknown MiniMax business codes.
- Return OpenAI-compatible `invalid_request_error` for local validation failures.

## Known Limitations

- MiniMax image-to-image/reference generation is not enabled.
- Image routes still select one account; multi-account image retry/cooldown remains follow-up work.
- Existing MiniMax connections may need `image-01` added to `supportedModels`; new connections include it automatically.
- No live MiniMax API smoke test was run because no provider key was supplied.
- Base64 responses are buffered with a 64 MiB wire limit; high concurrency should be monitored.

## Documentation

- Research: `plans/research/260720-0926-minimax-image-integration.md`
- Technical journal: `docs/journals/260720-0951-minimax-image-integration.md`
- Evergreen architecture, codebase, and roadmap docs updated in the same session.

## Next Steps

1. Run one live URL and one Base64 request with a funded MiniMax key.
2. Add `image-01` to existing MiniMax connection model lists where required.
3. Implement image-route multi-account retry/cooldown.
4. Live-probe reference-image formats before enabling `/v1/images/edits`.

## Unresolved Questions

- Whether MiniMax image references accept Base64 Data URLs in production.
- Whether `image-01-live` is generally available or account-gated.

