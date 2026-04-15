---
status: pending
priority: high
effort: 1 week
phase: 3
dependencies: [phase-1, phase-2]
---

# Phase 3: Provider Adapters (Week 3)

## Goals

Refactor all provider adapters to fix critical bugs, implement proper error handling, and ensure consistent behavior across all 7 providers.

## Critical Bugs to Fix

### Kiro EventStream
- **Issue:** CRC validation fails intermittently
- **Root cause:** Incorrect CRC32 calculation for prelude/message
- **Impact:** 30% of Kiro requests fail with "CRC mismatch"

### Anthropic SSE
- **Issue:** SSE format doesn't match Anthropic spec
- **Root cause:** Missing event type prefix, incorrect data format
- **Impact:** Streaming responses fail to parse

### OpenAI Codex
- **Issue:** Tool call translation incorrect
- **Root cause:** Codex uses different tool format than OpenAI
- **Impact:** Tool-based requests fail

## Tasks

### 3.1 Provider Interface

**File:** `internal/provider/interface.go`

Define unified provider interface:

```go
type Provider interface {
    Execute(ctx context.Context, account *domain.ProviderAccount, req *Request) (*Response, error)
    Stream(ctx context.Context, account *domain.ProviderAccount, req *Request) (<-chan StreamChunk, error)
    ValidateAccount(ctx context.Context, account *domain.ProviderAccount) error
    RefreshToken(ctx context.Context, account *domain.ProviderAccount) (*domain.Credentials, error)
}

type Request struct {
    Model       string
    Messages    []Message
    Stream      bool
    Temperature float64
    MaxTokens   int
    Tools       []Tool
    ToolChoice  interface{}
}

type Response struct {
    ID      string
    Model   string
    Choices []Choice
    Usage   Usage
}

type StreamChunk struct {
    ID      string
    Model   string
    Delta   Delta
    Usage   *Usage
    Done    bool
    Error   error
}

type Message struct {
    Role       string
    Content    string
    ToolCalls  []ToolCall
    ToolCallID string
}

type Tool struct {
    Type     string
    Function Function
}

type ToolCall struct {
    ID       string
    Type     string
    Function FunctionCall
}

type Usage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
}
```

**Acceptance Criteria:**
- [ ] Interface covers all provider capabilities
- [ ] Request/Response types are provider-agnostic
- [ ] Streaming support with error handling
- [ ] Token refresh support
- [ ] Account validation support

---

### 3.2 Kiro Provider (Fix EventStream CRC)

**File:** `internal/provider/kiro/kiro.go`

Fix AWS EventStream binary protocol:

```go
type KiroProvider struct {
    httpClient *http.Client
}

func NewKiroProvider() *KiroProvider
func (p *KiroProvider) Execute(ctx context.Context, account *domain.ProviderAccount, req *Request) (*Response, error)
func (p *KiroProvider) Stream(ctx context.Context, account *domain.ProviderAccount, req *Request) (<-chan StreamChunk, error)
func (p *KiroProvider) translateRequest(req *Request) *KiroRequest
func (p *KiroProvider) translateResponse(resp *KiroResponse) *Response
func (p *KiroProvider) parseEventStream(reader io.Reader) (<-chan StreamChunk, error)
```

**EventStream Frame Format:**
```
[4B total length][4B headers length][4B prelude CRC][headers][payload][4B message CRC]
```

**CRC Calculation (IEEE CRC32):**
```go
func calculatePreludeCRC(totalLen, headersLen uint32) uint32 {
    buf := make([]byte, 8)
    binary.BigEndian.PutUint32(buf[0:4], totalLen)
    binary.BigEndian.PutUint32(buf[4:8], headersLen)
    return crc32.ChecksumIEEE(buf)
}

func calculateMessageCRC(totalLen, headersLen, preludeCRC uint32, headers, payload []byte) uint32 {
    h := crc32.NewIEEE()
    
    // Write prelude
    binary.Write(h, binary.BigEndian, totalLen)
    binary.Write(h, binary.BigEndian, headersLen)
    binary.Write(h, binary.BigEndian, preludeCRC)
    
    // Write headers and payload
    h.Write(headers)
    h.Write(payload)
    
    return h.Sum32()
}
```

**Event Types:**
- `assistantResponseEvent` — text delta
- `codeEvent` — code block
- `toolUseEvent` — tool call
- `messageStopEvent` — end of message
- `contextUsageEvent` — token usage (estimated)
- `meteringEvent` — billing info
- `metricsEvent` — token usage (actual)

**Acceptance Criteria:**
- [ ] CRC validation passes 100% of the time
- [ ] All event types parsed correctly
- [ ] Token usage from metricsEvent (fallback to contextUsageEvent)
- [ ] Tool calls translated correctly
- [ ] Streaming works without errors
- [ ] Unit tests with real EventStream samples
- [ ] Integration test with live Kiro API

