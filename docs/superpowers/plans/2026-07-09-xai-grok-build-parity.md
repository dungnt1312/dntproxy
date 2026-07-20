# xAI / Grok Build Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring dntproxy's xAI/Grok provider from minimal chat streaming to CLIProxyAPI-level Grok Build parity for production agent use: full tool calling, reasoning/thinking continuity, Responses + compact APIs, composer session headers, and unified image path — while keeping the existing OpenAI-compatible client surface and dntproxy architecture.

**Architecture:** Keep `port.ProviderExecutor` as the chat entrypoint, but grow `internal/adapter/xai` into a focused Responses runtime (request sanitize, tool normalize, stream event translation, reasoning replay, session headers). Add thin HTTP routes for `/v1/responses` and `/v1/responses/compact`. Do **not** port WebSocket or video in this plan (explicitly deferred). Prefer rewrite-by-behavior from `temp/CLIProxyAPI` rather than copying large files.

**Tech Stack:** Go 1.25+, Gin, existing `ProviderExecutor`, JSON DB credentials, token refresh singleflight, SSE over `net/http`, unit tests with `httptest.Server`, optional in-memory replay cache.

## Global Constraints

- File naming stays kebab-case (`request-translator.go`, `reasoning-replay.go`).
- Keep files under ~200 lines where practical; split helpers instead of growing one mega-executor.
- Domain types stay free of external deps; adapter implements ports.
- Public chat surface remains OpenAI-compatible SSE (`chat.completion.chunk`) unless the client hits `/v1/responses`.
- Do not break existing OAuth login/import/refresh paths already shipping.
- Reference implementation only: `temp/CLIProxyAPI/internal/runtime/executor/xai_*.go`, `temp/CLIProxyAPI/internal/auth/xai/`, `temp/CLIProxyAPI/internal/cache/xai_reasoning_replay_cache.go`, `temp/CLIProxyAPI/internal/signature/grok_validation.go`. Rewrite into dntproxy style.
- Out of scope for this plan: WebSocket Responses transport, video APIs, Claude/Gemini→xAI multi-protocol matrix, plugin SDK.
- Target = **Mốc B (Grok Build parity)** from the comparison notes. Mốc C (WS/video) is a follow-up plan.

## Current State (baseline)

Already present and should keep working:

- OAuth discovery/PKCE/exchange/refresh: `internal/adapter/auth/xai.go`
- Auth HTTP + import file: `internal/adapter/http/auth-xai-handler.go`
- Minimal chat→Responses + text delta stream: `internal/adapter/xai/{executor,translator}.go`
- Image gen/edit hooks: `internal/adapter/openai/xai-image-translator.go`, `internal/adapter/http/image-handler.go`
- Provider registration: `cmd/dntproxy/main.go` → `providers.RegisterExecutor("xai", xai.NewExecutor())`
- Models: `xai/grok-*` in `internal/domain/model-definition.go`

Missing vs CLIProxyAPI (this plan):

1. Tool calling multi-turn (request + stream)
2. Reasoning effort / thinking suffix
3. Reasoning replay cache + signature validation
4. Request sanitize / tool normalize
5. Composer session headers (`x-grok-conv-id`, `prompt_cache_key`)
6. Native `/v1/responses` + `/v1/responses/compact`
7. Stronger usage/error handling
8. Image path consistency (headers/auth) under xai package

## File Structure

### Create

- `internal/adapter/xai/model.go` — strip public prefixes, parse thinking suffix, model capability helpers
- `internal/adapter/xai/model_test.go`
- `internal/adapter/xai/tools.go` — normalize tools / tool_choice / parallel_tool_calls
- `internal/adapter/xai/tools_test.go`
- `internal/adapter/xai/request-translator.go` — OpenAI chat → Responses body
- `internal/adapter/xai/request-translator_test.go`
- `internal/adapter/xai/response-translator.go` — Responses SSE events → OpenAI chat chunks
- `internal/adapter/xai/response-translator_test.go`
- `internal/adapter/xai/sanitize.go` — strip unsupported reasoning fields, cleanup body
- `internal/adapter/xai/sanitize_test.go`
- `internal/adapter/xai/headers.go` — Authorization, Accept, session headers
- `internal/adapter/xai/headers_test.go`
- `internal/adapter/xai/signature.go` — Grok encrypted_content validation helpers
- `internal/adapter/xai/signature_test.go`
- `internal/adapter/xai/reasoning-replay.go` — scope + inject/extract reasoning items
- `internal/adapter/xai/reasoning-replay_test.go`
- `internal/adapter/xai/replay-cache.go` — in-memory TTL cache for encrypted reasoning items
- `internal/adapter/xai/replay-cache_test.go`
- `internal/adapter/xai/compact.go` — compact request builder/response helpers
- `internal/adapter/xai/compact_test.go`
- `internal/adapter/http/xai-responses-handler.go` — `/v1/responses`, `/v1/responses/compact`
- `internal/adapter/http/xai-responses-handler_test.go`

### Modify

- `internal/adapter/xai/translator.go` — thin re-exports or delete after split (keep package API stable for tests during migration)
- `internal/adapter/xai/executor.go` — rewrite to use new prepare/stream pipeline
- `internal/adapter/xai/executor_test.go` — expand coverage for tools/reasoning/errors
- `internal/adapter/xai/translator_test.go` — migrate assertions to new files, then delete obsolete cases
- `internal/adapter/auth/xai.go` — optional proxy-aware client helper only if needed by tests
- `internal/adapter/auth/token-refresh.go` — ensure refresh path stores tokenEndpoint consistently (no behavior break)
- `internal/adapter/http/router.go` — register responses routes
- `internal/adapter/http/image-handler.go` — call shared xai header/baseURL helpers
- `internal/domain/model-definition.go` — mark thinking-capable Grok models via metadata if missing
- `docs/project-changelog.md` or short note in README only if user-facing endpoints change

### Reference only (do not import)

