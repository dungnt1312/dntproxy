---
title: "MiniMax Image Integration Research"
type: research-report
status: complete
created: 2026-07-20T09:26:00+07:00
updated: 2026-07-20T10:29:14+07:00
scope: dntproxy
---

# Research Report: MiniMax Image Integration

## Table of Contents

- [Executive Summary](#executive-summary)
- [Research Scope and Method](#research-scope-and-method)
- [MiniMax Image API](#minimax-image-api)
- [Current dntproxy Readiness](#current-dntproxy-readiness)
- [Compatibility Mapping](#compatibility-mapping)
- [Recommended Architecture](#recommended-architecture)
- [Error, Fallback, and Security Policy](#error-fallback-and-security-policy)
- [Implementation Plan](#implementation-plan)
- [Test Matrix](#test-matrix)
- [Risks and Decisions](#risks-and-decisions)
- [References](#references)
- [Unresolved Questions](#unresolved-questions)

## Executive Summary

Recommendation: integrate MiniMax `image-01` behind dntproxy's existing OpenAI-compatible `POST /v1/images/generations`. Do not create a new public MiniMax-specific endpoint. The repository already has image routing, account selection, `@connectionId` pinning, model capability filtering, an OpenAI response builder, and an Image Playground. Required backend work is narrow: add a MiniMax request/response translator, dispatch MiniMax credentials to it, register `minimax/image-01`, and test error mapping.

Text-to-image shipped first. Phase 2 now implements MiniMax image-to-image through JSON `POST /v1/images/edits`: exactly one PNG/JPEG HTTP(S) URL or Base64 data URL is mapped to `subject_reference` with type `character`. This remains identity/character preservation rather than a general OpenAI mask/inpainting API; masks, multiple references, local file paths, and multipart MiniMax edits are rejected.

MiniMax image generation is synchronous. No polling or SSE is needed. Official limits at research time: prompt up to 1,500 characters, `n=1..9`, `image-01` at 10 RPM, URL results expire after 24 hours, and pay-as-you-go price is USD 0.0035/image.

## Research Scope and Method

- Research date: 2026-07-20, Asia/Bangkok.
- Recency target: current official API behavior and pricing.
- External sources: MiniMax official API guide, T2I/I2I reference, rate limits, pricing, and error reference.
- Repository review: image handler, image domain types, model registry/filtering, provider config, account selection, xAI translator pattern, and Image Playground.
- Boundary: research and implementation design only; no production API call because no MiniMax key was provided.
- Principles: reuse existing OpenAI-compatible surface; avoid new abstraction until a second non-OpenAI image protocol needs it.

## MiniMax Image API

### Endpoint and Authentication

```http
POST https://api.minimax.io/v1/image_generation
Authorization: Bearer ${MINIMAX_API_KEY}
Content-Type: application/json
```

Important: MiniMax uses `/v1/image_generation`, not OpenAI's `/v1/images/generations`.

### Supported Model

Use `image-01` for the initial integration.

MiniMax's T2I reference lists only `image-01`. Its I2I reference also mentions `image-01-live`, but the API overview describes only `image-01`. Do not advertise `image-01-live` until verified with an account entitled to use it.

### Native Request

```json
{
  "model": "image-01",
  "prompt": "A cinematic portrait in soft window light",
  "width": 1024,
  "height": 1024,
  "response_format": "base64",
  "n": 1,
  "seed": 42,
  "prompt_optimizer": false
}
```

Constraints:

| Field | MiniMax contract |
|---|---|
| `prompt` | Required, max 1,500 characters |
| `n` | 1-9, default 1 |
| `aspect_ratio` | `1:1`, `16:9`, `4:3`, `3:2`, `2:3`, `3:4`, `9:16`, `21:9` |
| `width`, `height` | Both required together; 512-2048; divisible by 8; `image-01` only |
| `response_format` | `url` or `base64`; default `url` |
| `seed` | Optional signed 64-bit integer |
| `prompt_optimizer` | Optional boolean; documented default `false` |

If both `aspect_ratio` and width/height exist, MiniMax prioritizes `aspect_ratio`.

### Native Response

URL response:

```json
{
  "id": "trace-id",
  "data": {
    "image_urls": ["https://..."]
  },
  "metadata": {
    "failed_count": "0",
    "success_count": "1"
  },
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

Base64 response uses `data.image_base64`.

`base_resp.status_code` must be checked even when HTTP status is 200. Non-zero means failure. `metadata.failed_count` must also be observed: a batch may be partially successful.

### Cost and Limits

| Item | Official value at research time |
|---|---|
| Price | USD 0.0035 per generated image |
| Rate limit | 10 requests/minute for `image-01` |
| URL lifetime | 24 hours |
| Batch size | Up to 9 images/request |

## Current dntproxy Readiness

Already available:

- `POST /v1/images/generations` and `/v1/images/edits`.
- OpenAI image request/response domain types.
- Provider-qualified model parsing: `minimax/image-01`.
- Pinned account parsing: `minimax/image-01@connectionId`.
- Tenant and API-key policy checks.
- Account credential selection.
- `GET /v1/models?type=image` based on `image-generation` capability.
- Image Playground with model, size, count, format, generate, and edit controls.
- xAI request translator provides a close implementation pattern.

Implemented:

- MiniMax dispatch for generation and JSON edit requests.
- MiniMax generation/edit translator, including `subject_reference` mapping.
- MiniMax response parser and `base_resp` validation.
- `minimax/image-01` registry entry with `image-generation`.
- Default/recommended connection exposure for `image-01`.
- MiniMax translator and handler tests.
- Model-aware Playground edit controls: one PNG/JPEG reference, maximum 7 MB, no mask.

Still missing:

- Image request fallback across multiple accounts. Current image handler selects one account and does not use the chat service's retry chain.

## Compatibility Mapping

### OpenAI-Compatible Input to MiniMax

| dntproxy/OpenAI field | MiniMax field | Rule |
|---|---|---|
| `model=minimax/image-01` | `model=image-01` | Strip provider and optional account suffix |
| `prompt` | `prompt` | Trim; reject empty or over 1,500 chars |
| `n` | `n` | Default 1; reject outside 1-9 |
| `size=WxH` | `width`, `height` | Preserve exact dimensions when both are 512-2048 and divisible by 8 |
| empty/`auto` size | `aspect_ratio=1:1` | Deterministic default |
| `response_format=b64_json` | `response_format=base64` | Required name translation |
| `response_format=url` | `response_format=url` | URL expires in 24 hours |
| `quality` | none | Ignore for compatibility; do not forward |
| `style` | none | Ignore for compatibility; do not forward |
| `user` | none | Do not forward |

Allow MiniMax extensions in the JSON body:

```json
{
  "aspect_ratio": "21:9",
  "seed": 42,
  "prompt_optimizer": true
}
```

Precedence:

1. Explicit valid `aspect_ratio`.
2. Explicit valid `width` + `height`.
3. Parsed OpenAI `size`.
4. Default `1:1`.

Reject contradictory or invalid values locally with OpenAI-style `400 invalid_request_error`.

### MiniMax Output to OpenAI-Compatible Output

| MiniMax | dntproxy |
|---|---|
| `data.image_urls[i]` | `data[i].url` |
| `data.image_base64[i]` | `data[i].b64_json` |
| `id` | log/upstream trace metadata; not part of current public image response |
| `base_resp.status_msg` | OpenAI-style error message on failure |
| current Unix time | top-level `created` |

MiniMax does not return a revised prompt. Leave `revised_prompt` absent.

### Image-to-Image

Native MiniMax shape:

```json
{
  "model": "image-01",
  "prompt": "Keep the same person, change the scene to a library",
  "subject_reference": [
    {
      "type": "character",
      "image_file": "https://public.example/reference.jpg"
    }
  ]
}
```

Implemented phase-2 mapping:

- Accept JSON `/v1/images/edits`; MiniMax multipart edits are unsupported.
- Map exactly one `images[0].image_url` or `image` to one `subject_reference`.
- Reject masks: MiniMax has no documented mask/inpainting equivalent.
- Reject more than one reference.
- Do not silently discard extra images or masks.
- Accept PNG/JPEG references as HTTP(S) URLs or valid Base64 data URLs. The Playground accepts one PNG/JPEG upload up to 7 MB and sends it as a data URL.

## Recommended Architecture

Keep protocol translation close to the existing image path:

```text
POST /v1/images/generations
  -> parse minimax/image-01[@connection]
  -> tenant/policy/account selection
  -> MiniMax translator
  -> POST {baseURL}/v1/image_generation
  -> validate HTTP + base_resp + metadata
  -> []domain.ImageResult
  -> existing OpenAI response builder
```

Recommended files:

| File | Change |
|---|---|
| `internal/adapter/minimax/image-translator.go` | Native request/response structs, validation, mapping |
| `internal/adapter/minimax/image-translator_test.go` | Table-driven translator/parser tests |
| `internal/adapter/http/image-handler.go` | Dispatch MiniMax generation; keep transport/logging integration |
| `internal/adapter/http/image-handler-minimax_test.go` | Upstream HTTP and public response tests |
| `internal/domain/model-definition.go` | Register `minimax/image-01` with `image-generation` |
| `internal/domain/provider-config.go` | Make `image-01` available on new MiniMax connections |
| `ui/src/components/screens/playground/image-generator.tsx` | Optional phase 2: model-aware controls/extensions |

Do not modify `port.ProviderExecutor` for the MVP. Its contract is chat/SSE-oriented, while image generation is synchronous and already handled as a specialized HTTP path. A generic multimodal executor interface is premature unless video/music integration follows.

Use `domain.StripVersionSuffix(creds.BaseURL)` before appending `/v1/image_generation`; this prevents `/v1/v1/image_generation` for custom MiniMax base URLs.

### Model Registration

Suggested registry entry:

```go
"minimax/image-01": {
    ID:              "image-01",
    Name:            "MiniMax Image 01",
    Provider:        "minimax",
    ContextWindow:   1,
    MaxOutputTokens: 1,
    Capabilities:    []string{"image-generation"},
    IsActive:        true,
    Metadata: map[string]interface{}{
        "pricePerImage": 0.0035,
        "maxImages":     9,
        "maxPromptChars": 1500,
    },
},
```

Add `image-01` to the models assigned to new MiniMax connections, while keeping `DefaultTestModel` on a text model. Without connection support, registry metadata alone will not place the image model in the effective tenant pool.

## Error, Fallback, and Security Policy

### MiniMax Business Error Mapping

| MiniMax code | Meaning | dntproxy status | Retry/fallback |
|---|---|---:|---|
| `0` | Success | 200 | No |
| `1001` | Timeout | 504 | Yes |
| `1002` | Rate limit | 429 | Yes; account cooldown |
| `1004`, `2049` | Invalid/unauthorized key | 401 upstream error | Try another account; mark connection unhealthy |
| `1008` | Insufficient balance | 402 | Try another account; long cooldown/manual action |
| `1024`, `1033` | Upstream/system error | 502 | Yes |
| `1026`, `1027` | Sensitive input/output | 400 | No |
| `2013` | Invalid parameters | 400 | No |
| unknown non-zero | Upstream failure | 502 | Conservative retry once on another account |

Never treat HTTP 200 plus non-zero `base_resp.status_code` as success.

### Multi-Account Behavior

Current image routes select only one credential. For parity with chat:

1. Select candidate credentials under tenant, policy, model, and optional pinned connection.
2. Execute once per eligible account.
3. Retry only retryable upstream failures.
4. For `@connectionId`, never switch accounts.
5. Mark cooldown/success using the same semantics as chat.

This can be a follow-up after MVP, but document the limitation if MVP ships first.

### Response Size

Current MiniMax batch max is nine images. A fixed 10 MiB upstream response cap may truncate a valid Base64 batch. Raise the image-only cap conservatively (for example 64 MiB) or decode with a bounded streaming decoder. Keep the public request-body limit separate.

### Security

- Never log API keys or full Base64 image bodies.
- `PrepareLoggedBody` should redact/truncate `image`, `images`, `subject_reference.image_file`, and Base64 results.
- Do not download reference URLs inside dntproxy; forwarding them to MiniMax avoids introducing an SSRF fetcher.
- If future validation fetches URLs, block private/link-local networks, redirects to private networks, and non-HTTP schemes.
- Validate prompt length, batch count, dimensions, response format, and reference count before upstream calls.
- Preserve MiniMax safety errors; do not retry safety-rejected prompts on other accounts.
- Prefer Base64 when clients need durable output because MiniMax URLs expire after 24 hours.

## Implementation Plan

### Phase 1: Text-to-Image MVP

1. Add `minimax/image-01` registry metadata and make it reachable from new/existing MiniMax connections.
2. Implement translator:
   - OpenAI fields to MiniMax fields.
   - MiniMax extension fields.
   - strict local validation.
3. Implement synchronous MiniMax HTTP execution:
   - Bearer auth.
   - base URL normalization.
   - bounded response reading.
   - HTTP and `base_resp` validation.
   - result conversion.
4. Add MiniMax branch before generic unsupported-provider return.
5. Add unit and handler tests.
6. Run Go tests and a manual playground smoke test with a real key.

### Phase 2: Image-to-Image

Status: implemented.

1. Added JSON edit mapping from exactly one `image` or `images[0].image_url` to MiniMax `subject_reference`.
2. Accepted HTTP(S) URLs and PNG/JPEG Base64 data URLs; rejected invalid/local references.
3. Added explicit rejection for masks, multiple references, and MiniMax multipart edits.
4. Made Playground controls model-aware:
   - hide the mask for MiniMax;
   - enforce one PNG/JPEG reference;
   - enforce a 7 MB upload limit.

Supported aspect-ratio controls and optional seed/prompt-optimizer controls remain potential Playground enhancements.

### Phase 3: Reliability and Observability

1. Add multi-account image fallback/cooldown.
2. Log MiniMax trace ID, success count, failed count, latency, and estimated cost.
3. Add per-provider image metrics and 429 monitoring.
4. Consider shared image-provider interface only when another native image protocol is added.

## Test Matrix

### Translator Unit Tests

- Default request -> `image-01`, `1:1`, `n=1`, correct response format.
- `b64_json` -> `base64`.
- `url` -> `url`.
- `1024x1024`, `1792x1024`, `1024x1792` -> exact width/height.
- Explicit `aspect_ratio` overrides size.
- Width/height bounds and divisibility.
- Prompt empty and over 1,500.
- `n=0` default and `n<1`/`n>9` validation policy.
- Seed `0`, positive, negative, and int64 bounds.
- Quality/style ignored, never forwarded.

### Response Parser Tests

- URL response.
- Base64 response.
- Multiple images.
- HTTP non-200.
- HTTP 200 with non-zero `base_resp`.
- Missing data.
- Partial success metadata.
- Malformed JSON.
- Response over configured limit.

### Handler Tests

- `minimax/image-01`.
- `minimax/image-01@connectionId`.
- Tenant isolation and allowed-model/allowed-connection policy.
- No available MiniMax connection.
- API key redaction in logs.
- URL and Base64 OpenAI-compatible output.
- `GET /v1/models?type=image` includes MiniMax only when an eligible connection supports it.

### Live Smoke Test

```bash
curl http://localhost:20199/v1/images/generations \
  -H "Authorization: Bearer $DNTPROXY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax/image-01",
    "prompt": "A small red paper boat on a quiet lake at sunrise",
    "size": "1024x1024",
    "n": 1,
    "response_format": "b64_json"
  }'
```

Success criteria:

- HTTP 200.
- Exactly one `data[].b64_json`.
- Valid decoded image.
- No secret/Base64 payload in structured logs.
- Cost estimate USD 0.0035.

## Risks and Decisions

| Decision | Rationale |
|---|---|
| Reuse `/v1/images/generations` | Existing client/UI compatibility; least code |
| Separate MiniMax translator | Native path and response are not OpenAI-compatible |
| T2I before I2I | Allowed the reference-image JSON contract and model-aware Playground validation to follow the generation MVP |
| No streaming | MiniMax image endpoint is synchronous |
| Do not register `image-01-live` initially | Official docs disagree on availability |
| Validate `base_resp` | Avoid false success on HTTP 200 |
| Ignore unsupported `quality`/`style` | Compatible behavior without inventing semantics |
| Defer generic image port | YAGNI; current specialized image path is adequate |

## References

Official MiniMax sources:

- [Image Generation Guide](https://platform.minimax.io/docs/guides/image-generation)
- [Text-to-Image API Reference](https://platform.minimax.io/docs/api-reference/image-generation-t2i)
- [Image-to-Image API Reference](https://platform.minimax.io/docs/api-reference/image-generation-i2i)
- [API Overview](https://platform.minimax.io/docs/api-reference/api-overview)
- [Rate Limits](https://platform.minimax.io/docs/guides/rate-limits)
- [Pay-as-You-Go Pricing](https://platform.minimax.io/docs/guides/pricing-paygo)
- [Error Codes](https://platform.minimax.io/docs/api-reference/errorcode)
- [API Release Notes](https://platform.minimax.io/docs/release-notes/apis)

## Unresolved Questions

1. Is `image-01-live` generally available, account-gated, or I2I-only?
2. Can a batch return HTTP 200 with both successful images and `failed_count > 0`; if yes, should dntproxy return partial success or fail the whole request?
3. Which HTTP statuses accompany `base_resp` business errors in production?
4. When should image execution gain the chat path's multi-account fallback and cooldown behavior?
