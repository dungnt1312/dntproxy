---
title: "Refactor: Provider Operations Interface (Option C - Hybrid)"
status: pending
created: 2026-04-13
updated: 2026-04-13
labels: [refactor, architecture, http-handlers, provider-operations]
---

# Plan: Refactor Provider Operations Interface (Option C - Hybrid)

## Overview

Refactor `connection-handler.go` (913 dòng) và `auth-handler.go` (1000 dòng) thành cấu trúc **hybrid**: tạo interface `ProviderOperations` cho các operation khác biệt theo provider (test, quota, fetch-models), đồng thời split auth handlers theo từng provider. Mục tiêu: mỗi file < 200 dòng, dễ test unit, dễ thêm provider mới.

### Scope
- **IN**: `internal/adapter/http/`, `internal/port/`, `internal/adapter/{kiro,openai,glm,minimax,qwen,anthropic,gemini}/`
- **OUT**: UI components, CLI commands, core chat service, provider executor (đã có interface riêng)

### Success Criteria
1. Mỗi file trong `internal/adapter/http/` < 200 dòng
2. Interface `ProviderOperations` có đúng 3 methods (không optional)
3. Thêm provider mới chỉ cần: tạo `provider_ops.go` (1 file) + register trong `main.go` (+1 dòng)
4. Toàn bộ tests hiện tại pass
5. API backward compatible — UI/CLI không đổi

---

## Architecture Decisions

### Decision 1: Interface `ProviderOperations` (port layer)

```go
// internal/port/provider-ops.go
type ProviderOperations interface {
    ID() string
    TestConnection(conn *domain.ProviderConnection, baseURL string) (*TestResult, error)
    CheckQuota(conn *domain.ProviderConnection) (*QuotaResult, error)
    FetchModels(conn *domain.ProviderConnection) ([]string, error)
}
```

**Lý do**: Chỉ 3 methods — đủ cho HTTP layer, không optional, đúng Go idiom. `ProviderExecutor` (đã có) lo phần streaming chat request — tách biệt rõ ràng.

### Decision 2: Adapter per provider implement `ProviderOperations`

```
internal/adapter/kiro/provider_ops.go        — KiroOps
internal/adapter/openai/provider_ops.go      — OpenAIOps
internal/adapter/glm/provider_ops.go         — GLMOps
internal/adapter/minimax/provider_ops.go     — MiniMaxOps
internal/adapter/qwen/provider_ops.go        — QwenOps
internal/adapter/anthropic/provider_ops.go   — AnthropicOps
internal/adapter/gemini/provider_ops.go      — GeminiOps
```

**Lý do**: Logic test/quota/models khác nhau hoàn toàn giữa providers → mỗi provider tự implement, không duplicate code trong handler chung.

### Decision 3: Auth handlers tách per-provider (không qua interface)

```
internal/adapter/http/auth-kiro-handler.go
internal/adapter/http/auth-openai-handler.go
internal/adapter/http/auth-qwen-handler.go
internal/adapter/http/auth-session.go         — Session structs + cleanup (shared)
```

**Lý do**: OAuth flows quá khác nhau (device code vs PKCE vs social callback) — interface sẽ có quá nhiều optional methods. Thay vào đó, `RegisterAuthRoutes` gọi trực tiếp handlers theo tên.

### Decision 4: Handler files split theo chức năng

```
internal/adapter/http/
├── api-handler.go              — Route wiring only (~100 dòng, giữ nguyên)
├── connection-list-handler.go  — apiListConnections (~100 dòng)
├── connection-crud-handler.go  — import, delete, update, reset-cooldown (~200 dòng)
├── connection-add-handler.go   — Generic add + per-provider wrappers (~180 dòng)
├── connection-test-handler.go  — apiTestConnection, apiTestModel (~150 dòng, delegate ProviderOperations)
├── connection-quota-handler.go — apiCheckQuota (~100 dòng, delegate ProviderOperations)
├── connection-detect-handler.go — apiDetectKiroToken (~100 dòng)
├── auth-handler.go             — Auth router only (~80 dòng)
├── auth-kiro-handler.go        — Kiro: Builder ID, IDC, Social, Poll (~250 dòng)
├── auth-openai-handler.go      — OpenAI: Start + Exchange (~200 dòng)
├── auth-qwen-handler.go        — Qwen: Start + Poll (~120 dòng)
└── auth-session.go             — Session structs + cleanup (~80 dòng)
```