- `temp/CLIProxyAPI/internal/runtime/executor/xai_executor.go`
- `temp/CLIProxyAPI/internal/runtime/executor/xai_reasoning_replay.go`
- `temp/CLIProxyAPI/internal/cache/xai_reasoning_replay_cache.go`
- `temp/CLIProxyAPI/internal/signature/grok_validation.go`
- `temp/CLIProxyAPI/internal/thinking/provider/xai/apply.go`

## Architecture Notes for Implementers

### Chat path (must keep working)

```
POST /v1/chat/completions { model: "xai/grok-...", messages, tools, stream }
  → chat-service resolves provider=xai + credentials
  → xai.Executor.Execute(model, body, creds, reqlog)
      1. Parse model (strip xai/grok prefix, thinking suffix)
      2. Translate chat → Responses JSON
      3. Sanitize + normalize tools
      4. Apply reasoning effort
      5. Inject reasoning replay items if cache hit
      6. Apply session headers / prompt_cache_key
      7. POST {baseURL}/responses (SSE)
      8. Translate events → OpenAI chat chunks
      9. On completed: cache reasoning items + usage
```

### Native Responses path (new)

```
POST /v1/responses
  → resolve model/provider/creds like chat
  → prepare body (less chat translation; still sanitize/tools/thinking/replay)
  → stream or return upstream Responses events (pass-through with light fixups)
```

### Compact path (new)

```
POST /v1/responses/compact
  → same auth/model resolve
  → strip stream/tools/compaction_trigger
  → POST {baseURL}/responses/compact
  → return compact JSON
```

### Package API to stabilize early

```go
// model.go
func CanonicalModel(model string) string
func ParseModel(model string) (base string, effort string) // effort: "", low, medium, high, ...

// request-translator.go
func TranslateChatToResponses(model string, body []byte) ([]byte, error)

// tools.go
func NormalizeTools(body []byte) []byte
func NormalizeToolChoiceForTools(body []byte) []byte

// sanitize.go
func SanitizeResponsesBody(body []byte, model string) []byte
func ApplyReasoningEffort(body []byte, effort string) []byte

// response-translator.go
type StreamState struct { /* exported fields used by executor */ }
func NewStreamState(model string) *StreamState
func TranslateResponsesEvent(data []byte, state *StreamState) string

// headers.go
func ApplyRequestHeaders(req *http.Request, token string, stream bool, sessionID string)
func ResolveBaseURL(creds *domain.Credentials) string
func ResolveSessionID(model string, body []byte, creds *domain.Credentials) string

// reasoning-replay.go + replay-cache.go
func ApplyReplay(body []byte, model, sessionKey string, cache *ReplayCache) ([]byte, error)
func CacheFromCompleted(completed []byte, model, sessionKey string, cache *ReplayCache)

// signature.go
func IsValidGrokEncryptedContent(raw string) bool

// compact.go
func BuildCompactRequest(body []byte) ([]byte, error)
```

---

### Task 1: Model helpers (prefix + thinking suffix)

**Files:**
- Create: `internal/adapter/xai/model.go`
- Create: `internal/adapter/xai/model_test.go`

**Interfaces:**
- Consumes: public model strings like `xai/grok-4.3`, `grok/grok-4.3-high`, `grok-4.20-0309-reasoning`
- Produces:
  - `func CanonicalModel(model string) string`
  - `func ParseModel(model string) (base string, effort string)`
  - `func SupportsReasoningEffort(baseModel string) bool`

- [ ] **Step 1: Write failing tests**

```go
package xai

import "testing"

func TestCanonicalModel(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"xai/grok-4.3", "grok-4.3"},
		{"grok/grok-4.3", "grok-4.3"},
		{"grok-4.3", "grok-4.3"},
		{"xai/grok-4.3-high", "grok-4.3-high"},
	}
	for _, tt := range tests {
		if got := CanonicalModel(tt.in); got != tt.want {
			t.Fatalf("CanonicalModel(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseModelThinkingSuffix(t *testing.T) {
	base, effort := ParseModel("xai/grok-4.3-high")
	if base != "grok-4.3" || effort != "high" {
		t.Fatalf("got base=%q effort=%q", base, effort)
	}
	base, effort = ParseModel("grok-4.20-0309-reasoning")
	if base != "grok-4.20-0309-reasoning" || effort != "" {
		t.Fatalf("reasoning model should not strip itself: base=%q effort=%q", base, effort)
	}
	base, effort = ParseModel("grok-3-mini-low")
	if base != "grok-3-mini" || effort != "low" {
		t.Fatalf("got base=%q effort=%q", base, effort)
	}
}

func TestSupportsReasoningEffort(t *testing.T) {
	if !SupportsReasoningEffort("grok-3-mini") {
		t.Fatal("expected grok-3-mini to support effort")
	}
	if SupportsReasoningEffort("grok-imagine-image") {
		t.Fatal("image model should not support effort")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/xai -run "TestCanonicalModel|TestParseModelThinkingSuffix|TestSupportsReasoningEffort" -count=1`

Expected: FAIL (functions undefined)

- [ ] **Step 3: Implement `model.go`**

```go
package xai

import "strings"

var thinkingSuffixes = []string{"xhigh", "high", "medium", "low", "minimal", "none"}

func CanonicalModel(model string) string {
	model = strings.TrimSpace(model)
	if i := strings.IndexByte(model, '/'); i >= 0 {
		prefix := model[:i]
		if prefix == "xai" || prefix == "grok" {
			model = model[i+1:]
		}
	}
	return model
}

func ParseModel(model string) (base string, effort string) {
	base = CanonicalModel(model)
	for _, suffix := range thinkingSuffixes {
		token := "-" + suffix
		if strings.HasSuffix(base, token) {
			trimmed := strings.TrimSuffix(base, token)
			if trimmed == "" {
				return base, ""
			}
			// Do not strip if the whole id ends with a model name that includes the token naturally
			// and is listed as a dedicated model id (e.g. keep explicit non-suffix models intact).
			return trimmed, suffix
		}
	}
	return base, ""
}

func SupportsReasoningEffort(baseModel string) bool {
	baseModel = CanonicalModel(baseModel)
	if strings.Contains(baseModel, "imagine") || strings.Contains(baseModel, "image") || strings.Contains(baseModel, "video") {
		return false
	}
	// Default: allow effort for text Grok models; executor still strips if upstream rejects.
	return strings.HasPrefix(baseModel, "grok-")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/xai -run "TestCanonicalModel|TestParseModelThinkingSuffix|TestSupportsReasoningEffort" -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/xai/model.go internal/adapter/xai/model_test.go
git commit -m "$(cat <<'EOF'
feat(xai): add model canonicalization and thinking suffix parsing

EOF
)"
```

