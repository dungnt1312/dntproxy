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
      │    └── storage/       (On-disk persistence adapter)
      │
      ├── port/               [Interfaces] Interfaces connecting Adapters and Services
      │
      ├── service/            [Use Cases] Coordination of entities to achieve outcomes
      │    ├── chat-service.go
      │    └── model-resolver.go
      │
      └── domain/             [Entities] Pure Data schemas and core rules
```

## Request Flow
The core lifecycle of an incoming chat completion request:
1. **Frontend Proxy**: Request hits the OpenAI-compatible HTTP router exposed by the Gin adapter inside `/v1/chat/completions`.
2. **Model Resolver**: Identifies if the requested model is a single direct model (`kr/claude`), an `alias` (short-name mapping), or a `combo` (model rotation strategy).
3. **Account Strategy**: `account-selector.go` locates valid Kiro accounts associated with the model. It references priority ranks and tests for existing active cooldowns.
4. **Execution Translation**: Adapter layer morphs the OpenAI JSON into Kiro AWS structure.
5. **Event Streaming**: `adapter/kiro` makes the upstream connection. AWS replies using a binary EventStream protocol. The adapter decodes this binary stream framing by framing.
6. **Delivery**: Binary frames are parsed, mapped back into standard OpenAI SSE lines (`data: {...}`), and piped over the HTTP connection to the client via `Flush()`.

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
