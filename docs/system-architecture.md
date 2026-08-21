# System Architecture

## Architecture Pattern
`dntproxy` implements **Clean Architecture** in Go to strictly decouple external infrastructure dependencies from business validation and application flow logic.

### Layer Diagram
```
cmd/                     [User Edge] Entrypoint / thin ops CLI (serve, version, update)
 └── internal/
      ├── adapter/            [Infrastructure] External libraries, framework specific APIs (Gin, file IO)
      │    ├── http/          (Gin server / HTTP Listeners / Messages endpoint)
      │    ├── kiro/          (Kiro API Translator / AWS EventStream parser)
      │    ├── openai/        (OpenAI chat/image executor + Codex translator)
      │    ├── xai/           (xAI chat/image executor)
      │    ├── minimax/       (MiniMax chat/image executor)
      │    ├── byteplus/      (ModelArk Seedream image adapter)
      │    ├── gemini/        (Gemini native image adapter)
      │    ├── anthropic/     (Anthropic Messages API executor + bidirectional translation)
      │    ├── auth/          (OAuth flows: Builder ID, IDC, Social, Import)
      │    ├── tunnel/        (Cloudflared download, lifecycle, state)
      │    ├── custom/        (NoOp adapters for providers without APIs)
      │    ├── shared/        (HTTP client, body sanitization, utilities)
      │    └── storage/       (On-disk JSON + SQLite persistence)
      │
      ├── port/               [Interfaces] Interfaces connecting Adapters and Services
      │
      ├── service/            [Use Cases] Coordination of entities to achieve outcomes
      │    ├── chat-service.go
      │    ├── model-resolver.go
      │    ├── model-cache.go
      │    ├── account-selector.go
      │    ├── combo-handler.go
      │    ├── token-refresh-scheduler.go
      │    └── tunnel-service.go
      │
      ├── logger/             [Telemetry] Structured logging (ring buffer + SQLite)
      │
      └── domain/             [Entities] Pure Data schemas and core rules
           ├── provider-config.go   (Provider definitions + auth methods)
           ├── model-definition.go  (Model registry with pricing)
           └── ...
```

## Supported Providers
The provider catalog currently contains 11 providers. Chat and image support
are registered independently, so an image-only provider does not need a fake
chat executor.

| Provider | ID | Auth | Protocol |
|----------|-----|------|----------|
| Kiro (AWS CodeWhisperer) | `kiro` | OAuth | aws-eventstream |
| OpenAI | `openai` | API Key, OAuth | openai-chat |
| OpenAI Compatible | `openai-compatible` | API Key | openai-chat |
| GLM (Zhipu AI) | `glm` | API Key | openai-chat |
| MiniMax | `minimax` | API Key | openai-chat |
| Qwen (Alibaba) | `qwen` | API Key, OAuth | openai-chat |
| Anthropic | `anthropic` | API Key | anthropic-messages |
| ClinePass | `cline` | API Key | openai-chat |
| Command Code | `commandcode` | API Key | commandcode-ndjson |
| Google Gemini | `gemini` | API Key | openai-chat + native images |
| BytePlus ModelArk | `byteplus` | API Key | image API |
| xAI | `xai` | OAuth | responses + image API |

OpenAI/OpenAI-compatible, xAI, MiniMax, BytePlus, and Gemini image operations
are registered in the dedicated image registry.

## Request Flow
The core lifecycle of an incoming chat completion request:
1. **Frontend Proxy**: Request hits the OpenAI-compatible HTTP router exposed by the Gin adapter inside `/v1/chat/completions`.
2. **Model Resolver**: Identifies if the requested model is a direct provider model (`kr/claude`, `oai/gpt-4`, `glm/glm-5`), an `alias` (short-name mapping), or a `combo` (model rotation strategy).
3. **Account Strategy**: `account-selector.go` locates valid provider accounts associated with the model, filters cooldown/model locks, then applies the configured connection strategy (`weighted-random`, `priority-fallback`, or `round-robin`).
4. **Token Refresh**: If credentials need refresh, auto-refresh is triggered before request execution.
5. **Execution Translation**: Adapter layer morphs the OpenAI JSON into provider-specific structure (EventStream for Kiro, standard Chat API for others).
6. **Event Streaming**: 
   - **Kiro**: AWS EventStream binary protocol, decoded frame-by-frame.
   - **Other providers**: Standard SSE or streaming JSON.