---

### Task 2: Tool normalization

**Files:**
- Create: `internal/adapter/xai/tools.go`
- Create: `internal/adapter/xai/tools_test.go`

**Interfaces:**
- Consumes: Responses-shaped JSON body bytes with optional `tools`, `tool_choice`, `parallel_tool_calls`
- Produces:
  - `func NormalizeTools(body []byte) []byte`
  - `func NormalizeToolChoiceForTools(body []byte) []byte`
  - `func ChatToolsToResponses(tools []byte) ([]byte, error)` if needed by request translator

Behavior (from CLIProxyAPI):

- Accept OpenAI chat tools `{type:"function", function:{name,description,parameters}}` and convert to Responses `{type:"function", name, description, parameters}`
- Accept already-flat Responses tools
- Expand `namespace` tools into nested tools when present
- Allow built-ins: `web_search`, `image_generation`, `tool_search`, `custom` (pass through)
- If tools empty after filter: delete `tools`, `tool_choice`, `parallel_tool_calls`

- [ ] **Step 1: Write failing tests**

```go
package xai

import (
	"encoding/json"
	"testing"
)

func TestNormalizeToolsConvertsOpenAIFunctionShape(t *testing.T) {
	in := []byte(`{
	  "tools":[{
	    "type":"function",
	    "function":{"name":"sum","description":"add","parameters":{"type":"object"}}
	  }]
	}`)
	out := NormalizeTools(in)
	var payload struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Tools) != 1 {
		t.Fatalf("tools=%v", payload.Tools)
	}
	if payload.Tools[0]["name"] != "sum" {
		t.Fatalf("expected flat name, got %#v", payload.Tools[0])
	}
	if _, ok := payload.Tools[0]["function"]; ok {
		t.Fatalf("function wrapper should be removed: %#v", payload.Tools[0])
	}
}

func TestNormalizeToolChoiceRemovedWhenNoTools(t *testing.T) {
	in := []byte(`{"tools":[],"tool_choice":"auto","parallel_tool_calls":true}`)
	out := NormalizeToolChoiceForTools(NormalizeTools(in))
	var payload map[string]interface{}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["tools"]; ok {
		t.Fatalf("empty tools should be deleted: %v", payload)
	}
	if _, ok := payload["tool_choice"]; ok {
		t.Fatalf("tool_choice should be deleted: %v", payload)
	}
	if _, ok := payload["parallel_tool_calls"]; ok {
		t.Fatalf("parallel_tool_calls should be deleted: %v", payload)
	}
}

func TestNormalizeToolsPassesWebSearch(t *testing.T) {
	in := []byte(`{"tools":[{"type":"web_search"}]}`)
	out := NormalizeTools(in)
	if !json.Valid(out) {
		t.Fatal("invalid json")
	}
	var payload struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	_ = json.Unmarshal(out, &payload)
	if len(payload.Tools) != 1 || payload.Tools[0]["type"] != "web_search" {
		t.Fatalf("got %#v", payload.Tools)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/xai -run "TestNormalizeTools|TestNormalizeToolChoice" -count=1`

Expected: FAIL

- [ ] **Step 3: Implement `tools.go`**

Use `encoding/json` only (no new deps). Keep helpers unexported except the two public normalize functions.

Key conversion for OpenAI function tools:

```go
// pseudo-structure for each tool object after normalize
{
  "type": "function",
  "name": "...",
  "description": "...",
  "parameters": { ... }
}
```

If `type` is missing but `function.name` exists, treat as function.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/xai -run "TestNormalizeTools|TestNormalizeToolChoice" -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/xai/tools.go internal/adapter/xai/tools_test.go
git commit -m "$(cat <<'EOF'
feat(xai): normalize Responses tools and tool_choice

EOF
)"
```

---

### Task 3: Request translator rewrite (chat → Responses)

**Files:**
- Create: `internal/adapter/xai/request-translator.go`
- Create: `internal/adapter/xai/request-translator_test.go`
- Modify: `internal/adapter/xai/translator.go` (delegate `TranslateChatToResponses` to new file or move and keep alias)

**Interfaces:**
- Consumes: OpenAI chat completions JSON
- Produces: `func TranslateChatToResponses(model string, body []byte) ([]byte, error)`
- Output Responses fields:
  - `model`, `input`, `instructions`, `stream`, `temperature`, `top_p`, `max_output_tokens`, `tools`, `tool_choice`, `parallel_tool_calls` when present

Critical behaviors:

1. `system` / `developer` → concatenated `instructions`
2. `user` / `assistant` → `input[]` with role + content (preserve multimodal array content)
3. `assistant` tool_calls → Responses `function_call` items (type/name/arguments/call_id)
4. `tool` role → Responses `function_call_output` items (`call_id`, `output`)
5. tools converted via Task 2
6. `max_completion_tokens` / `max_tokens` → `max_output_tokens`
7. force `stream: true` for executor chat path (caller may override later for native responses)

- [ ] **Step 1: Write failing tests**

```go
package xai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTranslateChatToResponses_Basic(t *testing.T) {
	body := []byte(`{
		"model":"xai/grok-4.3",
		"messages":[
			{"role":"system","content":"be concise"},
			{"role":"user","content":"hello"}
		],
		"temperature":0.2,
		"max_tokens":123
	}`)
	got, err := TranslateChatToResponses("grok-4.3", body)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	_ = json.Unmarshal(got, &payload)
	if payload["instructions"] != "be concise" {
		t.Fatalf("instructions=%v", payload["instructions"])
	}
	if payload["max_output_tokens"].(float64) != 123 {
		t.Fatalf("max_output_tokens=%v", payload["max_output_tokens"])
	}
	if payload["stream"] != true {
		t.Fatalf("stream=%v", payload["stream"])
	}
}