### Decision 5: Usage handler giữ nguyên

`usage-handler.go` đã có logic phức tạp cho Kiro/OpenAI OAuth. Sẽ refactor sau — scope plan này chỉ tập trung vào connection + auth handlers.

---

## Implementation Phases

### Phase 1: Create `ProviderOperations` Interface

**Mục tiêu**: Định nghĩa contract, chưa thay đổi code cũ.

1. Tạo `internal/port/provider-ops.go`
2. Define interface `ProviderOperations` với 3 methods
3. Define struct `TestResult` (currently inline in connection-handler.go)
4. Compile check — chưa dùng ở đâu

**File changes**:
- **NEW**: `internal/port/provider-ops.go` (~40 dòng)

**Deliverable**: Interface compile OK, không breaking changes.

---

### Phase 2: Implement `ProviderOperations` cho Kiro

**Mục tiêu**: Extract logic từ `connection-handler.go` + `quota-handler.go` → `KiroOps`.

1. Tạo `internal/adapter/kiro/provider_ops.go`
2. Extract `TestConnection()` từ hàm `testProviderAPI()` (phần Kiro-specific)
3. Extract `CheckQuota()` từ `handleKiroQuota()` trong `quota-handler.go`
4. `FetchModels()` → hiện tại Kiro không có model list API, return default models từ config
5. Compile check

**File changes**:
- **NEW**: `internal/adapter/kiro/provider_ops.go` (~120 dòng)
- **MODIFY**: `internal/adapter/kiro/` — kiểm tra xem có file nào cần reuse HTTP client không

**Dependencies**: Phase 1 xong

**Deliverable**: `KiroOps` implement đầy đủ, có thể test unit độc lập.

---

### Phase 3: Implement `ProviderOperations` cho OpenAI

**Mục tiêu**: Extract logic từ `quota-handler.go` + `connection-handler.go` → `OpenAIOps`.

1. Tạo `internal/adapter/openai/provider_ops.go`
2. `TestConnection()` → test qua `/v1/models` hoặc Codex probe cho OAuth
3. `CheckQuota()` → từ `handleOpenAIOAuthQuota()` + `handleOpenAIAPIKeyQuota()` + `checkCodexQuota()`
4. `FetchModels()` → từ `/v1/models` endpoint
5. Compile check

**File changes**:
- **NEW**: `internal/adapter/openai/provider_ops.go` (~150 dòng)
- **MODIFY**: `internal/adapter/openai/executor.go` — kiểm tra có thể reuse HTTP client

**Dependencies**: Phase 1 xong

**Deliverable**: `OpenAIOps` implement đầy đủ.

---

### Phase 4: Implement `ProviderOperations` cho providers đơn giản (GLM, MiniMax, Qwen, Anthropic, Gemini)

**Mục tiêu**: Các provider chỉ có API key — implement nhanh.

1. Tạo 5 files `provider_ops.go` cho từng provider
2. `TestConnection()` → gửi request test đến provider endpoint (giống `testProviderAPI()` generic)
3. `CheckQuota()` → return `nil, nil` (không hỗ trợ)
4. `FetchModels()` → return default models từ config (giống `NoOpModelFetcher`)
5. Có thể tạo base struct `DefaultOps` trong `internal/adapter/shared/` để reuse logic chung

**Option 4a** (Recommended): Tạo `shared/default-provider-ops.go` làm base, mỗi provider embed + override nếu cần:

```go
// internal/adapter/shared/default-provider-ops.go
type DefaultOps struct {
    ProviderID string
    HTTPClient *http.Client
}

func (d *DefaultOps) ID() string { return d.ProviderID }

func (d *DefaultOps) TestConnection(conn *domain.ProviderConnection, baseURL string) (*port.TestResult, error) {
    // Generic test: POST /v1/chat/completions với max_tokens=1
    // ... (extract từ testProviderAPI() generic path)
}

func (d *DefaultOps) CheckQuota(conn *domain.ProviderConnection) (*port.QuotaResult, error) {
    return nil, nil // Not supported
}

func (d *DefaultOps) FetchModels(conn *domain.ProviderConnection) ([]string, error) {
    cfg := domain.GetProviderConfig(conn.Provider)
    return cfg.DefaultModels, nil
}
```

Sau đó mỗi provider chỉ cần:
```go
// internal/adapter/glm/provider_ops.go
type GLMOps struct{ *shared.DefaultOps }
func NewGLMOps(client *http.Client) *GLMOps {
    return &GLMOps{&shared.DefaultOps{ProviderID: "glm", HTTPClient: client}}
}
```

**File changes**:
- **NEW**: `internal/adapter/shared/default-provider-ops.go` (~80 dòng)
- **NEW**: `internal/adapter/glm/provider_ops.go` (~20 dòng)
- **NEW**: `internal/adapter/minimax/provider_ops.go` (~20 dòng)
- **NEW**: `internal/adapter/qwen/provider_ops.go` (~20 dòng)
- **NEW**: `internal/adapter/anthropic/provider_ops.go` (~20 dòng)
- **NEW**: `internal/adapter/gemini/provider_ops.go` (~20 dòng)

**Dependencies**: Phase 1 xong (không cần chờ Phase 2-3)

**Deliverable**: Tất cả providers có `ProviderOperations` implementation.

---

### Phase 5: Split Auth Handlers

**Mục tiêu**: Tách `auth-handler.go` (1000 dòng) thành 4 files.

1. Tạo `internal/adapter/http/auth-session.go` — move session structs + cleanup functions
2. Tạo `internal/adapter/http/auth-kiro-handler.go` — Builder ID, IDC, Social login, poll
3. Tạo `internal/adapter/http/auth-openai-handler.go` — OpenAI Start + Exchange
4. Tạo `internal/adapter/http/auth-qwen-handler.go` — Qwen Start + Poll
5. `auth-handler.go` còn lại ~80 dòng — chỉ `RegisterAuthRoutes()`

**File changes**:
- **NEW**: `internal/adapter/http/auth-session.go` (~80 dòng)
- **NEW**: `internal/adapter/http/auth-kiro-handler.go` (~250 dòng)
- **NEW**: `internal/adapter/http/auth-openai-handler.go` (~200 dòng)
- **NEW**: `internal/adapter/http/auth-qwen-handler.go` (~120 dòng)
- **MODIFY**: `internal/adapter/http/auth-handler.go` → ~80 dòng (chỉ route wiring)

**Dependencies**: Không phụ thuộc phases trước

**Deliverable**: `auth-handler.go` từ 1000 → 80 dòng.

---

### Phase 6: Split Connection Handlers

**Mục tiêu**: Tách `connection-handler.go` (913 dòng) thành 6 files.

1. Tạo `internal/adapter/http/connection-list-handler.go` — `apiListConnections` (~100 dòng)
2. Tạo `internal/adapter/http/connection-crud-handler.go` — import, delete, update, reset-cooldown (~200 dòng)
3. Tạo `internal/adapter/http/connection-add-handler.go` — generic add + per-provider wrappers (~180 dòng)
4. Tạo `internal/adapter/http/connection-test-handler.go` — `apiTestConnection`, `apiTestModel`, delegate `ProviderOperations` (~150 dòng)
5. Tạo `internal/adapter/http/connection-quota-handler.go` — `apiCheckQuota`, delegate `ProviderOperations` (~100 dòng)
6. Tạo `internal/adapter/http/connection-detect-handler.go` — `apiDetectKiroToken` (~100 dòng)
7. Xóa `connection-handler.go` cũ