7. **Delivery**: Provider responses are parsed, mapped back into standard OpenAI SSE lines (`data: {...}`), and piped over the HTTP connection to the client via `Flush()`.
8. **Logging**: Request metadata, usage tokens, and estimated cost are persisted to SQLite.

### Image Provider Architecture

`ImageProviderRegistry` is separate from the chat `ProviderRegistry`. The HTTP
layer keeps authentication, tenant policy, `model@connection` pinning,
credential selection, capability checks, logging, and the OpenAI response/error
envelope. Provider adapters own request translation, upstream transport, and
response parsing.

```text
POST /v1/images/generations or /v1/images/edits
  -> authenticate and resolve provider/model@connection
  -> select tenant-scoped credentials
  -> ImageProviderRegistry.GetImageProvider(provider)
  -> validate ImageCapabilities for the selected model
  -> provider Generate/Edit
  -> return OpenAI-compatible data[].url or data[].b64_json
```

Startup registers OpenAI/OpenAI-compatible, xAI, MiniMax, BytePlus, and Gemini.
OpenAI/Codex streaming uses the optional `StreamingImageProvider` interface;
Gin still owns SSE framing and keepalives. OpenAI multipart editing remains
available only when the selected model and credentials support it. All other
new edit integrations use JSON.

`GET /v1/models?type=image` adds an `image_capabilities` object to each eligible
model. Runtime adapter metadata is authoritative and includes generation/edit
support, multipart/mask/streaming flags, reference limits, accepted input
formats, byte limits, and response formats. The Image Playground consumes this
metadata to enable or constrain edit controls rather than checking provider
names.

#### JSON edit contracts

| Provider | Reference input | Implemented limits | Response format |
|---|---|---|---|
| OpenAI/Codex | JSON or multipart, model-dependent | Up to 16 references for supported `gpt-image` models; masks and streaming are capability-gated | `url`, `b64_json` |
| xAI | JSON URL/data URL objects | Up to 10 PNG/JPEG/WebP references; masks supported; no multipart | `url`, `b64_json` |
| MiniMax `image-01` | JSON `image` or `images[0].image_url` | Exactly one PNG/JPEG reference, maximum 7 MiB; no mask or multipart | `url`, `b64_json` |
| BytePlus Seedream | JSON URL/data URL strings or objects | Seedream 5 Pro advertises 10 references; other integrated edit models advertise 14; no mask or multipart | `url`, `b64_json` |
| Gemini image models | JSON URL/data URL strings or objects | Up to 14 references within a 7 MiB aggregate proxy envelope; PNG/JPEG/WebP; no mask or multipart | `b64_json` |

Gemini edit references are converted to native `inline_data` before
`models/{model}:generateContent`. Remote images are loaded only through the
shared bounded loader: HTTP(S) only, no URL credentials, public IPs only,
bounded redirects, a 15-second request timeout, a 10 MiB read limit, and
MIME/content validation. MiniMax and BytePlus URL references are sent to their
upstream APIs rather than fetched by dntproxy.

The MiniMax translator additionally validates its prompt, image count,
dimensions/aspect ratio, and business response. BytePlus uses ModelArk
`/api/v3/images/generations` for both generation and reference editing. Gemini
uses its native `generateContent` endpoint while the existing Gemini chat
registration remains unchanged.

