---
date: 2026-07-20 09:51
session: minimax-image-integration
severity: medium
component: image-generation
status: resolved
---

# MiniMax image-01 MVP

## Context

We added MiniMax `image-01` to the existing OpenAI-compatible `POST /v1/images/generations` route. The goal was a narrow text-to-image MVP that reused dntproxy routing, credentials, logging, and response shapes instead of inventing another public API.

## What Changed

- Added `internal/adapter/minimax/image-translator.go` to translate OpenAI image fields into MiniMax `POST /v1/image_generation`, validate prompt length, `n`, aspect ratio/dimensions, and normalize `url`/`b64_json`.
- Parsed both `data.image_urls` and `data.image_base64`, while treating `base_resp.status_code` as authoritative even when upstream HTTP is 200.
- Registered `minimax/image-01` with `image-generation` capability, `$0.0035` per-image metadata, nine-image maximum, and 1,500-character prompt limit.
- Preserved `minimax/image-01@connection-id` pinning and tenant-aware request logging.

## Review Findings and Fixes

The first pass was functionally plausible but not safe enough. Review found three concrete holes:

1. Image routing bypassed API-key model and connection policy. `selectImageCredentials` now checks `ModelAllowedByPolicy` and `ConnectionAllowed`; violations return 403 instead of silently using forbidden credentials.
2. The MiniMax request did not inherit the client request context. `http.NewRequestWithContext` now stops the upstream call on disconnect; the regression test asserts the returned error contains `context canceled`.
3. A response with `metadata.failed_count > 0` could expose a partial batch as success. `ParseImageResponse` now fails the entire request with `MiniMax image batch partially failed: N image(s) failed`.

The brutal truth: these were not cosmetic review comments. Shipping the initial version would have created an authorization bypass, wasted paid generations after clients disconnected, and lied about batch success. Catching all three before release was a relief, but it is frustrating that the image path had drifted from the policy guarantees already expected elsewhere.

## Decisions Made

| Decision | Rejected alternative | Why |
|---|---|---|
| Reuse `/v1/images/generations` | Add MiniMax-specific public route | Keeps clients OpenAI-compatible |
| Fail partial batches atomically | Return successful subset | Avoids ambiguous billing and silent data loss |
| Ship generation only | Emulate edits with reference images | MiniMax reference semantics are not OpenAI edit/inpainting semantics |

## Verification

`go test ./internal/adapter/minimax ./internal/adapter/http ./internal/domain` passed. Coverage includes translation validation, business-code mapping, missing envelopes/output, partial batches, cancellation, API-key policy, endpoint/auth headers, and model registration.

## Known Limitations

- No MiniMax image edits.
- No multi-account fallback within one image request.
- No smoke test against a live MiniMax API key; all upstream tests use `httptest`.

## Next Steps

- Backend owner: add account retry/fallback with cooldown semantics before calling this production-resilient.
- Integration owner: run one low-cost live-key smoke test before release and record the exact response envelope.
- Product/API owner: design a separate reference-image contract before adding MiniMax edit-like behavior.

AgentWiki publish skipped: no AgentWiki CLI or MCP capability was available in this session.
