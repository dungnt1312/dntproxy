---
title: "Image Provider Registry, BytePlus, and Gemini"
description: "Capability-driven image provider architecture plus BytePlus Seedream and Gemini native image generation/editing."
status: completed
priority: P1
effort: "large"
tags: [backend, frontend, image, providers]
created: 2026-07-20
---

# Image Provider Registry, BytePlus, and Gemini

## Overview

Replace provider-name branching in the OpenAI-compatible image handlers with a
dedicated image-provider registry. Migrate the existing OpenAI, xAI, and MiniMax
implementations without changing public behavior, then add BytePlus Seedream
and Gemini native image generation/editing. Expose structured capabilities to
the playground so it only presents inputs supported by the selected model.

## Goals

| # | Goal | Priority |
|---|------|----------|
| 1 | Add a separate `ImageProvider` contract and thread-safe registry | P0 |
| 2 | Migrate OpenAI, xAI, and MiniMax behind the registry | P0 |
| 3 | Add BytePlus Seedream generation/editing | P1 |
| 4 | Add Gemini native image generation/editing | P1 |
| 5 | Drive playground controls from model capabilities | P1 |

## Phases

| # | Phase | Status |
|---|-------|--------|
| 1 | [Foundation and registry](./phase-01-start.md) | Completed |
| 2 | [Migrate existing providers](./phase-02-migrate-existing-image-providers.md) | Completed |
| 3 | [Integrate BytePlus Seedream](./phase-03-integrate-byteplus-seedream.md) | Completed |
| 4 | [Integrate Gemini native images](./phase-04-integrate-gemini-native-images.md) | Completed |
| 5 | [Capability UI, verification, deploy](./phase-05-capability-driven-ui-verification-and-deploy.md) | Completed |

## Success Criteria

- [x] `/v1/images/generations` and `/v1/images/edits` retain their current OpenAI-compatible response/error envelope.
- [x] OpenAI, xAI, MiniMax, BytePlus, and Gemini are dispatched only through `ImageProviderRegistry`.
- [x] Existing OpenAI/Codex streaming and OpenAI multipart edit behavior pass regression tests.
- [x] BytePlus and Gemini support text-to-image and image-guided edit through JSON requests.
- [x] `/v1/models?type=image` exposes structured per-model image capabilities.
- [x] Playground enables edit/upload/multi-reference controls from capabilities, not provider names.
- [x] Tenant/API-key policy, pinned connections, account selection, logging, and body redaction are preserved.
- [x] Go tests, vet/build, UI build, local install, PM2 restart, and health check succeed.

## Guardrails

- Patch the dirty worktree surgically; do not reset or rewrite unrelated changes.
- Keep provider-specific wire payloads and response parsing inside adapter packages.
- Runtime `ImageProviderRegistry` capabilities are authoritative for handler
  validation and `/v1/models`; persisted model capabilities remain a coarse
  discovery filter. Runtime metadata wins when both exist. Unknown/custom models
  use the provider's conservative default capabilities.
- BytePlus is image-only: do not register a fake chat executor. Connection testing
  must use a non-generative auth/model-list probe; never create a billed image.
- Preserve the 10 MB handler body limit, MiniMax's provider-specific limits, and
  current public status/error mapping.
- Any server-side image URL fetch must use the shared safe downloader: HTTP(S)
  only; validate DNS/IP on every redirect; block loopback/private/link-local and
  cloud metadata ranges; cap redirects, timeout, encoded/decoded per-image and
  aggregate bytes; verify MIME and magic bytes; redact signed URL query strings.
- Account selection/pinning stays in HTTP/service orchestration. Adapters receive
  only context, selected credentials, normalized request input, and logger.

## Implemented Flow

`apiKeyMiddleware` authenticates and injects tenant/model/connection policy
before the image routes. Each handler parses `provider/model@connection`,
checks runtime capabilities, selects tenant-scoped credentials, creates a
request log, and dispatches through `ImageProviderRegistry`. Provider-specific
payload and transport code now lives in its adapter package.

```text
JSON/multipart -> auth middleware -> model/@connection parse -> tenant/policy
-> account select -> image registry -> capability validation -> adapter
-> bounded provider transport/parser -> OpenAI response/error
```

