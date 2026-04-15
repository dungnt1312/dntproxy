---
status: pending
priority: high
effort: 1 week
phase: 4
dependencies: [phase-1, phase-2, phase-3]
---

# Phase 4: HTTP Layer (Week 4)

## Goals

Build the HTTP API layer with proper authentication, middleware, error handling, and OpenAI-compatible endpoints.

## Tasks

### 4.1 HTTP Server Setup

**File:** `internal/http/server.go`

Setup Gin server with graceful shutdown:

```go
type Server struct {
    router      *gin.Engine
    orchestrator *service.RequestOrchestrator
    configStore  *storage.ConfigStore
    logStore     *storage.LogStore
    port         int
}

func NewServer(
    orchestrator *service.RequestOrchestrator,
    configStore *storage.ConfigStore,
    logStore *storage.LogStore,
    port int,
) *Server

func (s *Server) Start() error
func (s *Server) Shutdown(ctx context.Context) error
func (s *Server) setupRoutes()
func (s *Server) setupMiddleware()
```

**Acceptance Criteria:**
- [ ] Gin server with custom config
- [ ] Graceful shutdown (SIGTERM, SIGINT)
- [ ] CORS middleware
- [ ] Request ID middleware
- [ ] Recovery middleware
- [ ] Logging middleware

---

### 4.2 Authentication Middleware

**File:** `internal/http/middleware/auth.go`

Implement API key authentication:

```go
type AuthMiddleware struct {
    configStore *storage.ConfigStore
    required    bool
}

func NewAuthMiddleware(store *storage.ConfigStore, required bool) *AuthMiddleware
func (m *AuthMiddleware) Authenticate() gin.HandlerFunc
func (m *AuthMiddleware) extractAPIKey(c *gin.Context) string
func (m *AuthMiddleware) validateAPIKey(apiKey string) (*domain.APIKey, error)
```

**Key Extraction:**
- Header: `Authorization: Bearer sk-xxx`
- Query: `?api_key=sk-xxx`
- Priority: Header > Query

**Acceptance Criteria:**
- [ ] Extract API key from header/query
- [ ] Validate key hash
- [ ] Check key active status
- [ ] Check key expiry
- [ ] Set key in context for downstream handlers
- [ ] Return 401 if invalid/missing
- [ ] Skip validation if not required (settings)
- [ ] Unit tests with 95%+ coverage

---

### 4.3 Rate Limit Middleware

**File:** `internal/http/middleware/ratelimit.go`

Implement rate limiting middleware:

```go
type RateLimitMiddleware struct {
    rateLimiter *service.RateLimiter
}

func NewRateLimitMiddleware(limiter *service.RateLimiter) *RateLimitMiddleware
func (m *RateLimitMiddleware) Limit() gin.HandlerFunc
func (m *RateLimitMiddleware) estimateTokens(req *service.ChatRequest) int
```

**Acceptance Criteria:**
- [ ] Check RPM limit
- [ ] Check TPM limit (estimated)
- [ ] Return 429 if exceeded
- [ ] Include Retry-After header
- [ ] Unit tests with 95%+ coverage

---

### 4.4 Logging Middleware

**File:** `internal/http/middleware/logging.go`

Implement request logging middleware:

```go
type LoggingMiddleware struct {
    logger *log.Logger
}

func NewLoggingMiddleware() *LoggingMiddleware
func (m *LoggingMiddleware) Log() gin.HandlerFunc
```

**Log Format:**
```
[2026-04-15 10:00:00] POST /v1/chat/completions | 200 | 123ms | req-abc123 | sk-xxx***
```

**Acceptance Criteria:**
- [ ] Log all requests
- [ ] Include method, path, status, duration, request ID
- [ ] Mask API key (show first 6 chars)
- [ ] Don't log request/response bodies (privacy)
- [ ] Unit tests

---

### 4.5 Chat Completion Handler

**File:** `internal/http/handler/chat.go`

Implement OpenAI-compatible chat endpoint:

```go
type ChatHandler struct {
    orchestrator *service.RequestOrchestrator
}

func NewChatHandler(orchestrator *service.RequestOrchestrator) *ChatHandler
func (h *ChatHandler) HandleChatCompletion(c *gin.Context)
func (h *ChatHandler) streamResponse(c *gin.Context, chunks <-chan service.StreamChunk)
func (h *ChatHandler) sendNonStreamResponse(c *gin.Context, resp *service.Response)
```

**Endpoint:** `POST /v1/chat/completions`

**Request Body:**
```json
{
  "model": "kr/sonnet",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "stream": false,
  "temperature": 0.7,
  "max_tokens": 1000,
  "tools": []
}
```

**Response (Non-Stream):**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "kr/sonnet",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  }
}
```

**Response (Stream):**
```
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"kr/sonnet","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"kr/sonnet","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"kr/sonnet","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}

