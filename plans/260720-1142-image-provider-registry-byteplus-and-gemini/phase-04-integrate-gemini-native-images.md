---
title: "Phase 4: Integrate Gemini Native Images"
status: completed
---

# Phase 4: Integrate Gemini Native Images

## Overview

Reuse Gemini API-key connections but call the native
`models/{model}:generateContent` API for image generation/editing instead of the
OpenAI-compatible chat endpoint.

## Requirements

- [x] Send prompt plus optional reference images as `contents[].parts`.
- [x] Use `generationConfig.responseModalities=["IMAGE"]` and map supported aspect ratio/size.
- [x] Accept URL/data-URI references, normalize them to `inline_data`, and enforce
  the shared SSRF, redirect, timeout, decoded-size, aggregate-size, MIME, and
  magic-byte controls.
- [x] Parse non-thought image parts from candidates into base64 OpenAI image results.

## Implementation Steps

1. Add `internal/adapter/gemini` translator, fetch/normalization helper, provider, parser, and tests.
2. Register Gemini image models/capabilities and image adapter.
3. Preserve existing Gemini chat registration.
4. Map safety/API errors into the standard error envelope without exposing credentials or raw image bodies.

## Todo

- [x] Support current image model IDs from the research report.
- [x] Skip thought/intermediate parts and return final non-thought images.
- [x] Return a deterministic error when no image part is present.
- [x] Signed input URL query strings never appear in logs or returned errors.

## Success Criteria

Golden tests cover text-to-image, one/many reference edit, size mapping,
base64 parsing, no-image responses, and upstream errors.

## Risk and Rollback

Gemini may return safety/text parts without a final image. Scan all candidates,
ignore thought/intermediate parts, and return a stable bounded error when no
image exists. Rollback only Gemini image registration; existing Gemini chat
registration is untouched.