**File changes**:
- **NEW**: `internal/adapter/http/connection-list-handler.go` (~100 dòng)
- **NEW**: `internal/adapter/http/connection-crud-handler.go` (~200 dòng)
- **NEW**: `internal/adapter/http/connection-add-handler.go` (~180 dòng)
- **NEW**: `internal/adapter/http/connection-test-handler.go` (~150 dòng)
- **NEW**: `internal/adapter/http/connection-quota-handler.go` (~100 dòng)
- **NEW**: `internal/adapter/http/connection-detect-handler.go` (~100 dòng)
- **DELETE**: `internal/adapter/http/connection-handler.go`

**Dependencies**: Phase 1-4 xong (để test/quota handlers dùng `ProviderOperations`). Phase 5 có thể làm song song.

**Deliverable**: `connection-handler.go` từ 913 → 0 dòng (deleted), split thành 6 files < 200 dòng.

---

### Phase 7: Register `ProviderOperations` trong `main.go`

**Mục tiêu**: Wire `ProviderOperations` implementations vào router.

1. Sửa `NewRouter()` signature để nhận `ops map[string]port.ProviderOperations`
2. Trong `main.go`, tạo map và truyền vào:

```go
ops := map[string]port.ProviderOperations{
    "kiro":      kiro.NewKiroOps(sharedClient),
    "openai":    openai.NewOpenAIOps(sharedClient),
    "glm":       glm.NewGLMOps(sharedClient),
    "minimax":   minimax.NewMiniMaxOps(sharedClient),
    "qwen":      qwen.NewQwenOps(sharedClient),
    "anthropic": anthropic.NewAnthropicOps(sharedClient),
    "gemini":    gemini.NewGeminiOps(sharedClient),
}
r := http.NewRouter(store, providers, tunnelMgr, ops)
```

3. Sửa `api-handler.go` để truyền `ops` vào các handler cần thiết

**File changes**:
- **MODIFY**: `internal/adapter/http/router.go` — thêm param `ops`
- **MODIFY**: `internal/adapter/http/api-handler.go` — truyền `ops` vào test/quota handlers
- **MODIFY**: `cmd/dntproxy/main.go` — tạo ops map, truyền vào `NewRouter()`

**Dependencies**: Tất cả phases trước xong

**Deliverable**: Toàn bộ wiring compile OK.

---

### Phase 8: Cleanup & Deprecate NoOp Adapters

**Mục tiêu**: Xóa `NoOpModelFetcher` và `NoOpQuotaChecker` — đã thay thế bằng `ProviderOperations`.

1. Tìm nơi register `NoOpModelFetcher` và `NoOpQuotaChecker` trong `main.go`
2. Xóa registration — thay bằng `ProviderOperations` map
3. Xóa files `internal/adapter/custom/model-fetcher.go` và `internal/adapter/custom/quota-checker.go`
4. Compile + test

**File changes**:
- **DELETE**: `internal/adapter/custom/model-fetcher.go`
- **DELETE**: `internal/adapter/custom/quota-checker.go`
- **MODIFY**: `cmd/dntproxy/main.go` — xóa NoOp registration

**Dependencies**: Phase 7 xong

**Deliverable**: Không còn dead code, architecture sạch sẽ.

---

### Phase 9: Test & Verification

**Mục tiêu**: Đảm bảo không regression.

1. `go build ./...` — compile OK
2. `go test ./...` — toàn bộ tests pass
3. Manual test:
   - GET /api/connections — list OK
   - POST /api/connections/:id/test — test Kiro, OpenAI, GLM OK
   - POST /api/connections/:id/check-quota — quota Kiro, OpenAI OK
   - Auth flows: Kiro Builder ID, OpenAI OAuth — hoàn thành flow OK
4. Verify file sizes: `wc -l internal/adapter/http/*.go` — tất cả < 200 dòng

**Deliverable**: Green CI, file sizes达标.

---

## File Changes Summary