func TestTranslateChatToResponses_ToolRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"grok-4.3",
		"messages":[
			{"role":"user","content":"sum 1 2"},
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_1","type":"function","function":{"name":"sum","arguments":"{\"a\":1,\"b\":2}"}}
			]},
			{"role":"tool","tool_call_id":"call_1","content":"3"}
		],
		"tools":[{"type":"function","function":{"name":"sum","parameters":{"type":"object"}}}]
	}`)
	got, err := TranslateChatToResponses("grok-4.3", body)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, `"type":"function_call"`) {
		t.Fatalf("missing function_call item: %s", s)
	}
	if !strings.Contains(s, `"type":"function_call_output"`) {
		t.Fatalf("missing function_call_output item: %s", s)
	}
	if !strings.Contains(s, `"call_id":"call_1"`) {
		t.Fatalf("missing call_id: %s", s)
	}
	// tools must be flat responses shape
	var payload struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	_ = json.Unmarshal(got, &payload)
	if payload.Tools[0]["name"] != "sum" {
		t.Fatalf("tools not normalized: %#v", payload.Tools[0])
	}
}

func TestTranslateChatToResponses_MultimodalUser(t *testing.T) {
	body := []byte(`{
		"messages":[{"role":"user","content":[
			{"type":"text","text":"what is this?"},
			{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}
		]}]
	}`)
	got, err := TranslateChatToResponses("grok-4.3", body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "image_url") && !strings.Contains(string(got), "input_image") {
		// Accept either passthrough image_url or converted input_image depending on chosen mapping.
		t.Fatalf("multimodal content lost: %s", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/xai -run "TestTranslateChatToResponses_" -count=1`

Expected: FAIL on tool/multimodal cases (basic may already pass via old translator)

- [ ] **Step 3: Implement request translator**

Recommended input item shapes:

```json
{"role":"user","content":"..."}
{"type":"function_call","id":"call_1","call_id":"call_1","name":"sum","arguments":"{\"a\":1}"}
{"type":"function_call_output","call_id":"call_1","output":"3"}
```

Notes:

- Prefer including both `id` and `call_id` if upstream samples require one of them.
- If assistant content is empty/null and only tool_calls exist, do not emit empty assistant message.
- Keep old function name `TranslateChatToResponses` to avoid churn.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/adapter/xai -run "TestTranslateChatToResponses" -count=1`

Expected: PASS (update/remove obsolete `TestTranslateChatToResponsesRejectsUnsupportedTool` — web_search is now allowed)

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/xai/request-translator.go internal/adapter/xai/request-translator_test.go internal/adapter/xai/translator.go internal/adapter/xai/translator_test.go
git commit -m "$(cat <<'EOF'
feat(xai): rewrite chat-to-responses translator with tool round-trip

EOF
)"
```

---

### Task 4: Response stream translator (tools + text + usage)

**Files:**
- Create: `internal/adapter/xai/response-translator.go`
- Create: `internal/adapter/xai/response-translator_test.go`
- Modify: `internal/adapter/xai/translator.go` (delegate or remove old event translation)

**Interfaces:**
- Produces:
  - `type StreamState struct`
  - `func NewStreamState(model string) *StreamState`
  - `func TranslateResponsesEvent(data []byte, state *StreamState) string`
  - `func (s *StreamState) Usage() (in, out int)`

Must handle event types:

| Upstream event | OpenAI chat chunk behavior |
|---|---|
| `response.created` / `response.in_progress` | optional ignore or set id |
| `response.output_text.delta` | `delta.content` |
| `response.output_item.added` (function_call) | start tool_call slot |
| `response.function_call_arguments.delta` | `delta.tool_calls[i].function.arguments` |
| `response.output_item.done` (function_call) | ensure name/id present |
| `response.completed` | finish_reason `stop` or `tool_calls`, usage, `[DONE]` |
| `response.incomplete` | finish_reason `length` |
| `response.failed` / `error` | surface error text once |

OpenAI tool call streaming shape:

```json
{
  "choices":[{
    "index":0,
    "delta":{
      "tool_calls":[{
        "index":0,
        "id":"call_1",
        "type":"function",
        "function":{"name":"sum","arguments":"{\"a\""}
      }]
    }
  }]
}
```

- [ ] **Step 1: Write failing tests**

```go
package xai

import (
	"strings"
	"testing"
)

func TestTranslateResponsesEvent_TextDelta(t *testing.T) {
	st := NewStreamState("grok-4.3")
	got := TranslateResponsesEvent([]byte(`{"type":"response.output_text.delta","delta":"hi"}`), st)
	if !strings.Contains(got, `"content":"hi"`) {
		t.Fatalf("got %s", got)
	}
}

func TestTranslateResponsesEvent_ToolCallStream(t *testing.T) {
	st := NewStreamState("grok-4.3")
	_ = TranslateResponsesEvent([]byte(`{
		"type":"response.output_item.added",
		"output_index":0,
		"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"sum","arguments":""}
	}`), st)
	args := TranslateResponsesEvent([]byte(`{
		"type":"response.function_call_arguments.delta",
		"output_index":0,
		"delta":"{\"a\":1}"
	}`), st)
	if !strings.Contains(args, `"tool_calls"`) || !strings.Contains(args, `"sum"`) {
		t.Fatalf("args chunk=%s", args)
	}
	done := TranslateResponsesEvent([]byte(`{
		"type":"response.completed",
		"response":{
			"status":"completed",
			"output":[{"type":"function_call","call_id":"call_1","name":"sum","arguments":"{\"a\":1}"}],
			"usage":{"input_tokens":11,"output_tokens":5,"total_tokens":16}
		}
	}`), st)
	if !strings.Contains(done, `"finish_reason":"tool_calls"`) && !strings.Contains(done, `"finish_reason":"stop"`) {
		// Prefer tool_calls when output has function_call.
		t.Fatalf("completed chunk=%s", done)
	}
	in, out := st.Usage()
	if in != 11 || out != 5 {
		t.Fatalf("usage in=%d out=%d", in, out)
	}
}
```

