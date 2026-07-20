---
date: 2026-07-20 10:29
severity: medium
component: minimax-image-editing
status: resolved
---

# MiniMax Image Editing Was Missing Behind a Working Route

**Date**: 2026-07-20 10:29  
**Severity**: Medium  
**Component**: MiniMax image editing  
**Status**: Resolved

## What Happened

`POST /v1/images/edits` existed, but MiniMax requests terminated with `image editing not supported for provider: minimax`. We had shipped image generation and left editing on the generic unsupported-provider branch in `internal/adapter/http/image-handler.go`. The fix adds a JSON edit path that maps one reference image into MiniMax `subject_reference`.

## The Brutal Truth

This was irritating because the route and playground implied a broader capability than the backend actually implemented. The failure was not subtle or upstream instability; we simply stopped at generation and did not complete the provider integration. That creates exactly the kind of misleading “almost works” feature that wastes debugging time at the API boundary.

## Technical Details

MiniMax edit now accepts JSON with a prompt and exactly one `image`, either an HTTP(S) PNG/JPEG URL or a `data:image/png;base64,...` / `data:image/jpeg;base64,...` URL. It rejects masks, multipart bodies, unsupported media, and multiple references rather than pretending MiniMax supports OpenAI edit semantics.

The translator emits:

```json
{"subject_reference":[{"type":"character","image_file":"..."}]}
```

Request logging now redacts `image`, `images`, `mask`, `subject_reference`, and `image_file`, preventing full Base64 payloads from landing in logs. The playground enforces the same one-image, PNG/JPEG, 7 MB, no-mask constraints before sending.

Live validation completed in 18.7 seconds with an image URL and 26.4 seconds with a data URL. Focused tests, the full Go suite, vet, build, UI build, and review passed. `install-local.sh` completed and PM2 restarted `dntproxy` successfully.

## What We Tried

The original fallback returned the unsupported-provider error. We rejected multipart conversion because MiniMax’s contract is JSON `subject_reference`, and rejected mask emulation because it would silently promise behavior the provider does not offer.

## Root Cause Analysis

We implemented `/v1/images/generations` first and reused an edit dispatcher whose MiniMax branch never existed. The missing acceptance test allowed the public route and UI to drift from provider capability.

## Lessons Learned

Provider support must be checked per operation, not per model name. Base64 also expands binary data by roughly 33%; a 7 MB file becomes about 9.3 MB before JSON overhead, so request limits, memory, and logging must be designed around encoded size.

## Next Steps

- Backend owner: keep URL and data-URL edit cases in regression tests on every image change.
- Frontend owner: preserve matching validation and explicit unsupported-mask messaging.
- Maintainer: monitor edit latency and payload sizes in the next release; revisit the 7 MB client limit only with measured memory data.