data: [DONE]
```

**Acceptance Criteria:**
- [ ] Parse request body
- [ ] Validate required fields
- [ ] Get API key from context
- [ ] Call orchestrator
- [ ] Stream SSE if stream=true
- [ ] Return JSON if stream=false
- [ ] Handle errors (401, 403, 429, 402, 500)
- [ ] Set proper headers (Content-Type, Cache-Control)
- [ ] Unit tests with 95%+ coverage

---

### 4.6 Models Handler

**File:** `internal/http/handler/models.go`

Implement model listing endpoint:

```go
type ModelsHandler struct {
    configStore *storage.ConfigStore
}

func NewModelsHandler(store *storage.ConfigStore) *ModelsHandler
func (h *ModelsHandler) ListModels(c *gin.Context)
```

**Endpoint:** `GET /v1/models`

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "kr/sonnet",
      "object": "model",
      "created": 1234567890,
      "owned_by": "kiro"
    },
    {
      "id": "oai/gpt-4",
      "object": "model",
      "created": 1234567890,
      "owned_by": "openai"
    }
  ]
}
```

**Acceptance Criteria:**
- [ ] List all available models
- [ ] Filter by API key permissions (if authenticated)
- [ ] Include combos as models
- [ ] OpenAI-compatible format
- [ ] Unit tests

---

### 4.7 Admin API: API Keys

**File:** `internal/http/handler/admin/apikeys.go`

Implement API key management endpoints:

```go
type APIKeysHandler struct {
    configStore *storage.ConfigStore
}

func NewAPIKeysHandler(store *storage.ConfigStore) *APIKeysHandler
func (h *APIKeysHandler) List(c *gin.Context)
func (h *APIKeysHandler) Create(c *gin.Context)
func (h *APIKeysHandler) Get(c *gin.Context)
func (h *APIKeysHandler) Update(c *gin.Context)
func (h *APIKeysHandler) Delete(c *gin.Context)
func (h *APIKeysHandler) AddCredits(c *gin.Context)
```

**Endpoints:**
- `GET /admin/api-keys` — List all keys
- `POST /admin/api-keys` — Create new key
- `GET /admin/api-keys/:id` — Get key details
- `PUT /admin/api-keys/:id` — Update key
- `DELETE /admin/api-keys/:id` — Delete key
- `POST /admin/api-keys/:id/credits` — Add credits

**Create Request:**
```json
{
  "name": "Production Key",
  "allowed_models": ["kr/sonnet", "oai/gpt-4"],
  "allowed_providers": ["*"],
  "rpm": 60,
  "tpm": 100000,
  "credit_limit": 1000.00,
  "expires_at": "2027-01-01T00:00:00Z",
  "tags": ["customer-a"]
}
```

**Create Response:**
```json
{
  "id": "pk-abc123",
  "key": "sk-1234567890abcdef",
  "name": "Production Key",
  "credit_balance": 0,
  "created_at": "2026-04-15T10:00:00Z"
}
```

**Note:** Full key only shown on creation

**Acceptance Criteria:**
- [ ] List with pagination
- [ ] Create with bcrypt hash
- [ ] Show full key only on creation
- [ ] Update (except key_hash)
- [ ] Delete (soft delete: set is_active=false)
- [ ] Add credits (increment balance)
- [ ] Validate input
- [ ] Unit tests with 95%+ coverage

---

### 4.8 Admin API: Provider Accounts

**File:** `internal/http/handler/admin/accounts.go`

Implement provider account management endpoints:

```go
type AccountsHandler struct {
    configStore *storage.ConfigStore
    providers   *provider.Registry
}

func NewAccountsHandler(store *storage.ConfigStore, providers *provider.Registry) *AccountsHandler
func (h *AccountsHandler) List(c *gin.Context)
func (h *AccountsHandler) Create(c *gin.Context)
func (h *AccountsHandler) Get(c *gin.Context)
func (h *AccountsHandler) Update(c *gin.Context)
func (h *AccountsHandler) Delete(c *gin.Context)
func (h *AccountsHandler) Test(c *gin.Context)
func (h *AccountsHandler) RefreshToken(c *gin.Context)
```

**Endpoints:**
- `GET /admin/accounts` — List all accounts
- `POST /admin/accounts` — Create new account
- `GET /admin/accounts/:id` — Get account details
- `PUT /admin/accounts/:id` — Update account
- `DELETE /admin/accounts/:id` — Delete account
- `POST /admin/accounts/:id/test` — Test account
- `POST /admin/accounts/:id/refresh` — Refresh token

**Acceptance Criteria:**
- [ ] List with provider filter
- [ ] Create with encrypted credentials
- [ ] Update (except credentials)
- [ ] Delete (soft delete: set is_active=false)
- [ ] Test (call provider ValidateAccount)
- [ ] Refresh token (call provider RefreshToken)
- [ ] Mask credentials in responses
- [ ] Unit tests with 95%+ coverage

---

### 4.9 Admin API: Combos

**File:** `internal/http/handler/admin/combos.go`

Implement combo management endpoints:

```go
type CombosHandler struct {
    configStore *storage.ConfigStore
}

func NewCombosHandler(store *storage.ConfigStore) *CombosHandler
func (h *CombosHandler) List(c *gin.Context)
func (h *CombosHandler) Create(c *gin.Context)
func (h *CombosHandler) Get(c *gin.Context)
func (h *CombosHandler) Update(c *gin.Context)
func (h *CombosHandler) Delete(c *gin.Context)
```

**Endpoints:**
- `GET /admin/combos` — List all combos
- `POST /admin/combos` — Create new combo
- `GET /admin/combos/:id` — Get combo details
- `PUT /admin/combos/:id` — Update combo
- `DELETE /admin/combos/:id` — Delete combo

**Acceptance Criteria:**
- [ ] CRUD operations
- [ ] Validate models exist
- [ ] Validate strategy (fallback, round-robin)
- [ ] Unit tests with 95%+ coverage

---

### 4.10 Admin API: Logs

**File:** `internal/http/handler/admin/logs.go`

Implement log viewing endpoints:

```go
type LogsHandler struct {
    logStore *storage.LogStore
}

func NewLogsHandler(store *storage.LogStore) *LogsHandler
func (h *LogsHandler) List(c *gin.Context)
func (h *LogsHandler) Stream(c *gin.Context)
func (h *LogsHandler) Clear(c *gin.Context)
```

**Endpoints:**
- `GET /admin/logs` — List logs with filters
- `GET /admin/logs/stream` — SSE stream of new logs
- `DELETE /admin/logs` — Clear old logs

**Query Params:**
- `api_key_id` — Filter by API key
- `provider_id` — Filter by provider
- `start_date` — Filter by start date
- `end_date` — Filter by end date
- `limit` — Limit results (default 100)
- `offset` — Offset for pagination

**Acceptance Criteria:**
- [ ] List with filters
- [ ] Pagination
- [ ] SSE streaming for live logs
- [ ] Clear logs older than N days
- [ ] Unit tests with 95%+ coverage

---

### 4.11 Admin API: Settings

**File:** `internal/http/handler/admin/settings.go`

Implement settings management endpoints:

```go
type SettingsHandler struct {
    configStore *storage.ConfigStore
}

func NewSettingsHandler(store *storage.ConfigStore) *SettingsHandler
func (h *SettingsHandler) Get(c *gin.Context)
func (h *SettingsHandler) Update(c *gin.Context)
```

**Endpoints:**
- `GET /admin/settings` — Get settings
- `PUT /admin/settings` — Update settings

**Acceptance Criteria:**
- [ ] Get current settings
- [ ] Update settings
- [ ] Validate input
- [ ] Unit tests

---

### 4.12 Error Handling

**File:** `internal/http/errors.go`

Implement consistent error responses:

```go
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Message string `json:"message"`
    Type    string `json:"type"`
    Code    string `json:"code,omitempty"`
}

func HandleError(c *gin.Context, err error)
func NewErrorResponse(message, errType, code string) *ErrorResponse
```

**Error Types:**
- `invalid_request_error` — 400
- `authentication_error` — 401
- `permission_error` — 403
- `not_found_error` — 404
- `rate_limit_error` — 429
- `insufficient_credits_error` — 402
- `server_error` — 500

**Acceptance Criteria:**
- [ ] Consistent error format
- [ ] Proper HTTP status codes
- [ ] OpenAI-compatible error format
- [ ] Unit tests

---

## Dependencies

- Phase 1 (ConfigStore, LogStore)
- Phase 2 (RequestOrchestrator)
- Phase 3 (Provider Registry)
- `github.com/gin-gonic/gin` — HTTP framework
- `golang.org/x/crypto/bcrypt` — key hashing

---

## Testing Strategy

### Unit Tests
- Mock dependencies
- Test each handler in isolation
- Test middleware
- 95%+ coverage per file

### Integration Tests
- Real HTTP server (test mode)
- Test full request flow
- Test error scenarios
- Test streaming

### E2E Tests
- Test with real providers (test accounts)
- Test admin API workflows
- Test rate limiting
- Test credit deduction

---

## Deliverables

- [ ] HTTP server with graceful shutdown
- [ ] Authentication middleware
- [ ] Rate limit middleware
- [ ] Logging middleware
- [ ] Chat completion handler (stream + non-stream)
- [ ] Models handler
- [ ] Admin API (keys, accounts, combos, logs, settings)
- [ ] Error handling
- [ ] 95%+ test coverage
- [ ] Integration tests
- [ ] E2E tests
- [ ] Documentation (godoc + API docs)

---

## Estimated Effort

- HTTP server setup: 4 hours
- Auth middleware: 4 hours
- Rate limit middleware: 3 hours
- Logging middleware: 2 hours
- Chat handler: 8 hours
- Models handler: 2 hours
- Admin API (keys): 6 hours
- Admin API (accounts): 6 hours
- Admin API (combos): 4 hours
- Admin API (logs): 4 hours
- Admin API (settings): 2 hours
- Error handling: 3 hours
- Unit tests: 10 hours
- Integration tests: 6 hours
- E2E tests: 4 hours
- Documentation: 4 hours

**Total:** 72 hours (1.5 weeks)