### Detailed Flow Diagram
```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              CLIENT REQUEST                                   │
│                    POST /v1/chat/completions                                  │
└──────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                         chatHandler (HTTP Layer)                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │ 1. Check RequireAPIKey setting                                       │    │
│  │    └── If enabled: validate API key from Authorization header         │    │
│  │ 2. Parse body → extract "model" field                               │    │
│  │ 3. Call chatService.HandleChat(body, model)                         │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                         ChatService.HandleChat                               │
│                                                                              │
│  ┌─────────────────┐     ┌──────────────────────────────────────────────┐   │
│  │ GetComboModels  │     │ ModelResolver.Resolve(modelStr)              │   │
│  │ (check if combo)│     │ ┌────────────────────────────────────────┐  │   │
│  └────────┬────────┘     │ │ "kr/model" → provider="kiro", model    │  │   │
│           │              │ │ "alias"   → lookup alias → kr/model    │  │   │
│           ▼              │ │ "combo"   → empty provider (combo)      │  │   │
│  ┌─────────────────┐     │ └────────────────────────────────────────┘  │   │
│  │ Is Combo?       │     └──────────────────────┬───────────────────────┘   │
│  │                 │                            │                           │
│  │ Yes ────────────┼──► handleComboChat()       │                           │
│  │                 │                            ▼                           │
│  │ No ─────────────┼──► handleSingleModel() ──► ┌────────────────────────┐ │
│  └─────────────────┘                            │ getExecutor(provider)   │ │
│                                                  │ "kiro" → KiroExecutor  │ │
│                                                  │ "openai" → OpenAIExec  │ │
│                                                  └───────────┬────────────┘ │
└──────────────────────────────────────────────────────────────┼───────────────┘
                                                               │
                    ┌──────────────────────────────────────────┴───────────────┐
                    │                    Combo Path (handleComboChat)           │
                    │  ┌────────────────────────────────────────────────────┐   │
                    │  │ 1. Get combo strategy (fallback / round-robin)   │   │
                    │  │ 2. ComboHandler.HandleCombo(models, strategy)     │   │
                    │  │    ┌─────────────────────────────────────────┐    │   │
                    │  │    │ For each model in strategy order:      │    │   │
                    │  │    │   handleSingleModel()                   │    │   │
                    │  │    │   ├── Success (200) → return stream    │    │   │
                    │  │    │   └── Failure → try next model          │    │   │
                    │  │    └─────────────────────────────────────────┘    │   │
                    │  │ 3. All failed → 503 "All combo models unavailable"│   │
                    │  └────────────────────────────────────────────────────┘   │
                    └───────────────────────────────────────────────────────────┘
                                      │
                    ┌─────────────────┴─────────────────┐
                    │                                   │
                    ▼                                   ▼
┌───────────────────────────────────┐  ┌───────────────────────────────────────┐
│      Single Model Path            │  │      AccountSelector.SelectCredentials │
│  handleSingleModel()             │  │                                       │
│  ┌─────────────────────────────┐  │  │  ┌─────────────────────────────────┐  │
│  │ ModelResolver.Resolve()     │  │  │  │ 1. GetActiveConnections()     │  │
│  │ └── provider + model        │  │  │  │    (active provider accounts) │  │
│  └─────────────┬───────────────┘  │  │  └─────────────┬─────────────────┘  │
│                │                  │  │                │                      │
│                ▼                  │  │                ▼                      │
│  ┌─────────────────────────────┐  │  │  ┌─────────────────────────────────┐│
│  │ Update body with resolved   │  │  │  │ 2. Filter connections:        ││
│  │ model name                  │  │  │  │    • Skip RateLimitedUntil >now││
│  └─────────────┬───────────────┘  │  │  │    • Skip ModelLocks[model]   ││
│                │                  │  │  │    • Skip !SupportsModel()     ││
│                └──────────┬───────┘  │  │  └─────────────┬─────────────────┘│
│                           │          │  │                │                    │
│                           ▼          │  │                ▼                    │
│  ┌─────────────────────────────────┐│  │  ┌─────────────────────────────────┐│
│  │ LOOP: Try accounts until OK    ││  │  │ 3. Check token expiry         ││
│  │                                 ││  │  │    • NeedsRefresh() → refresh ││
│  │ SelectCredentials(excludeIDs) ││  │  │ 4. Return Credentials          ││
│  │   ├── 404: No credentials      ││  │  └─────────────────────────────────┘│
│  │   ├── 503: All unavailable     ││  └─────────────────────────────────────┘
│  │   └── OK: creds ───────────────┼──┼───────────────────────────────────────►
│  │                                │  │                                           │
│  │ executor.Execute(model, body,  │  │
│  │           creds)               │  │
│  │   ├── Success → return stream  │  │
│  │   └── Failure:                 │  │
│  │        MarkUnavailable()        │  │
│  │        excludeIDs[id] = true   │  │
│  │        continue loop           │  │
│  └─────────────────────────────────┘│
└───────────────────────────────────┘
                                      │
                                      ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                          Executor.Execute()                                   │
│  ┌─────────────────────────────────────────────────────────────────────────┐  │
│  │ KiroExecutor (AWS EventStream)                                          │  │
│  │                                                                          │  │
│  │  1. TranslateRequest()                                                   │  │
│  │     OpenAI JSON → Kiro AWS EventStream format                           │  │
│  │                                                                          │  │
│  │  2. SignRequest()                                                        │  │
│  │     AWS Signature V4 signing with credentials                           │  │
│  │                                                                          │  │
│  │  3. DoRequest()                                                          │  │
│  │     HTTP POST to Kiro endpoint                                         │  │
│  │                                                                          │  │
│  │  4. ParseEventStream()                                                  │  │
│  │     Binary frames → OpenAI SSE events                                  │  │
│  │                                                                          │  │
│  │  5. TranslateResponse()                                                 │  │
│  │     Kiro events → OpenAI chat.completion chunks                        │  │
│  └─────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐  │
│  │ OpenAIExecutor / GLM / MiniMax / Qwen (Standard Chat API)              │  │
│  │                                                                          │  │
│  │  1. TranslateRequest()                                                   │  │
│  │     OpenAI JSON → Provider-specific format (if needed)                 │  │
│  │                                                                          │  │
│  │  2. DoRequest()                                                          │  │
│  │     HTTP POST to provider endpoint with API key / OAuth token          │  │
│  │                                                                          │  │
│  │  3. ParseStream()                                                       │  │
│  │     SSE / streaming JSON → OpenAI chat.completion chunks              │  │
│  └─────────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                           Response to Client                                  │
│                                                                              │
│  Success: 200 OK + SSE stream (text/event-stream)                          │
│  ├── data: {"choices":[{"delta":{"content":"..."}}]}                      │
│  ├── data: {"choices":[{"delta":{"content":"..."}}]}                      │
│  └── data: [DONE]                                                           │
│                                                                              │
│  Error:                                                                     │
│  ├── 400 Bad Request  - Invalid model / JSON                               │
│  ├── 401 Unauthorized - Missing/invalid API key                            │
│  ├── 404 Not Found    - No credentials for provider                        │
│  └── 503 Service Unavailable - All accounts failed                        │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Account State Transitions
```
┌──────────────┐     success      ┌──────────────┐
│   ACTIVE     │◄────────────────│    COOLDOWN   │
│              │                 │   (backoff)    │
└──────┬───────┘                 └───────┬───────┘
       │                                 │
       │ error + transient               │ cooldown expired
       │ (rate limit, 5xx)               │
       ▼                                 ▼