- [ ] **Step 2: Run tests (expect FAIL)**

Run: `go test ./internal/adapter/xai -run "TestTranslateResponsesEvent_" -count=1`

- [ ] **Step 3: Implement response translator**

Implementation tips:

- Keep `map[int]toolCallState` keyed by `output_index`
- On first sight of a function_call item, emit name/id chunk if not emitted
- On completed: if any tool call seen and no plain text finish yet, finish_reason=`tool_calls`
- Always append `data: [DONE]\n\n` exactly once on terminal events
- Ignore unknown event types (return "")

- [ ] **Step 4: Run tests (expect PASS)**

Run: `go test ./internal/adapter/xai -run "TestTranslateResponsesEvent" -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/xai/response-translator.go internal/adapter/xai/response-translator_test.go internal/adapter/xai/translator.go internal/adapter/xai/translator_test.go
git commit -m "$(cat <<'EOF'
feat(xai): stream function-call events as OpenAI tool_calls chunks

EOF
)"
```

---

### Task 5: Sanitize + reasoning effort

**Files:**
- Create: `internal/adapter/xai/sanitize.go`
- Create: `internal/adapter/xai/sanitize_test.go`

**Interfaces:**
- `func SanitizeResponsesBody(body []byte, baseModel string) []byte`
- `func ApplyReasoningEffort(body []byte, effort string) []byte`

Rules:

1. `ApplyReasoningEffort`: if effort non-empty and model supports it, set `reasoning.effort`
2. `SanitizeResponsesBody`:
   - if model does **not** support effort, delete `reasoning.effort` and empty `reasoning` object
   - delete `previous_response_id` for plain chat executor path (native responses handler may keep it)
   - leave tools alone (already normalized)

- [ ] **Step 1: Failing tests**

```go
package xai

import (
	"encoding/json"
	"testing"
)

func TestApplyReasoningEffort(t *testing.T) {
	body := []byte(`{"model":"grok-3-mini"}`)
	out := ApplyReasoningEffort(body, "high")
	var payload map[string]interface{}
	_ = json.Unmarshal(out, &payload)
	reasoning := payload["reasoning"].(map[string]interface{})
	if reasoning["effort"] != "high" {
		t.Fatalf("got %v", payload)
	}
}

func TestSanitizeStripsEffortForImageModel(t *testing.T) {
	body := []byte(`{"model":"grok-imagine-image","reasoning":{"effort":"high"}}`)
	out := SanitizeResponsesBody(body, "grok-imagine-image")
	var payload map[string]interface{}
	_ = json.Unmarshal(out, &payload)
	if _, ok := payload["reasoning"]; ok {
		t.Fatalf("reasoning should be removed: %v", payload)
	}
}
```

- [ ] **Step 2: Run fail**

Run: `go test ./internal/adapter/xai -run "TestApplyReasoningEffort|TestSanitizeStripsEffort" -count=1`

- [ ] **Step 3: Implement**

Use encoding/json mutate-and-marshal, or careful map edits. Avoid adding `tidwall/sjson` dependency unless already in go.mod (it is not required in dntproxy today).

- [ ] **Step 4: Run pass**

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/xai/sanitize.go internal/adapter/xai/sanitize_test.go
git commit -m "$(cat <<'EOF'
feat(xai): apply and sanitize reasoning.effort by model capability

EOF
)"
```

---

### Task 6: Signature validation + reasoning replay cache

**Files:**
- Create: `internal/adapter/xai/signature.go`
- Create: `internal/adapter/xai/signature_test.go`
- Create: `internal/adapter/xai/replay-cache.go`
- Create: `internal/adapter/xai/replay-cache_test.go`
- Create: `internal/adapter/xai/reasoning-replay.go`
- Create: `internal/adapter/xai/reasoning-replay_test.go`

**Interfaces:**
- `func IsValidGrokEncryptedContent(raw string) bool`
- `type ReplayCache struct` + `func NewReplayCache(ttl time.Duration, maxEntries int) *ReplayCache`
- `func (c *ReplayCache) Put(model, sessionKey string, items []json.RawMessage)`
- `func (c *ReplayCache) Get(model, sessionKey string) ([]json.RawMessage, bool)`
- `func ApplyReplay(body []byte, model, sessionKey string, cache *ReplayCache) ([]byte, error)`
- `func CacheFromCompleted(completed []byte, model, sessionKey string, cache *ReplayCache)`
- `func SessionKeyFrom(creds *domain.Credentials, sessionID string) string`

Behavior:

1. Validate encrypted content is Grok-like (not Claude/Codex signature) using lightweight checks inspired by CLIProxyAPI (`signature/grok_validation.go`) — base64-ish alphabet + entropy heuristic is enough; do not overfit.
2. In-memory cache keyed by `model + "\x00" + sessionKey`
3. Default TTL 30m, max 1024 entries, simple LRU/evict-oldest
4. `ApplyReplay` prepends/inserts cached reasoning (+ related function_call items if required) into `input` only when body does not already include valid reasoning encrypted content
5. `CacheFromCompleted` extracts from `response.output` items with types `reasoning`, `function_call`, `custom_tool_call`

- [ ] **Step 1: Failing tests**

```go
package xai

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestIsValidGrokEncryptedContent_RejectsEmpty(t *testing.T) {
	if IsValidGrokEncryptedContent("") {
		t.Fatal("empty should be invalid")
	}
}

func TestReplayCacheRoundTrip(t *testing.T) {
	c := NewReplayCache(time.Minute, 10)
	items := []json.RawMessage{json.RawMessage(`{"type":"reasoning","encrypted_content":"abc"}`)}
	c.Put("grok-4.3", "sess-1", items)
	got, ok := c.Get("grok-4.3", "sess-1")
	if !ok || len(got) != 1 {
		t.Fatalf("get failed: ok=%v got=%v", ok, got)
	}
}

