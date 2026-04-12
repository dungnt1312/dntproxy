# System Architecture

## Architecture Pattern
`dntproxy` implements **Clean Architecture** in Go to strictly decouple external infrastructure dependencies from business validation and application flow logic.

### Layer Diagram
```
cmd/                     [User Edge] Entrypoint / CLI parsing
 └── internal/
      ├── adapter/            [Infrastructure] External libraries, framework specific APIs (Gin, file IO)
      │    ├── http/          (Gin server / HTTP Listeners)
      │    ├── kiro/          (Kiro API Translator / AWS EventStream parser)
      │    ├── openai/        (OpenAI executor + Codex translator)
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
The system supports 7 providers with an extensible architecture:

| Provider | ID | Auth | Protocol |
|----------|-----|------|----------|
| Kiro (AWS CodeWhisperer) | `kiro` | OAuth | aws-eventstream |
| OpenAI | `openai` | API Key, OAuth | openai-chat |
| OpenAI Compatible | `openai-compatible` | API Key | openai-chat |
| GLM (Zhipu AI) | `glm` | API Key | openai-chat |
| MiniMax | `minimax` | API Key | openai-chat |
| Qwen (Alibaba) | `qwen` | API Key, OAuth | openai-chat |
| Anthropic | `anthropic` | API Key | anthropic-msg (TODO) |

## Request Flow
The core lifecycle of an incoming chat completion request:
1. **Frontend Proxy**: Request hits the OpenAI-compatible HTTP router exposed by the Gin adapter inside `/v1/chat/completions`.
2. **Model Resolver**: Identifies if the requested model is a direct provider model (`kr/claude`, `oai/gpt-4`, `glm/glm-5`), an `alias` (short-name mapping), or a `combo` (model rotation strategy).
3. **Account Strategy**: `account-selector.go` locates valid provider accounts associated with the model. It references priority ranks and tests for existing active cooldowns.
4. **Token Refresh**: If credentials need refresh, auto-refresh is triggered before request execution.
5. **Execution Translation**: Adapter layer morphs the OpenAI JSON into provider-specific structure (EventStream for Kiro, standard Chat API for others).
6. **Event Streaming**: 
   - **Kiro**: AWS EventStream binary protocol, decoded frame-by-frame.
   - **Other providers**: Standard SSE or streaming JSON.
7. **Delivery**: Provider responses are parsed, mapped back into standard OpenAI SSE lines (`data: {...}`), and piped over the HTTP connection to the client via `Flush()`.
8. **Logging**: Request metadata, usage tokens, and estimated cost are persisted to SQLite.

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
│  │ └── provider + model        │  │  │  │    (sorted by priority ASC)   │  │
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
- **CLI**: `dntproxy tunnel enable|disable|status`

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
