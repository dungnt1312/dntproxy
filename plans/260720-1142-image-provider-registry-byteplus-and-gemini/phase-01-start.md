---
title: "Phase 1: Foundation and Registry"
status: completed
---

# Phase 1: Start

## Overview

Create an image-specific port and registry independent of the chat
`ProviderExecutor`. Keep HTTP concerns in the handler and provider wire concerns
in adapters.

## Requirements

- [x] Define `ImageCapabilities` with generate/edit/multipart/mask/multi-reference/streaming, limits, formats, and response formats.
- [x] Define generation/edit request envelopes that carry normalized model, JSON or multipart input, selected credentials, context, and request logger.
- [x] Define `ImageProvider` plus the optional streaming interface.
- [x] Add a thread-safe registry with register/get/capabilities methods.

## Implementation Steps

1. Add `internal/domain/image-capabilities.go`.
2. Add `internal/port/image-provider.go`.
3. Add `internal/adapter/provider/image-registry.go` and unit tests.
4. Pass the registry from `cmd/dntproxy/main.go` through `NewRouter`.
5. Update the models handler to include `image_capabilities`.
6. Add a bounded SSRF-safe image input loader shared by native adapters.

## Todo

- [x] Registry lookup is independent from chat providers.
- [x] Unknown provider/model returns a stable unsupported-image error.
- [x] Capabilities can vary by model within one provider.
- [x] Runtime registry metadata overrides persisted coarse capabilities.
- [x] Server-loaded Base64 and URL images enforce decoded limits and MIME/content validation; provider-specific aggregate limits are applied by the Gemini adapter.

## Success Criteria

Unit tests prove concurrent-safe registration/lookup and capability serialization.

## Risk and Rollback

Risk: coupling the port to Gin or storing mutable request state in singleton
adapters. Keep the boundary limited to standard-library/domain/port types and
make all request data method-local. Rollback by removing the unused new port and
registry before Phase 2 wiring.