| Phase | Action | Files | Lines (approx) |
|-------|--------|-------|----------------|
| 1 | NEW | `internal/port/provider-ops.go` | +40 |
| 2 | NEW | `internal/adapter/kiro/provider_ops.go` | +120 |
| 3 | NEW | `internal/adapter/openai/provider_ops.go` | +150 |
| 4 | NEW | `internal/adapter/shared/default-provider-ops.go` | +80 |
| 4 | NEW | `internal/adapter/{glm,minimax,qwen,anthropic,gemini}/provider_ops.go` | +100 (5×20) |
| 5 | NEW | `internal/adapter/http/auth-session.go` | +80 |
| 5 | NEW | `internal/adapter/http/auth-kiro-handler.go` | +250 |
| 5 | NEW | `internal/adapter/http/auth-openai-handler.go` | +200 |
| 5 | NEW | `internal/adapter/http/auth-qwen-handler.go` | +120 |
| 5 | MODIFY | `internal/adapter/http/auth-handler.go` | 1000 → 80 |
| 6 | NEW | `internal/adapter/http/connection-*.go` (6 files) | +830 |
| 6 | DELETE | `internal/adapter/http/connection-handler.go` | -913 |
| 7 | MODIFY | `internal/adapter/http/router.go`, `api-handler.go` | +20 |
| 7 | MODIFY | `cmd/dntproxy/main.go` | +15 |
| 8 | DELETE | `internal/adapter/custom/model-fetcher.go`, `quota-checker.go` | -50 |

**Net change**: ~0 lines total (refactor), nhưng +14 files mới, -3 files cũ, 4 files sửa.

---

## Testing Strategy

### Unit Tests
1. **`ProviderOperations` mocks**: Tạo mock implementation cho mỗi provider, test handlers độc lập
2. **Test `DefaultOps`**: Test generic `TestConnection()` với HTTP mock server
3. **Test `KiroOps.CheckQuota()`**: Mock Kiro API response, verify parsing đúng

### Integration Tests
1. Test full connection lifecycle: add → test → update → delete
2. Test auth flows: Kiro Builder ID device code flow, OpenAI PKCE flow
3. Test quota check: Kiro quota API, OpenAI rate-limit headers

### Regression Tests
1. Chạy existing tests: `go test ./internal/service/...` (chat-service tests)
2. Manual test với UI: connections page load OK, test buttons hoạt động
3. Verify API responses không thay đổi (backward compatible)

---

## Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| **`TestConnection()` signature khác `testProviderAPI()`** | Trung bình | Thấp | Giữ `TestResult` struct tương thích, chỉ extract logic, không thay đổi behavior |
| **Auth session state (in-memory map) bị break khi split** | Cao | Trung bình | Giữ `auth-session.go` trong cùng package `http`, không move sang package khác |
| **`usage-handler.go` vẫn còn duplicate logic quota** | Thấp | Cao | Scope phase sau — hiện tại không ảnh hưởng correctness |
| **`provider-config.go` thiếu Gemini config** | Thấp | Thấp | Kiểm tra trước khi implement Phase 4 |
| **Race condition khi truy cập `ProviderOperations` map** | Trung bình | Thấp | Dùng `sync.RWMutex` hoặc pass map ở startup (immutable sau đó) |
| **Breaking changes cho UI nếu response format đổi** | Cao | Thấp | Giữ nguyên JSON response shape — chỉ refactor internal dispatch |
| **Circular dependency: adapter/http → adapter/kiro → adapter/shared** | Trung bình | Thấp | Review import graph trước khi merge mỗi phase |

---

## Execution Order

```
Phase 1 (interface)
    ├── Phase 2 (KiroOps)
    ├── Phase 3 (OpenAIOps)
    └── Phase 4 (DefaultOps + simple providers) — có thể làm song song với 2-3

Phase 5 (auth split) — có thể làm song song với 2-4

Phase 6 (connection split) — sau Phase 1 + Phase 5

Phase 7 (wire up) — sau Phase 2-6

Phase 8 (cleanup) — sau Phase 7

Phase 9 (test) — sau Phase 8
```

**Estimated parallel execution**: Nếu làm 2 phases song song (2-3 và 5), tổng thời gian implement ~7 steps sequential.
