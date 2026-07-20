---
title: "Phase 3: Integrate BytePlus Seedream"
status: completed
---

# Phase 3: Integrate BytePlus Seedream

## Overview

Add BytePlus ModelArk Seedream as an image-only provider using
`POST /api/v3/images/generations`; edit requests use the same endpoint with the
`image` field.

## Requirements

- [x] Bearer API-key auth and configurable regional base URL.
- [x] Text generation plus one/many reference-image editing via URL or data URI.
- [x] Map OpenAI size/response format to supported BytePlus fields and reject unsupported inputs clearly.
- [x] Parse the official `data[].url` contract; only emit `b64_json` when the
  official API returns it or a bounded safe download is explicitly performed.

## Implementation Steps

1. Add BytePlus provider config/aliases and recommended Seedream models; store
   base URL exactly once as `https://ark.ap-southeast.bytepluses.com/api/v3`
   and safely append `/images/generations`.
2. Add `internal/adapter/byteplus` translator, client/provider, parser, and tests.
3. Register only in the image registry.
4. Make connection testing use a non-generative ModelArk model-list/auth probe
   when a provider has no chat executor; wire required metadata through management handlers.
5. Add model capabilities including multi-reference limits and supported input formats.

## Todo

- [x] No fake `ProviderExecutor` is created.
- [x] AP base defaults to `https://ark.ap-southeast.bytepluses.com/api/v3`.
- [x] Provider errors are sanitized before returning/logging.
- [x] Official request/response fixtures define whether `b64_json` is native;
  no undocumented response field is assumed.

## Success Criteria

Golden request/response tests cover generation, single edit, multi-reference,
official URL output, optional safe URL-to-base64 conversion, invalid input, and
upstream errors.

## Risk and Rollback

Model IDs and limits vary by account. Pass endpoint IDs through, use conservative
unknown-model capabilities, and do not claim unsupported output formats.
Rollback by removing BytePlus startup/catalog registration; other adapters and
routes remain intact.