Codex streaming uses an optional image-provider interface; Gin remains
responsible for SSE headers, keepalive, cancellation, final frame and `[DONE]`.
Registry/adapters are process-lifetime singletons, immutable after startup, and
must not retain request, credential, tenant, or image state.

## Dependencies and Execution

```text
Foundation -> existing-provider adapters
Foundation -> BytePlus adapter
Foundation -> Gemini adapter
existing + BytePlus + Gemini -> HTTP/UI integration -> deploy
```

The registry is constructed and populated in `cmd/dntproxy/main.go`, then
passed to the Gin router.

## Compatibility and Migration

- Route paths, the 10 MiB handler body limit, OpenAI error envelope, and
  response shape remain unchanged.
- API-key policy and pinned-account selection remain in HTTP orchestration.
- Codex OAuth sync/SSE/multipart behavior remains in the OpenAI image adapter.
- MiniMax prompt/reference/business limits remain in the MiniMax adapter and
  its tests.
- Add `image_capabilities` to model objects without removing current
  `capabilities` (`internal/adapter/http/models-handler.go:13-83`).
- `openai-compatible` continues to use the OpenAI image adapter.

## Test Matrix

| Level | Required evidence |
|---|---|
| Unit | registry unknown/concurrent lookup; capability serialization; every translator/parser/error mapper; byte/reference/format limits |
| Adapter contract | five providers via `httptest`: generate, edit, non-2xx, malformed/oversize response, cancellation/timeout, redacted logging |
| HTTP integration | JSON + multipart, tenant policy deny, pinned allow/deny, unsupported capability, stable status/error/response |
| Regression | Codex partial/final/DONE SSE and multipart; xAI generate/edit; MiniMax generation/edit/business errors |
| UI | metadata mapping; edit/mask/multi-ref visibility and limits; alert/toast |
| Release | `go test ./...`, `go vet ./...`, Go build, UI build, install, PM2 online, `/health` |

Live BytePlus/Gemini calls are conditional on credentials; deterministic
contract tests are never optional.

No BytePlus or Gemini production credential was available during this work, so
live provider calls were skipped. Adapter contract tests cover both native
protocols; local PM2 health returned HTTP 200 after deployment.

## Failure Modes, Risk, and Rollback

| Failure | L/I | Mitigation | Rollback |
|---|---|---|---|
| Dirty-worktree overwrite | High/High | Re-read before patch; focused diffs; never reset/checkout | Revert only plan-owned lines/new files |
| Registry migration alters legacy output | Med/High | Characterization tests before extraction | Re-register legacy adapters/restore old dispatch |
| Base64/signed URL leaks | Med/High | Existing body sanitizer + explicit redaction tests | Disable new provider registration |
| Provider schema/model drift | Med/High | Isolated translators; conservative unknown-model capabilities | Remove affected registration/models |
| Memory/timeout exhaustion | Med/High | encoded/decoded limits, bounded reads, context-aware dedicated clients | Disable provider; old providers remain |
| UI advertises auth-dependent feature | Med/Med | requirements in capability metadata; server validates authoritatively | Hide new UI controls without API rollback |

No persisted-data migration is required. Removing a new provider from startup
registration and model/provider catalogs rolls back that integration without
touching existing connections or image routes.

## Parallel Ownership

- Foundation/root exclusively owns shared ports/domain/provider registry,
  `main.go`, router/handlers, provider config, model definitions, model metadata,
  safe downloader, and UI.
- BytePlus work owns only `internal/adapter/byteplus/**`.
- Gemini work owns only `internal/adapter/gemini/**`.
- Existing-provider migration owns only image adapter files under OpenAI/xAI/
  MiniMax after foundation boundaries are established.

No two parallel workers edit the same file. `main.go`, handler/router, provider
config, model definitions, shared downloader, model-list integration, UI and
their integration tests remain foundation/root-owned.

## Out of Scope

Items 4–6 from the research roadmap are deferred: BFL/Qwen/Together/Stability,
async job APIs, and specialist mask/inpainting workflows.

## Open Questions

- Live API keys may be unavailable; report live smoke as skipped, never as pass.
- BytePlus account-specific endpoint/model IDs must come from the connection;
  do not hard-code an unverified endpoint ID.

<!-- slug: image-provider-registry-byteplus-and-gemini -->