┌──────────────┐                 ┌──────────────┐
│  RATE_LIMITED │────────────────►│    ACTIVE     │
│  + BackoffLevel++              │              │
└──────────────┘   cooldown       └──────────────┘
                      expired
```

### Combo Strategies
```
FALLBACK strategy:     ROUND-ROBIN strategy:
┌─────────────────┐    ┌─────────────────┐
│ Request 1:      │    │ Request 1:      │
│ A → B → C       │    │ A(start) → B → C│
├─────────────────┤    ├─────────────────┤
│ Request 2:      │    │ Request 2:      │
│ A → B → C       │    │ B(start) → C → A│
├─────────────────┤    ├─────────────────┤
│ Request 3:      │    │ Request 3:      │
│ A → B → C       │    │ C(start) → A → B│
└─────────────────┘    └─────────────────┘
```

### Connection Strategies
- `weighted-random`: default, selects an available connection using `weight` as probability.
- `priority-fallback`: selects the lowest `priority` value first; equal priority keeps config order after weight tie-break.
- `round-robin`: rotates available connections per provider/model/allowlist key; pinned `model@connectionId` bypasses strategy.

## Resilience Patterns
- **Exponential Backoff Strategy**: When an API key faces a failure (rate limit, suspension), it is locked into a cooldown tier. (1s → 2s → 4s ... max 2m) ensuring automatic degradation without excessive retries.
- **Combo Flow**: If a model fails, and the request targeted a combo list, the request can fallback to the next available model configuration securely.

## Storage
A serverless `db.json` document store is paired with `gofrs/flock` for guaranteed sequential application read/writes across multi-process access, providing database-level safety with zero infra requirements.

## Cloudflare Tunnel
The tunnel subsystem enables exposing the local proxy via a public URL:

### Architecture
```
┌──────────────┐     ┌───────────────┐     ┌──────────────────┐
│   Client     │────►│ Cloudflare    │────►│ cloudflared      │
│   Request    │     │ Edge Network  │     │ (local process)  │
└──────────────┘     └───────────────┘     └────────┬─────────┘
                                                    │
                                             ┌──────▼──────────┐
                                             │ dntproxy        │
                                             │ localhost:port  │
                                             └─────────────────┘