func TestApplyReplayInjectsWhenMissing(t *testing.T) {
	c := NewReplayCache(time.Minute, 10)
	c.Put("grok-4.3", "sess-1", []json.RawMessage{
		json.RawMessage(`{"type":"reasoning","encrypted_content":"ZW5jcnlwdGVk"}`),
	})
	body := []byte(`{"model":"grok-4.3","input":[{"role":"user","content":"continue"}]}`)
	out, err := ApplyReplay(body, "grok-4.3", "sess-1", c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"encrypted_content":"ZW5jcnlwdGVk"`) {
		t.Fatalf("replay not injected: %s", out)
	}
}

func TestCacheFromCompletedStoresReasoning(t *testing.T) {
	c := NewReplayCache(time.Minute, 10)
	completed := []byte(`{
		"type":"response.completed",
		"response":{"output":[
			{"type":"reasoning","encrypted_content":"ZW5jcnlwdGVk"},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}
		]}
	}`)
	CacheFromCompleted(completed, "grok-4.3", "sess-1", c)
	got, ok := c.Get("grok-4.3", "sess-1")
	if !ok || len(got) == 0 {
		t.Fatal("expected cached reasoning")
	}
}
```

- [ ] **Step 2: Run fail**

Run: `go test ./internal/adapter/xai -run "TestIsValidGrok|TestReplay|TestApplyReplay|TestCacheFromCompleted" -count=1`

- [ ] **Step 3: Implement cache + replay**

Package-level default cache for executor:

```go
var defaultReplayCache = NewReplayCache(30*time.Minute, 1024)
```

Allow tests to inject cache via executor field later.

- [ ] **Step 4: Run pass**

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/xai/signature.go internal/adapter/xai/signature_test.go \
  internal/adapter/xai/replay-cache.go internal/adapter/xai/replay-cache_test.go \
  internal/adapter/xai/reasoning-replay.go internal/adapter/xai/reasoning-replay_test.go
git commit -m "$(cat <<'EOF'
feat(xai): add grok reasoning replay cache and signature checks

EOF
)"
```

---

### Task 7: Headers + session/composer helpers

**Files:**
- Create: `internal/adapter/xai/headers.go`
- Create: `internal/adapter/xai/headers_test.go`

**Interfaces:**
- `func ResolveBaseURL(creds *domain.Credentials) string` → default `https://api.x.ai/v1`
- `func ApplyRequestHeaders(req *http.Request, token string, stream bool, sessionID string)`
- `func ResolveSessionID(model string, body []byte, creds *domain.Credentials) string`

Header rules:

- `Authorization: Bearer <token>`
- `Content-Type: application/json`
- `Accept: text/event-stream` when stream, else `application/json`
- if sessionID != "": `x-grok-conv-id: <sessionID>`
- session resolution order:
  1. `creds.ProviderSpecificData["sessionId"]` or `["grokConvId"]`
  2. JSON body `prompt_cache_key` if present
  3. if model has prefix `grok-composer-`, generate stable UUID per connection+model (store in memory map optional); else empty

Also set body `prompt_cache_key` when sessionID present (done in executor prepare step).

- [ ] **Step 1: Failing tests**

```go
package xai

import (
	"net/http/httptest"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestResolveBaseURLDefault(t *testing.T) {
	if got := ResolveBaseURL(&domain.Credentials{}); got != "https://api.x.ai/v1" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyRequestHeadersSession(t *testing.T) {
	req := httptest.NewRequest("POST", "http://example/responses", nil)
	ApplyRequestHeaders(req, "tok", true, "conv-1")
	if req.Header.Get("Authorization") != "Bearer tok" {
		t.Fatal(req.Header.Get("Authorization"))
	}
	if req.Header.Get("Accept") != "text/event-stream" {
		t.Fatal(req.Header.Get("Accept"))
	}
	if req.Header.Get("x-grok-conv-id") != "conv-1" {
		t.Fatal(req.Header.Get("x-grok-conv-id"))
	}
}
```

- [ ] **Step 2–4: Fail → implement → pass**

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/xai/headers.go internal/adapter/xai/headers_test.go
git commit -m "$(cat <<'EOF'
feat(xai): centralize request headers and session id resolution

EOF
)"
```

---

### Task 8: Rewrite executor pipeline (P0 integration)

**Files:**
- Modify: `internal/adapter/xai/executor.go`
- Modify: `internal/adapter/xai/executor_test.go`

**Interfaces:**
- Still implements `port.ProviderExecutor`
- `func NewExecutor() *Executor`
- Optional: `func NewExecutorWithCache(cache *ReplayCache) *Executor` for tests

Pipeline inside `Execute`:

```go
base, effort := ParseModel(model)
translated, err := TranslateChatToResponses(base, body)
translated = ApplyReasoningEffort(translated, effort)
translated = NormalizeTools(translated)
translated = NormalizeToolChoiceForTools(translated)
translated = SanitizeResponsesBody(translated, base)
sessionID := ResolveSessionID(base, translated, credentials)
sessionKey := SessionKeyFrom(credentials, sessionID)
translated, err = ApplyReplay(translated, base, sessionKey, e.cache)
// if sessionID != "", set prompt_cache_key on body
// POST ResolveBaseURL(credentials)+"/responses"
// stream via TranslateResponsesEvent
// on completed event bytes: CacheFromCompleted(...); reqlog.SetUsage(...)
```

- [ ] **Step 1: Expand executor tests with httptest upstream**

Cover:

1. Existing text stream still works
2. Tool-call stream emits OpenAI tool_calls chunks
3. Authorization + `/responses` path
4. Upstream 401/400 returns status + body snippet
5. Reasoning effort present in upstream request when model ends with `-high`
6. `x-grok-conv-id` sent when ProviderSpecificData has session id

Example additional test skeleton:

```go
func TestExecutor_ToolCallStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"sum\",\"arguments\":\"\"}}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.function_call_arguments.delta\",\"output_index\":0,\"delta\":\"{\\\"a\\\":1}\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"sum\",\"arguments\":\"{\\\"a\\\":1}\"}],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"))
	}))
	defer server.Close()

	exec := NewExecutor()
	logger := &testRequestLogger{}
	stream, status, err := exec.Execute("grok-4.3", []byte(`{
		"messages":[{"role":"user","content":"sum"}],
		"tools":[{"type":"function","function":{"name":"sum","parameters":{"type":"object"}}}]
	}`), &domain.Credentials{AccessToken: "t", BaseURL: server.URL}, logger)
	if err != nil || status != 200 {
		t.Fatalf("status=%d err=%v", status, err)
	}
	defer stream.Close()
	b, _ := io.ReadAll(stream)
	if !strings.Contains(string(b), "tool_calls") {
		t.Fatalf("output=%s", b)
	}
}
```

- [ ] **Step 2: Run fail against old executor**

Run: `go test ./internal/adapter/xai -run TestExecutor -count=1`

- [ ] **Step 3: Rewrite executor.go**

Keep streaming via `io.Pipe` + goroutine as today. Use `shared.StreamingHTTPClient`. Buffer scanner 50MB as today.

Ensure old tests still pass (`TestExecutorExecuteStreamsResponsesAsOpenAIChunks`).

- [ ] **Step 4: Run all xai package tests**

Run: `go test ./internal/adapter/xai -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/xai/executor.go internal/adapter/xai/executor_test.go
git commit -m "$(cat <<'EOF'
feat(xai): wire full responses prepare/stream pipeline in executor

