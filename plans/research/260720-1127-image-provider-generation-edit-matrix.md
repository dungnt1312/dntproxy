---
title: Image provider generation and editing matrix
status: completed
created: 2026-07-20
updated: 2026-07-20T11:27:00+07:00
tags: [image-generation, image-editing, providers, architecture]
---

# Research Report: Image Providers for Generation and Editing

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Scope and Method](#scope-and-method)
3. [Provider Matrix](#provider-matrix)
4. [Detailed Findings](#detailed-findings)
5. [Architecture Recommendation](#architecture-recommendation)
6. [Integration Roadmap](#integration-roadmap)
7. [Security and Operational Requirements](#security-and-operational-requirements)
8. [Official References](#official-references)
9. [Unresolved Questions](#unresolved-questions)

## Executive Summary

dntproxy already supports OpenAI/Codex, xAI and MiniMax image generation/editing. The strongest additional first-party candidates are:

1. **BytePlus Seedream** — best first implementation. Near OpenAI-shaped JSON, one endpoint for text-to-image and reference-image editing, URL/Base64 inputs, multi-reference, optional streaming.
2. **Google Gemini Image** — highest product value. Existing `gemini` connection can be reused, but image calls need native Interactions or `generateContent` translation rather than the current OpenAI-compatible chat executor.
3. **Black Forest Labs FLUX.2** — strong general generation/editing, multi-reference and commercial API. Requires task polling.
4. **Alibaba Model Studio Qwen Image** — strong text editing and multilingual workflows. Must be a separate Model Studio connection; current `qwen` provider points to Qwen Chat/Portal OAuth and is not the same API product.
5. **Stability AI** — deepest mask/inpaint/outpaint/control surface. Best when precise region editing matters, but adapter scope is larger.

For a marketplace adapter, choose **Together AI first**. Its image API exposes FLUX and Google models through a relatively consistent image-generation contract. `fal` and Replicate offer greater model breadth but each model has its own schema and async lifecycle, increasing maintenance.

Do not add more provider branches directly to `image-handler.go`. First introduce an `ImageProvider` port, image registry and machine-readable capabilities. This lets the API and Playground support provider-specific features without accumulating conditionals.

## Scope and Method

- Research timestamp: 2026-07-20, Asia/Bangkok.
- Sources: current official provider/API documentation; marketplace documentation used only for marketplace behavior.
- Evaluated:
  - text-to-image;
  - instruction/reference image editing;
  - explicit mask/inpainting;
  - URL/Base64/multipart inputs;
  - synchronous versus task/queue execution;
  - auth and compatibility with dntproxy;
  - expected adapter effort.
- Out of scope:
  - self-hosted inference/ComfyUI;
  - image-only utilities such as upscaling or background removal unless bundled with generation/editing;
  - subjective image-quality benchmarking;
  - exact price comparison, which changes frequently.

## Provider Matrix

Legend: **Yes** = documented native support; **Ref** = reference/instruction editing without an explicit pixel mask; **Mask** = explicit inpaint/fill mask.

| Provider | Current image family | Generate | Edit | Mask | Multi-ref | Input | Execution | Fit for dntproxy |
|---|---|---:|---:|---:|---:|---|---|---|
| OpenAI | GPT Image | Yes | Yes | Yes | Yes | multipart, image IDs/URLs depending API | sync/stream | Implemented |
| xAI | Grok Imagine Image | Yes | Yes | No documented mask | Limited | JSON URL/Base64 | sync | Implemented |
| MiniMax | image-01 | Yes | Ref | No | API array; current adapter limits to 1 | JSON URL/data URL | sync | Implemented |
| Google Gemini | Gemini 3.1 Flash/Lite Image, Gemini 3 Pro Image | Yes | Ref, conversational semantic mask | No binary mask contract | Up to model limit | inline Base64/image parts | sync/multi-turn | **Priority A** |
| BytePlus | Seedream 5/4.5/4 | Yes | Ref, interactive locations | No standard mask field | 10-14 by model | JSON URL/data URL | sync; some models stream | **Priority A** |
| BFL | FLUX.2, FLUX.1 Kontext | Yes | Ref | Separate FLUX Fill for masks | Up to 8 API refs for FLUX.2 | URL/Base64/provider upload varies | task/poll | **Priority A** |
| Alibaba Model Studio | Qwen Image / Qwen Image Edit | Yes | Ref | Model-specific control support | Yes | Model Studio JSON, URLs/Base64 | sync/async by model | **Priority A/B** |
| Stability AI | Stable Image | Yes | Yes | Yes | Control/reference endpoints | multipart binary | sync and async operations | **Priority B** |
| Ideogram | Ideogram 4/3 | Yes | Remix/edit | Yes | Character/style refs | JSON + multipart by endpoint | mostly sync | **Priority B** |
| Recraft | Recraft V4/V3 | Yes | Background/edit operations | Yes | Style refs | OpenAI-like JSON generation; multipart edits | sync/async by operation | **Priority B** |
| AWS Bedrock | Nova Canvas | Yes | Inpaint/outpaint/variation | Yes | Model/task dependent | InvokeModel Base64 JSON | sync | Priority C |
| Adobe Firefly | Image5/Firefly Image | Yes | Fill/expand/composite | Yes | Style/structure refs | upload ID or allow-listed URL | upload + async job | Priority C |
| Runway | Gen-4 Image | Yes | Reference-guided transform | No standard mask | Up to 3 refs | URL/data URI | task/async | Priority C |
| Together AI | FLUX, Google image models | Yes | Ref | Model dependent | Yes | JSON `image_url`/`reference_images` | sync-like API | **Marketplace A** |
| fal | 1,000+ model endpoints | Yes | Model dependent | Model dependent | Model dependent | per-model JSON | queue recommended | Marketplace B |
| Replicate | Official/community models | Yes | Model dependent | Model dependent | Model dependent | per-model JSON | prediction/queue | Marketplace B |

## Detailed Findings

### 1. Google Gemini Image

Recommended models as of research date:

- `gemini-3.1-flash-image`: default balanced option; multiple references and strong text rendering.
- `gemini-3.1-flash-lite-image`: cheapest/high-throughput option.
- `gemini-3-pro-image`: premium complex edits and up to 4K output.

Generation and editing use multimodal input: text plus inline image parts. Editing is conversational and can perform semantic “change only this element” operations, but it is not the same as a deterministic binary mask API.

Important: Google documents Imagen deprecation and shutdown on **2026-08-17**. Do not build a new Imagen adapter; target Gemini native image models.

dntproxy impact:

- reuse existing `gemini` credentials/provider;
- add native image translator and response-part parser;
- do not send image calls to `/v1beta/openai/chat/completions`;
- preserve multi-turn thought signatures only if conversational edit sessions are later exposed.

### 2. BytePlus Seedream

Strongest quick win:

- endpoint already resembles OpenAI: `POST /api/v3/images/generations`;
- text generation and image editing share the same endpoint;
- `image` accepts an accessible URL or `data:image/...;base64,...`;
- current models support single and multi-reference editing;
- some models support streaming multiple outputs;
- response already contains `created` and `data[].url`.

Recommended first model set:

- Seedream 5 Pro/Lite if account availability is confirmed;
- Seedream 4.5 as stable fallback;
- exclude older text-only Seedream models from edit capability.

Create provider `byteplus`, not `openai-compatible`, because model-specific image fields and streaming behavior need translation.

### 3. Black Forest Labs

FLUX.2 is the recommended BFL generation/edit family. FLUX.1 Kontext remains useful but is previous generation.

Strengths:

- unified prompt-based generation and editing;
- strong character/product consistency;
- multi-reference;
- text editing and high-resolution output.

Costs to architecture:

- task submission plus polling/status lifecycle;
- different fields and reference limits by model;
- FLUX Fill/inpainting is a separate capability from Kontext-style edits.

### 4. Alibaba Model Studio Qwen Image

Qwen Image Edit supports natural-language edits, multi-image input, text replacement, object changes and style transfer.

Do not attach this to the current `qwen` provider without an explicit migration:

- current dntproxy `qwen` uses `portal.qwen.ai` and Qwen Chat OAuth/API behavior;
- Model Studio/DashScope uses a different base URL, API key product and image contract.

Recommended provider ID: `dashscope` or `alibaba-model-studio`. Expose Qwen Image models under that provider while keeping existing Qwen Chat connections backward compatible.

### 5. Stability AI

Best provider for deterministic editing tools:

- text-to-image;
- inpaint with explicit image and mask;
- outpaint;
- search/replace;
- replace background and relight;
- sketch/structure/style control.

The adapter must support multipart binary requests and both binary and Base64 JSON responses. This is more work than reference-only editors but maps well to the existing `/v1/images/edits` multipart route.

### 6. Ideogram

Useful for typography, posters, logos and controlled design:

- generate;
- remix;
- inpaint;
- edit/face swap;
- reframe and background workflows.

Endpoints are operation-specific and often multipart. Model and endpoint selection should be driven by capabilities rather than assuming one universal edit endpoint.

### 7. Recraft

Generation is close to OpenAI:

- `POST /v1/images/generations`;
- `url` and `b64_json`;
- OpenAI SDK can be pointed at its base URL.

Specialized edits such as `generateBackground` use separate multipart endpoints with masks. Recraft is attractive if vector generation and design assets matter to the product.

### 8. Cloud and Enterprise Providers

**AWS Bedrock Nova Canvas**

- synchronous `InvokeModel`;
- text-to-image, variations, inpaint and outpaint;
- mature AWS security and private networking;
- needs Bedrock credentials/region/signing work unrelated to current Kiro OAuth.

**Adobe Firefly**

- rich fill, expand, composite and style/structure reference APIs;
- commercially attractive for brand-safe workflows;
- OAuth client credentials plus `x-api-key`;
- upload IDs, allow-listed external URL domains and async job polling make it the most complex adapter in this set.

**Runway**

- Gen-4 Image supports text/image-to-image and up to three references;
- URL and Base64 data URIs;
- task-based API;
- better categorized as reference-guided generation than precise inpainting.

### 9. Marketplace Providers

**Together AI**

- best first marketplace adapter;
- one account exposes FLUX and Google image models;
- editing fields are documented as `image_url` or `reference_images`;
- still requires per-model capability metadata.

**fal**

- very broad catalog;
- queue API is recommended;
- every model has its own schema, limits and output fields.

**Replicate**

- broad official/community catalog with stable official-model endpoints;
- prediction object can remain queued after a synchronous wait;
- model versions and schemas must be normalized.

Do not present fal/Replicate as fully OpenAI-compatible. Build them as generic async model runtimes with per-model adapters.

## Architecture Recommendation

### Problem in Current Code

`port.ProviderExecutor` is chat-specific:

```go
Execute(model string, body []byte, credentials *domain.Credentials, reqlog RequestLogger) (io.ReadCloser, int, error)
```

Image routing currently lives in `internal/adapter/http/image-handler.go` with provider branches. Adding ten providers this way creates a high-regression central file.

### Proposed Ports

```go
type ImageProvider interface {
    Generate(ctx context.Context, req ImageRequest, creds *domain.Credentials, log RequestLogger) (ImageResult, error)
    Edit(ctx context.Context, req ImageEditRequest, creds *domain.Credentials, log RequestLogger) (ImageResult, error)
    Capabilities(model string) ImageCapabilities
}

type ImageCapabilities struct {
    Generate          bool
    Edit              bool
    Mask              bool
    MultiReference    bool
    MaxReferences     int
    InputFormats      []string
    InputModes        []string // url, data-url, multipart, upload-id
    ResponseFormats   []string // url, b64_json
    Async             bool
    Streaming         bool
    MaxInputBytes     int64
}
```

Add `ImageProviderRegistry` keyed by provider ID. HTTP handlers should:

```text
OpenAI request
  -> resolve provider/model/account/policy
  -> normalize images and mask
  -> capability validation
  -> image provider registry
  -> provider translator/transport or async poller
  -> OpenAI-compatible response/error
```

### Model Metadata

Standardize capabilities:

- `image-generation`
- `image-edit`
- `image-mask`
- `image-multi-reference`
- `image-streaming`
- `image-vector`

Provider/model metadata should include limits. Playground renders controls from the selected model instead of provider-name checks.

### Async Abstraction

BFL, Adobe, Runway, fal and Replicate require tasks or queues. Add a reusable poller:

```go
type ImageTaskClient interface {
    Submit(ctx context.Context, ...) (taskID string, err error)
    Status(ctx context.Context, taskID string) (ImageTaskStatus, error)
    Cancel(ctx context.Context, taskID string) error
}
```

For the synchronous OpenAI endpoint, poll until completion with request cancellation and a bounded timeout. A later API can expose native async jobs without changing provider adapters.

## Integration Roadmap

### Phase 0 — Image Provider Foundation

- create image port and registry;
- move OpenAI, xAI and MiniMax behind adapters;
- add capability/limit metadata;
- centralize URL/data URL/multipart normalization;
- centralize sync and task-based execution;
- update Playground to use capabilities.

Acceptance: no provider-name branching in handler; existing image tests remain green.

### Phase 1 — Fast, High-Value Providers

1. **BytePlus Seedream**
2. **Google Gemini Image**

Why:

- BytePlus validates the unified JSON adapter quickly;
- Gemini reuses an existing connection and adds premium generation/editing;
- together they cover single/multi-reference, Base64 inputs, 2K/4K and optional streaming.

### Phase 2 — Commercial Image Specialists

1. **BFL FLUX.2**
2. **Alibaba Model Studio Qwen Image**
3. **Together AI**

This phase validates async polling, multi-reference schemas and marketplace model capability routing.

### Phase 3 — Precise Editing

1. **Stability AI**
2. **Ideogram**
3. **Recraft**

Add multipart normalization, masks, operation-specific routing, binary responses and vector outputs.

### Phase 4 — Enterprise and Broad Marketplaces

- AWS Bedrock Nova Canvas;
- Adobe Firefly;
- Runway;
- fal;
- Replicate.

Only start when demand justifies extra auth, upload and job lifecycle complexity.

## Security and Operational Requirements

- Never persist Base64 image inputs, masks, upload IDs or signed output URLs in raw logs.
- Enforce encoded and decoded byte limits before forwarding.
- Detect image MIME from magic bytes; do not trust data-URL declarations or multipart headers.
- Do not make unrestricted server-side fetches of arbitrary image URLs. If fetching is required, block private/link-local IPs, redirects to private networks and non-HTTP schemes.
- Preserve request-context cancellation through upload, submit and polling.
- Use image-specific overall, response-header and body-read timeouts.
- Cap polling duration and exponential backoff; propagate `Retry-After`.
- Normalize moderation failures without exposing provider internals.
- Delete provider uploads/tasks when supported and no longer needed.
- Treat signed URLs as credentials; redact query strings.
- Test partial-success batches and expired output URLs.

## Common Pitfalls

- Calling reference-guided generation “inpainting” when no explicit mask is supported.
- Assuming all image providers implement `/v1/images/edits`.
- Reusing chat streaming clients for long synchronous image jobs.
- Sending 10 MB files as Base64 through a 10 MB JSON body limit.
- Reusing current Qwen Chat credentials for Alibaba Model Studio.
- Integrating deprecated Imagen instead of Gemini native image models.
- Treating fal/Replicate model schemas as stable provider-wide contracts.
- Returning only URLs when the client requested `b64_json`, without a safe bounded downloader.

## Official References

### First-party Providers

- [Google Gemini image generation and editing](https://ai.google.dev/gemini-api/docs/image-generation)
- [xAI image editing](https://docs.x.ai/developers/model-capabilities/images/editing)
- [MiniMax image-to-image](https://platform.minimax.io/docs/api-reference/image-generation-i2i)
- [BytePlus image generation API](https://docs.byteplus.com/en/docs/ModelArk/1541523)
- [Black Forest Labs FLUX Kontext/FLUX.2 overview](https://docs.bfl.ai/kontext/kontext_overview)
- [Alibaba Model Studio Qwen Image Edit](https://help.aliyun.com/en/model-studio/qwen-image-edit-api)
- [Stability AI API reference](https://platform.stability.ai/docs/api-reference)
- [Ideogram API overview](https://developer.ideogram.ai/)
- [Ideogram inpaint endpoint](https://developer.ideogram.ai/api-reference/api-reference/inpaint-v3)
- [Recraft API endpoints](https://www.recraft.ai/docs/api-reference/endpoints)
- [Amazon Nova Canvas image generation/editing](https://docs.aws.amazon.com/nova/latest/userguide/image-generation.html)
- [Adobe Firefly API reference](https://developer.adobe.com/firefly-services/docs/firefly-api/api/)
- [Runway API reference](https://docs.dev.runwayml.com/api/)

### Marketplace Providers

- [Together image-to-image](https://docs.together.ai/docs/inference/images/reference-images)
- [fal model APIs](https://fal.ai/docs/documentation/model-apis/overview)
- [fal asynchronous inference](https://fal.ai/docs/documentation/model-apis/inference/queue)
- [Replicate HTTP API](https://replicate.com/docs/reference/http/)
- [Replicate official models](https://replicate.com/docs/topics/models/official-models/)

## Actionable Next Steps

1. Approve Phase 0 architecture before adding providers.
2. Obtain test API keys for BytePlus and Google Gemini.
3. Build live contract probes for generation, URL edit, Base64 edit, multi-reference and timeout.
4. Implement BytePlus, then Gemini.
5. Choose BFL direct versus Together marketplace based on billing/account preference.
6. Add Stability only when mask/inpainting is a product requirement.

## Unresolved Questions

1. Which provider accounts/API keys are already available for live integration tests?
2. Is precise mask/inpainting required now, or is instruction/reference editing sufficient?
3. Should `/v1/images/edits` support only JSON, or full OpenAI multipart for every capable provider?
4. Should dntproxy block synchronously while async providers finish, or expose a native task API?
5. Is one marketplace connection preferred over multiple first-party billing relationships?
6. Are vector outputs from Recraft part of the product scope?