```

### Key Components
- **`adapter/tunnel/cloudflared.go`**: Downloads cloudflared v2026.3.0 from GitHub, manages process lifecycle
- **`adapter/tunnel/state.go`**: Persistent state (`~/.dntproxy/tunnel/state.json`), PID tracking
- **`service/tunnel-service.go`**: Business logic (enable/disable/status, auto-restart on boot)
- **`adapter/http/tunnel-handler.go`**: REST API endpoints (`/api/tunnel/enable`, `/api/tunnel/disable`, `/api/tunnel/status`)
- **Control plane**: dashboard UI + `/api/tunnel/*` (no management CLI)

### Cross-Platform Support
- Auto-downloads appropriate binary for Windows, macOS, Linux (amd64/arm64)
- Extracts from `.tgz` archives on Unix, `.zip` on Windows
- Process kill: `taskkill` on Windows, `os.FindProcess` on Unix

## Logging System
Structured request logging with persistence and live streaming:

### Architecture
```
┌─────────────────┐     ┌───────────────┐     ┌─────────────────┐
│   Request       │────►│ Logger        │────►│ Ring Buffer     │
│   Metadata      │     │ (in-memory)   │     │ (1000 entries)  │
└─────────────────┘     └───────┬───────┘     └────────┬────────┘
                                │                      │
                         ┌──────▼───────┐      ┌──────▼──────────┐
                         │ SQLite DB    │      │ SSE Subscribers │
                         │ (logs.db)    │      │ (live stream)   │
                         └──────────────┘      └─────────────────┘
```

### Key Components
- **`logger/logger.go`**: In-memory ring buffer, SSE subscriber broadcast
- **`adapter/storage/sqlite-log-store.go`**: SQLite persistence, 30-day auto-retention
- **`port/log-store.go`**: `LogStore` interface (List, Summary, ConnectionSummaries, Prices)

### Features
- **Usage Tracking**: Input/output/total tokens per request
- **Cost Estimation**: Configurable model pricing profiles
- **Connection Filtering**: Filter logs by provider connection
- **Raw Body Logging**: Via `DNTPROXY_LOG_RAW_BODIES` env var (dev mode)
- **Body Sanitization**: Redacts sensitive fields (API keys, tokens, auth headers)
- **30-Day Retention**: Automatic cleanup of old logs

## Anthropic Messages API
Full bidirectional translation between OpenAI Chat Completions and Anthropic Messages API:

### Architecture
```
┌──────────────────┐     ┌─────────────────────┐     ┌──────────────────┐
│ OpenAI Request   │────►│ AnthropicExecutor   │────►│ Anthropic API    │
│ /v1/chat/        │     │ - Request translator│     │ /v1/messages     │
│ completions      │     │ - Response parser   │     │                  │
└──────────────────┘     └─────────────────────┘     └──────────────────┘
                                    │
                         ┌──────────▼──────────┐
                         │ SSE Event Stream    │
                         │ - message_start     │
                         │ - content_block_*   │
                         │ - message_delta     │
                         │ - message_stop      │
                         └─────────────────────┘
```

### Key Components
- **`adapter/anthropic/executor.go`**: Full executor with streaming support
- **`adapter/http/messages-handler.go`**: Native `/v1/messages` endpoint

### Features
- **Bidirectional Translation**: OpenAI ↔ Anthropic format conversion
- **Tool Calling**: Function/tool definitions and results
- **System Messages**: Proper system message handling
- **Content Blocks**: Text, tool use, tool result blocks
- **Streaming**: SSE event-by-event conversion
- **Stop Reasons**: Mapping between provider stop reasons

### Endpoints
- `POST /v1/chat/completions` with `model: anthropic/claude-*` → translates to Anthropic
- `POST /v1/messages` → native Anthropic Messages API endpoint

## Quota Checking System
Flexible quota checking with provider-specific implementations:

### Architecture
```
┌──────────────────┐     ┌─────────────────────┐     ┌──────────────────┐
│ UI/API Request   │────►│ QuotaChecker        │────►│ Provider API     │
│ GET /api/quota/:id│     │ Interface           │     │ (usage endpoint) │
└──────────────────┘     └─────────────────────┘     └──────────────────┘
                                    │
                         ┌──────────▼──────────┐
                         │ QuotaResult         │
                         │ - Buckets (used/max)│
                         │ - Reset times       │
                         │ - Percentages       │
                         └─────────────────────┘
```

### Key Components
- **`port/quota-checker.go`**: `QuotaChecker` interface with bucket-based results
- **`adapter/http/quota-handler.go`**: HTTP handler with provider-specific logic
- **`service/model-cache.go`**: TTL-based cache with singleflight deduplication

### Supported Providers
- **OpenAI (API Key)**: Rate-limit headers parsing
- **OpenAI (OAuth)**: Codex API usage endpoint
- **Kiro**: Amazon Q usage API
- **MiniMax**: `coding_plan/remains` API with interval/weekly buckets

### Features
- **Bucket-Based Results**: Flexible quota representation (daily, weekly, monthly)
- **Auto Token Refresh**: Refreshes expired OAuth tokens before checking
- **Gzip Support**: Handles compressed responses
- **Cloudflare Detection**: Detects and reports Cloudflare blocks
- **Model Caching**: Caches model lists with TTL to reduce API calls

## Model Fetching System
Dynamic model discovery with caching:

### Architecture
```
┌──────────────────┐     ┌─────────────────────┐     ┌──────────────────┐
│ UI/API Request   │────►│ ModelFetcher        │────►│ Provider API     │
│ GET /v1/models   │     │ Interface + Cache   │     │ /v1/models       │
└──────────────────┘     └─────────────────────┘     └──────────────────┘
                                    │
                         ┌──────────▼──────────┐
                         │ ModelCache          │
                         │ - TTL: 5 minutes    │
                         │ - Singleflight      │
                         └─────────────────────┘
```

### Key Components
- **`port/model-fetcher.go`**: `ModelFetcher` interface
- **`service/model-cache.go`**: Cache with deduplication
- **`adapter/custom/model-fetcher.go`**: NoOp implementation for providers without APIs

### Features
- **TTL Caching**: 5-minute cache to reduce API calls
- **Singleflight**: Deduplicates concurrent requests for same provider
- **Fallback**: Returns static model definitions if API unavailable