EOF
)"
```

---

### Task 9: Compact helpers

**Files:**
- Create: `internal/adapter/xai/compact.go`
- Create: `internal/adapter/xai/compact_test.go`

**Interfaces:**
- `func BuildCompactRequest(body []byte) ([]byte, error)`

Rules (from CLIProxyAPI):

- Ensure JSON object
- Force delete `stream`
- Delete `tools`, `tool_choice`, `parallel_tool_calls` if present
- Remove input items with `type == "compaction_trigger"`
- Keep model/input/instructions/reasoning as provided

- [ ] **Step 1: Failing test**

```go
package xai

import (
	"encoding/json"
	"testing"
)

func TestBuildCompactRequest(t *testing.T) {
	in := []byte(`{
		"model":"grok-4.3",
		"stream":true,
		"tools":[{"type":"web_search"}],
		"input":[
			{"role":"user","content":"hi"},
			{"type":"compaction_trigger"}
		]
	}`)
	out, err := BuildCompactRequest(in)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	_ = json.Unmarshal(out, &payload)
	if _, ok := payload["stream"]; ok {
		t.Fatal("stream should be removed")
	}
	if _, ok := payload["tools"]; ok {
		t.Fatal("tools should be removed")
	}
	input := payload["input"].([]interface{})
	if len(input) != 1 {
		t.Fatalf("input=%v", input)
	}
}
```

- [ ] **Step 2–4: Fail → implement → pass**

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/xai/compact.go internal/adapter/xai/compact_test.go
git commit -m "$(cat <<'EOF'
feat(xai): add responses compact request builder

EOF
)"
```

---

### Task 10: Native `/v1/responses` + `/v1/responses/compact` routes

**Files:**
- Create: `internal/adapter/http/xai-responses-handler.go`
- Create: `internal/adapter/http/xai-responses-handler_test.go`
- Modify: `internal/adapter/http/router.go`
- Modify: `internal/adapter/http/api-handler.go` only if route registration lives there (prefer `router.go` for `/v1`)

**Route behavior:**

```go
v1.POST("/responses", xaiResponsesHandler(store, providers))
v1.POST("/responses/compact", xaiResponsesCompactHandler(store, providers))
```

Shared resolve logic should mirror chat/image handlers:

1. Parse JSON body, read `model`
2. Resolve provider/model via existing model resolver / connection selection patterns used by `image-handler` or chat service
3. Build credentials from selected connection
4. For `/responses`:
   - if model is xAI (prefix `xai/` or `grok/` or provider connection xai), prepare with sanitize/tools/thinking/replay
   - if `stream` true: proxy SSE (optionally pass-through upstream events; do not force OpenAI chat translation)
   - if `stream` false: collect upstream non-stream/SSE-completed payload and return JSON
5. For `/compact`: `BuildCompactRequest` then POST `{base}/responses/compact`, return JSON

**Important:** Do not invent a second account selector. Reuse the same approach as `imageGenerationsHandler` / chat service public helpers if available. If chat selection is hard to reuse, extract a small internal helper in the handler file:

```go
func resolveXAIExecution(store port.CredentialStore, providers port.ProviderRegistry, model string) (canonicalModel string, creds *domain.Credentials, status int, err error)
```

Prefer calling into existing service methods if they already expose selection; otherwise keep handler-local selection consistent with image handler.

- [ ] **Step 1: Write handler tests with gin + httptest upstream + fake store**

Minimal cases:

1. missing model → 400
2. unknown connection/model → 404/400 matching project conventions
3. compact deletes tools and hits `/responses/compact`
4. stream responses proxies data lines

If full store mocking is heavy, unit-test pure prepare helpers and keep one integration-style handler test.

- [ ] **Step 2: Run fail**

Run: `go test ./internal/adapter/http -run XAIResponses -count=1`

- [ ] **Step 3: Implement handlers + register routes in `router.go` near other `/v1` routes**

```go
v1.POST("/chat/completions", chatHandler(...))
v1.POST("/messages", messagesHandler(...))
v1.POST("/responses", xaiResponsesHandler(store, providers))
v1.POST("/responses/compact", xaiResponsesCompactHandler(store, providers))
v1.POST("/images/generations", imageGenerationsHandler(...))
```

- [ ] **Step 4: Run package tests + compile**

Run:

```bash
go test ./internal/adapter/http -count=1
go test ./internal/adapter/xai -count=1
go build -o dntproxy ./cmd/dntproxy/
```

Expected: PASS / build OK

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/http/xai-responses-handler.go internal/adapter/http/xai-responses-handler_test.go internal/adapter/http/router.go
git commit -m "$(cat <<'EOF'
feat(http): add xAI /v1/responses and /responses/compact endpoints