---

### 3.3 Anthropic Provider (Fix SSE Format)

**File:** `internal/provider/anthropic/anthropic.go`

Fix SSE format to match Anthropic spec:

```go
type AnthropicProvider struct {
    httpClient *http.Client
    apiKey     string
}

func NewAnthropicProvider() *AnthropicProvider
func (p *AnthropicProvider) Execute(ctx context.Context, account *domain.ProviderAccount, req *Request) (*Response, error)
func (p *AnthropicProvider) Stream(ctx context.Context, account *domain.ProviderAccount, req *Request) (<-chan StreamChunk, error)
func (p *AnthropicProvider) translateRequest(req *Request) *AnthropicRequest
func (p *AnthropicProvider) translateResponse(resp *AnthropicResponse) *Response
func (p *AnthropicProvider) parseSSE(reader io.Reader) (<-chan StreamChunk, error)
```

**Anthropic SSE Format:**
```
event: message_start
data: {"type":"message_start","message":{"id":"msg_123",...}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

event: message_delta
data: {"type":"message_delta","usage":{"output_tokens":10}}

event: message_stop
data: {"type":"message_stop"}
```

**Key Fixes:**
- Add `event:` prefix before `data:`
- Parse event type from `event:` line
- Handle all event types (message_start, content_block_start, content_block_delta, content_block_stop, message_delta, message_stop)
- Extract usage from message_delta

**Acceptance Criteria:**
- [ ] SSE parser handles all event types
- [ ] Streaming responses parse correctly
- [ ] Token usage extracted from message_delta
- [ ] Tool calls supported
- [ ] Unit tests with real SSE samples
- [ ] Integration test with live Anthropic API

---

### 3.4 OpenAI Provider (Fix Codex Translation)

**File:** `internal/provider/openai/openai.go`

Fix tool call translation for Codex:

```go
type OpenAIProvider struct {
    httpClient *http.Client
    baseURL    string
}

func NewOpenAIProvider(baseURL string) *OpenAIProvider
func (p *OpenAIProvider) Execute(ctx context.Context, account *domain.ProviderAccount, req *Request) (*Response, error)
func (p *OpenAIProvider) Stream(ctx context.Context, account *domain.ProviderAccount, req *Request) (<-chan StreamChunk, error)
func (p *OpenAIProvider) translateRequest(req *Request, isCodex bool) interface{}
func (p *OpenAIProvider) translateResponse(resp interface{}, isCodex bool) *Response
func (p *OpenAIProvider) isCodexModel(model string) bool
```

**Codex Tool Format:**
```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather",
        "parameters": {...}
      }
    }
  ]
}
```

**OpenAI Tool Format:**
```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather",
        "parameters": {...}
      }
    }
  ]
}
```

**Key Fixes:**
- Detect Codex models (starts with "Codex-")
- Translate tool calls to Codex format
- Translate tool results from Codex format
- Handle streaming tool calls

**Acceptance Criteria:**
- [ ] Codex models detected correctly
- [ ] Tool calls translated to Codex format
- [ ] Tool results translated from Codex format
- [ ] Streaming tool calls work
- [ ] Unit tests with Codex samples
- [ ] Integration test with live OpenAI API

---

### 3.5 GLM Provider

**File:** `internal/provider/glm/glm.go`

Implement GLM (Zhipu AI) provider:

```go
type GLMProvider struct {
    httpClient *http.Client
    baseURL    string
}

func NewGLMProvider() *GLMProvider
func (p *GLMProvider) Execute(ctx context.Context, account *domain.ProviderAccount, req *Request) (*Response, error)
func (p *GLMProvider) Stream(ctx context.Context, account *domain.ProviderAccount, req *Request) (<-chan StreamChunk, error)
func (p *GLMProvider) translateRequest(req *Request) *GLMRequest
func (p *GLMProvider) translateResponse(resp *GLMResponse) *Response
```

**API:** `https://api.z.ai/v1/chat/completions` (OpenAI-compatible)

**Acceptance Criteria:**
- [ ] OpenAI-compatible API calls
- [ ] Streaming support
- [ ] Token usage extraction
- [ ] Error handling
- [ ] Unit tests
- [ ] Integration test with live GLM API

---

### 3.6 MiniMax Provider

**File:** `internal/provider/minimax/minimax.go`

Implement MiniMax provider:

```go
type MiniMaxProvider struct {
    httpClient *http.Client
    baseURL    string
}

func NewMiniMaxProvider() *MiniMaxProvider
func (p *MiniMaxProvider) Execute(ctx context.Context, account *domain.ProviderAccount, req *Request) (*Response, error)
func (p *MiniMaxProvider) Stream(ctx context.Context, account *domain.ProviderAccount, req *Request) (<-chan StreamChunk, error)
func (p *MiniMaxProvider) translateRequest(req *Request) *MiniMaxRequest
func (p *MiniMaxProvider) translateResponse(resp *MiniMaxResponse) *Response
```

**API:** `https://api.minimax.io/v1/chat/completions` (OpenAI-compatible)

**Acceptance Criteria:**
- [ ] OpenAI-compatible API calls
- [ ] Streaming support
- [ ] Token usage extraction
- [ ] Error handling
- [ ] Unit tests
- [ ] Integration test with live MiniMax API

---

### 3.7 Qwen Provider

**File:** `internal/provider/qwen/qwen.go`

Implement Qwen (Alibaba) provider:

```go
type QwenProvider struct {
    httpClient *http.Client
    baseURL    string
}

func NewQwenProvider() *QwenProvider
func (p *QwenProvider) Execute(ctx context.Context, account *domain.ProviderAccount, req *Request) (*Response, error)
func (p *QwenProvider) Stream(ctx context.Context, account *domain.ProviderAccount, req *Request) (<-chan StreamChunk, error)
func (p *QwenProvider) translateRequest(req *Request) *QwenRequest
func (p *QwenProvider) translateResponse(resp *QwenResponse) *Response
```

**API:** `https://portal.qwen.ai/v1/chat/completions` (OpenAI-compatible)

**Acceptance Criteria:**
- [ ] OpenAI-compatible API calls
- [ ] Streaming support
- [ ] Token usage extraction
- [ ] Error handling
- [ ] Unit tests
- [ ] Integration test with live Qwen API

---

### 3.8 Provider Registry

**File:** `internal/provider/registry.go`

Implement provider registry for dynamic lookup:

```go
type Registry struct {
    providers map[string]Provider
    mu        sync.RWMutex
}

func NewRegistry() *Registry
func (r *Registry) Register(providerID string, provider Provider)
func (r *Registry) Get(providerID string) (Provider, bool)
func (r *Registry) List() []string
```

**Acceptance Criteria:**
- [ ] Thread-safe registration
- [ ] Get returns provider by ID
- [ ] List returns all provider IDs
- [ ] Unit tests

---

### 3.9 Provider Tests

**Files:**
- `internal/provider/kiro/kiro_test.go`
- `internal/provider/anthropic/anthropic_test.go`
- `internal/provider/openai/openai_test.go`
- `internal/provider/glm/glm_test.go`
- `internal/provider/minimax/minimax_test.go`
- `internal/provider/qwen/qwen_test.go`

**Test Coverage:**
- [ ] Request translation (generic → provider-specific)
- [ ] Response translation (provider-specific → generic)
- [ ] Streaming parsing (SSE, EventStream)
- [ ] Error handling (401, 429, 500)
- [ ] Token usage extraction
- [ ] Tool call translation
- [ ] CRC validation (Kiro)
- [ ] Integration tests with live APIs

**Target:** 95%+ coverage per provider

---

## Dependencies

- Phase 1 (ConfigStore)
- Phase 2 (ProviderRouter)
- `hash/crc32` — Kiro CRC validation
- `bufio` — SSE parsing
- `encoding/binary` — EventStream parsing

---

## Testing Strategy

### Unit Tests
- Mock HTTP responses
- Test request/response translation
- Test streaming parsers
- Race detector enabled

### Integration Tests
- Live API calls (with test accounts)
- Verify all providers work end-to-end
- Test error scenarios (401, 429, 500)

### Performance Tests
- 100 concurrent requests per provider
- Measure p99 latency
- Verify no memory leaks

---

## Deliverables

- [ ] Unified provider interface
- [ ] Kiro provider with fixed CRC
- [ ] Anthropic provider with fixed SSE
- [ ] OpenAI provider with fixed Codex
- [ ] GLM, MiniMax, Qwen providers
- [ ] Provider registry
- [ ] 95%+ test coverage per provider
- [ ] Integration tests with live APIs
- [ ] Documentation (godoc)

---

## Estimated Effort

- Provider interface: 4 hours
- Kiro provider (fix CRC): 10 hours
- Anthropic provider (fix SSE): 6 hours
- OpenAI provider (fix Codex): 6 hours
- GLM provider: 4 hours
- MiniMax provider: 4 hours
- Qwen provider: 4 hours
- Provider registry: 2 hours
- Unit tests: 10 hours
- Integration tests: 6 hours
- Documentation: 2 hours

**Total:** 58 hours (1 week + 2 days overtime)
