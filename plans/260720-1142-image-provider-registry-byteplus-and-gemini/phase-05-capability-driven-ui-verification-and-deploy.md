---
title: "Phase 5: Capability-Driven UI, Verification, and Deploy"
status: completed
---

# Phase 5: Capability-Driven UI, Verification, and Deploy

## Overview

Expose runtime capabilities through the model list, consume them in the
playground, then run regression/build/deployment checks.

## Requirements

- [x] `image_capabilities` is additive and backward-compatible.
- [x] Playground hides/disables edit/upload/multi-reference options when unsupported.
- [x] Existing MiniMax alert/error behavior remains visible.
- [x] No secrets or base64 payloads appear in logs.

## Implementation Steps

1. Extend `modelObject` and `ui/src/lib/go-api.ts` types.
2. Refactor `image-generator.tsx` to use capabilities rather than provider names.
3. Run focused adapter/handler/domain tests, then `go test ./...`, `go vet ./...`, and Go build.
4. Run UI lint/type/build commands available in `ui/package.json`.
5. Run `install-local.sh`, restart PM2, verify `/health`, model metadata, and sample calls when credentials exist.
6. Request code review and update docs/plan status.

## Todo

- [x] Generation-only models cannot submit edits.
- [x] Single-reference models cap uploads at one; multi-reference models expose their max.
- [x] Provider errors surface as playground alerts.

## Success Criteria

Go tests/vet/build, UI type/build, and focused lint for the modified Image
Playground pass; PM2 reports online and health returns 200. Full-repository UI
lint remains under a documented baseline waiver (222 pre-existing errors and 6
warnings outside this feature's scope). Live provider calls are reported as
tested or explicitly skipped because credentials are absent.

## Risk and Rollback

This phase touches dirty shared files. Re-read each before editing, preserve
unrelated hunks, and inspect focused diffs. Roll back by unregistering new
providers and restoring legacy handler wiring; additive model metadata and
unused adapters are harmless.