EOF
)"
```

---

### Task 11: Unifyментация image path consistency (headers/base URL)

**Files:**
- Modify: `internal/adapter/http/image-handler.go`
- Modify: `internal/adapter/openai/xai-image-translator.go` (optional thin wrappers)

Goal: image requests use the same auth header helper and base URL resolver as chat/responses.

- [ ] **Step 1: Add test if one exists for image xAI path; otherwise add focused helper test in xai package (already done) and adjust call sites**

In `executeXAIImageGeneration` / edit:

```go
baseURL := xai.ResolveBaseURL(creds)
// ...
xai.ApplyRequestHeaders(req, token, false, "")
```

Token selection: `creds.AccessToken` if set else `creds.APIKey` (if project already does this elsewhere, match it).

- [ ] **Step 2: Run**

```bash
go test ./internal/adapter/http -count=1
go test ./internal/adapter/openai -count=1
go test ./internal/adapter/xai -count=1
```

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/http/image-handler.go internal/adapter/openai/xai-image-translator.go
git commit -m "$(cat <<'EOF'
refactor(xai): reuse shared headers/base URL for image requests

EOF
)"
```

---

### Task 12: Model metadata polish + regression sweep

**Files:**
- Modify: `internal/domain/model-definition.go` (only if metadata flags useful)
- Modify: `docs/project-changelog.md` (short entry)
- Optional README API blurb if project documents endpoints there

- [ ] **Step 1: Ensure thinking-capable models are listed and defaults remain sensible**

At minimum keep:

- `xai/grok-4.3`
- `xai/grok-4.20-0309-reasoning`
- `xai/grok-4.20-0309-non-reasoning`
- `xai/grok-3-mini`
- image models unchanged

If `ModelDefinition.Metadata` already used for capabilities, set `"supportsReasoningEffort": true` on mini/reasoning models. If unused, skip domain changes.

- [ ] **Step 2: Full regression commands**

```bash
go test ./internal/adapter/xai -count=1
go test ./internal/adapter/http -count=1
go test ./internal/adapter/auth -count=1
go test ./internal/service -count=1
go build -o dntproxy ./cmd/dntproxy/
```

Expected: all PASS, build OK

- [ ] **Step 3: Manual smoke (local, with a real or mocked account)**

```bash
# chat text
curl -s localhost:20199/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"xai/grok-4.3","stream":true,"messages":[{"role":"user","content":"hi"}]}'

# chat tools (model must support tools)
curl -s localhost:20199/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model":"xai/grok-4.3",
    "stream":true,
    "messages":[{"role":"user","content":"Use sum on 1 and 2"}],
    "tools":[{"type":"function","function":{"name":"sum","parameters":{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}}}}}]
  }'

# native responses compact
curl -s localhost:20199/v1/responses/compact \
  -H "Content-Type: application/json" \
  -d '{"model":"xai/grok-4.3","input":[{"role":"user","content":"long context..."}]}'
```

- [ ] **Step 4: Commit docs**

```bash
git add docs/project-changelog.md README.md internal/domain/model-definition.go
git commit -m "$(cat <<'EOF'
docs: note xAI responses/tool/reasoning parity upgrades

EOF
)"
```

---

## Acceptance Criteria (Mốc B)

### Must pass

- [ ] Existing OAuth login/import/refresh still works
- [ ] `POST /v1/chat/completions` streams text for `xai/grok-*`
- [ ] Function calling multi-turn works (assistant tool_calls → tool result → continue)
- [ ] Built-in tool types are not hard-rejected (`web_search` at least passes normalize)
- [ ] `model-high` / thinking suffix maps to `reasoning.effort` when supported
- [ ] Reasoning replay cache injects encrypted content on subsequent turns when session key present
- [ ] `POST /v1/responses` works for xAI models
- [ ] `POST /v1/responses/compact` works
- [ ] Image generation still works
- [ ] Usage logged on completed responses
- [ ] Unit tests for new packages green; project builds

### Explicitly deferred (not failure)

- WebSocket Responses transport
- Video generations/edits/extensions
- Claude/Gemini protocol translation into xAI
- Remote model registry updater
- Persistent replay cache across process restarts

## Test Plan Summary

| Area | Command |
|---|---|
| Model/tools/translators | `go test ./internal/adapter/xai -count=1` |
| HTTP routes | `go test ./internal/adapter/http -count=1` |
| Auth refresh untouched | `go test ./internal/adapter/auth -count=1` |
| Chat service still routes | `go test ./internal/service -count=1` |
| Compile | `go build -o dntproxy ./cmd/dntproxy/` |

## Implementation Order Rationale

1. Pure helpers first (model/tools/translators/sanitize/cache) → easy TDD, low risk
2. Executor integration next → user-visible chat parity
3. Compact + native responses routes → Build API surface
4. Image header reuse + docs → cleanup

## Risk Notes

- Upstream event names can drift; keep translators tolerant (ignore unknown events).
- Tool call id field naming (`id` vs `call_id`) is a common break point — test both emission and consumption.
- Replay cache is best-effort; never fail the request if cache read/write errors (log and continue), except when intentionally strict in tests.
- Do not set network timeouts on upstream streaming body reads (project streaming client already configured).
- Avoid importing anything from `temp/CLIProxyAPI` into go.mod.

## Self-Review Checklist

1. **Spec coverage:** Auth hardening minor items covered implicitly by existing singleflight refresh; P0 tools/reasoning/session/responses/compact/image consistency all have tasks. WS/video excluded on purpose.
2. **No placeholders:** Tasks include concrete files, APIs, tests, commands, commit messages.
3. **Type consistency:** Public function names (`TranslateChatToResponses`, `NormalizeTools`, `ApplyReplay`, `BuildCompactRequest`, `ResolveBaseURL`, `ApplyRequestHeaders`) are stable across tasks.

## Follow-up Plan (not this PR series)

Create later: `docs/superpowers/plans/YYYY-MM-DD-xai-grok-websocket-video.md` covering:

- WS session store + previous_response_id mapper
- video routes parity
- optional persistent replay cache in SQLite

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-09-xai-grok-build-parity.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration  
2. **Inline Execution** — execute tasks in this session with executing-plans and checkpoints  

Which approach?
