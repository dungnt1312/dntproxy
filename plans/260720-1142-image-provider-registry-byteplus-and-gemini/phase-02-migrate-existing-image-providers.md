---
title: "Phase 2: Migrate Existing Image Providers"
status: completed
---

# Phase 2: Migrate Existing Image Providers

## Overview

Extract provider-specific code from the 892-line HTTP handler into OpenAI, xAI,
and MiniMax image adapters, preserving all current behavior.

## Requirements

- [x] OpenAI-compatible generation/edit and multipart behavior remain unchanged.
- [x] Codex OAuth streaming remains available through the optional streaming interface.
- [x] xAI generation/edit payloads and error mapping remain unchanged.
- [x] MiniMax prompt validation, timeout, input validation, and error mapping remain unchanged.

## Implementation Steps

1. Add image provider files under `internal/adapter/openai`, `xai`, and `minimax`.
2. Move translators/parsers/HTTP calls out of `image-handler.go`.
3. Register all three adapters in `main.go`.
4. Refactor handlers to resolve auth/tenant/pinned account/logging, validate capability, then dispatch.
5. Move/update existing handler tests to adapter tests and retain endpoint regression tests.

## Todo

- [x] Remove MiniMax/xAI/OpenAI provider-name dispatch branches.
- [x] Retain existing status codes and OpenAI error envelope.
- [x] Retain request log finalization and upstream error-body redaction.

## Success Criteria

Existing image tests pass with all three providers reached through the registry.

## Risk and Rollback

Highest risk is Codex SSE/multipart regression. Fixture-test partial, final,
`[DONE]`, cancellation, JSON and multipart before deleting handler helpers.
Rollback is registration-level: retain/re-enable legacy handler dispatch while
leaving extracted adapters unused.
