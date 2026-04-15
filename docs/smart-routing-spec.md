# Smart Routing Specification — DNTProxy

**Version:** 2.1
**Date:** 2026-04-15
**Status:** Draft
**Changelog:** v2.1 — Fix lifecycle ordering, backoff math, token counting, streaming failure, discovery probes, model normalization, shutdown spec

---

## 1. Overview

### 1.1 Mục tiêu

Thiết kế routing system cho phép client gửi request với **bất kỳ format model name nào** — có hoặc không có provider prefix — và system tự động:

1. Nhận diện model thuộc provider nào
2. Tìm connection đang available
3. Route request đến đúng connection
4. Fallback tự động nếu fail
5. Deduct credits, log, rate limit

### 1.2 Model Name Formats được hỗ trợ

| Format | Ví dụ | Behavior |
|--------|-------|----------|
| Có prefix | `kr/claude-opus-4.6` | Parse prefix → resolve provider alias → route |
| Có prefix (full) | `kiro/claude-opus-4.6` | Parse prefix → route trực tiếp |
| Không prefix | `claude-opus-4.6` | Smart resolve qua 3-tier registry |
| Alias | `sonnet` | Lookup alias map → resolve target model |
| Combo name | `fast-fallback` | Expand thành model list → route từng cái |
| Wildcard trong key | `claude-*` | Pattern match tất cả models phù hợp |

### 1.3 Actors

| Actor | Vai trò |
|-------|---------|
| **Admin** | Quản lý connections, API keys, combos, pricing qua dashboard |
| **End-user** | Gọi API với API key, chỉ biết model name, không biết backend routing |
| **System** | Tự động resolve, route, fallback, rate limit, deduct credits |

---

## 2. Request Lifecycle

### 2.1 High-Level Flow

```
Client Request
  → 1. Validate API Key
  → 2. Normalize Model Name
  → 3. Check Rate Limit (RPM)
  → 4. Smart Model Resolver
  → 5. Check Permissions (model + provider)
  → 6. Pre-flight Credit Check
  → 7. Connection Discovery
  → 8. Execution Loop (bounded retry)
  → 9. Post-Execution (async)
  → Return SSE Stream
```

### 2.2 Timing Target

| Step | Latency Target | Storage |
|------|---------------|---------|
| Validate API Key | <0.1ms | In-memory hash map |
| Normalize Model Name | <0.01ms | In-memory string ops |
| Rate Limit Check (RPM) | <0.1ms | In-memory token bucket |
| Smart Resolver (cache hit) | <0.1ms | In-memory LRU cache |
| Smart Resolver (static) | <0.1ms | In-memory registry |
| Smart Resolver (discovery) | 200-800ms | Parallel HTTP probe (first time only) |
| Check Permissions | <0.1ms | In-memory array scan |
| Credit Pre-flight | <0.1ms | In-memory float compare |
| Connection Discovery | <0.5ms | In-memory filter + sort |
| Execute on Provider | Varies | Upstream latency |
| Credit Deduct | ~0ms | Async, non-blocking |
| Log | ~0ms | Async batch, non-blocking |

**Total proxy overhead target:** <5ms p95 (excluding upstream latency)

---

## 3. Step 1: Validate API Key

### 3.1 Mục đích

Xác thực API key hợp lệ và đang active.

### 3.2 Input

```
Authorization: Bearer pk-abc123def456
```

### 3.3 Logic

1. Extract API key từ `Authorization: Bearer <key>` header
2. Hash key bằng HMAC-SHA256 với fixed server secret (fast, constant-time; bcrypt ~100ms — incompatible với <0.1ms latency target)
3. Lookup hash trong in-memory key map
4. Kiểm tra `IsActive == true`
5. Kiểm tra `ExpiresAt` (nếu có) chưa quá hạn

### 3.4 Output

| Kết quả | HTTP Status | Response |
|---------|-------------|----------|
| Key hợp lệ | — | Tiếp tục bước 2 |
| Key không tồn tại | `401` | `{ "error": { "message": "Invalid API key", "type": "auth_error" } }` |
| Key bị disabled | `401` | `{ "error": { "message": "API key disabled", "type": "auth_error" } }` |
| Key hết hạn | `401` | `{ "error": { "message": "API key expired", "type": "auth_error" } }` |

### 3.5 Data Structures

**APIKey:**

| Field | Type | Ý nghĩa |
|-------|------|---------|
| `id` | string | Unique ID (`pk-xxx`) |
| `name` | string | Tên display cho admin |
| `key_hash` | string | Hash của actual key |
| `allowed_models` | []string | Whitelist models hoặc `["*"]` |
| `allowed_providers` | []string | Whitelist providers hoặc `["*"]` |
| `rpm` | int | Requests per minute (0 = unlimited) |
| `tpm` | int | Tokens per minute (0 = unlimited) |
| `credit_balance` | float64 | Số dư credit hiện tại (USD) |
| `credit_limit` | float64 | Giới hạn credit (0 = unlimited) |
| `expires_at` | *time.Time | Ngày hết hạn (nil = không hết hạn) |
| `is_active` | bool | Trạng thái active |
| `tags` | []string | Tag phân loại |
| `notes` | string | Ghi chú admin |
| `created_at` | time.Time | Ngày tạo |
| `updated_at` | time.Time | Ngày cập nhật |

**In-memory Index:**

```
keysByHash map[string]*APIKey
// key_hash → APIKey pointer
// O(1) lookup
```

---

## 4. Step 2: Normalize Model Name

### 4.1 Mục đích

Chuẩn hóa model name trước khi resolve, tránh mismatch do case, whitespace, hoặc ký tự không nhất quán.

### 4.2 Logic

```
normalized = model_string
normalized = strings.TrimSpace(normalized)        // Remove leading/trailing whitespace
normalized = strings.ToLower(normalized)           // Case-insensitive: "Claude-Opus-4.6" → "claude-opus-4.6"
// Note: provider prefix cũng được normalize: "KR/Claude" → "kr/claude"
```

**Không normalize:**
- Không replace underscores → hyphens (vì có models dùng underscore: `qwen3_coder`)
- Không collapse multiple slashes

### 4.3 Output

| Input | Normalized |
|-------|------------|
| `Claude-Opus-4.6` | `claude-opus-4.6` |
| ` gpt-4o ` | `gpt-4o` |
| `KR/Claude-Sonnet-4.5` | `kr/claude-sonnet-4.5` |
| `SONNET` | `sonnet` |

→ Tiếp tục bước 3 với normalized model name.

---

## 5. Step 3: Check Rate Limit (RPM)

### 5.1 Mục đích

Giới hạn số requests/tokens mỗi API key có thể gửi trong 1 phút.

### 5.2 Algorithm: Token Bucket

**Lý do chọn Token Bucket:**
- Cho phép burst requests (không như fixed window)
- Smooth rate limiting
- In-memory, không cần DB
- Dễ implement

**Parameters:**

| Parameter | Ý nghĩa | Default |
|-----------|---------|---------|
| `rpm` | Requests per minute | 0 (unlimited) |
| `tpm` | Tokens per minute | 0 (unlimited) |

### 5.3 Logic

#### 5.3.1 RPM Check

```
bucket = getOrCreateBucket(key_id + ":rpm")
bucket.capacity = rpm
bucket.refill_rate = rpm / 60.0  // tokens per second

// Trước mỗi request:
elapsed = now - last_refill
tokens = min(capacity, tokens + elapsed * refill_rate)
last_refill = now

IF tokens >= 1:
  tokens -= 1
  ALLOW
ELSE:
  DENY
```

#### 5.3.2 TPM Check (Post-Flight Only)

TPM enforcement chỉ thực hiện **post-flight** (sau khi nhận response từ provider, có actual token count).

**Lý do không pre-flight TPM:**
- Token counting khác nhau giữa providers (tiktoken cho OpenAI, claude tokenizer cho Anthropic)
- Proxy không bundle tokenizer libraries → character estimation quá inaccurate
- Messages chứa images (vision) thì pixel-based token count, phức tạp

**Lưu ý — Eventually Consistent:** Post-flight TPM là **soft enforcement** — concurrent requests đều pass check trước khi deduction xảy ra. Under high concurrency, burst consumption có thể vượt TPM limit trong 1 window. Acceptable cho current load levels; nếu cần strict enforcement sau này thì cần pre-flight estimate (YAGNI).

```
// Post-flight (async, trong Step 9):
actual_tokens = response.usage.total_tokens
bucket = getOrCreateBucket(key_id + ":tpm")
bucket.consume(actual_tokens)
// Nếu bucket.tokens < 0 → next request sẽ bị reject
```

### 5.4 Data Structures

```
buckets sync.Map
// key: "{key_id}:rpm" hoặc "{key_id}:tpm"
// value: *TokenBucket

TokenBucket {
  mu          sync.Mutex
  tokens      float64    // available tokens
  capacity    float64    // max tokens
  refillRate  float64    // tokens/second
  lastRefill  time.Time  // last refill time
}
```

### 5.5 Output

| Kết quả | HTTP Status | Response |
|---------|-------------|----------|
| Within RPM limit | — | Tiếp tục bước 4 |
| RPM exceeded | `429` | `{ "error": { "message": "Rate limit exceeded: RPM", "type": "rate_limit_error", "retry_after": 5 } }` |
| TPM exceeded (post-flight) | `429` | `{ "error": { "message": "Rate limit exceeded: TPM", "type": "rate_limit_error", "retry_after": 30 } }` |

### 5.6 Edge Cases

| Case | Behavior |
|------|----------|
| Key bị delete giữa chừng | Bucket vẫn tồn tại, nhưng key lookup fail ở Step 1 |
| Process restart | Tất cả buckets reset → brief unlimited window (acceptable) |
| `rpm=0, tpm=0` | Skip check entirely, unlimited |
| Concurrent requests | Mutex per bucket, serialized access |
| Stale buckets | Không eviction needed, memory footprint nhỏ |

---

## 6. Step 6: Pre-flight Credit Check

> **Note:** Document sections are ordered for readability, not strictly by lifecycle step number.
> In the lifecycle: Step 4 (Resolve) → Step 5 (Permissions) → **Step 6 (Credit Check)** → Step 7 (Connection Discovery).
> Credit Check is documented before Resolver because it's a self-contained module.

### 6.1 Mục đích

Kiểm tra API key có đủ credit để xử lý request trước khi forward tới provider.

### 6.2 Logic

#### 6.2.1 Cost Estimation

```
// Character-based estimation (no tokenizer dependency):
estimated_input_tokens = len(serialize(request.messages)) / 4  // ~4 chars per token
estimated_output_tokens = min(request.max_tokens OR 4096, 4096)  // cap at 4096 — large max_tokens (e.g. 128000) gây false-reject dù balance thực sự đủ

pricing = lookup_pricing(model)
IF pricing exists:
  estimated_cost = (estimated_input_tokens / 1_000_000 * pricing.input_price)
                 + (estimated_output_tokens / 1_000_000 * pricing.output_price)
ELSE:
  estimated_cost = 0  // unknown pricing, skip check
```

#### 6.2.2 Balance Check

```
// credit_balance = remaining USD balance (decreases on each deduct)
// credit_limit   = 0 means unlimited (no cap); > 0 enables credit tracking

IF credit_limit == 0:
  → SKIP (unlimited)

IF pricing == nil (unknown model):
  → SKIP pre-flight check
  → Proceed to execution
  → Actual cost calculated post-execution

IF credit_balance < estimated_cost - CREDIT_OVERDRAFT_TOLERANCE:
  → REJECT (insufficient credits)
```

### 6.3 Data Structures

**ModelPricing:**

| Field | Type | Ý nghĩa |
|-------|------|---------|
| `id` | string | Unique ID |
| `provider_id` | string | Provider (`kiro`, `openai`, `anthropic`) |
| `model_pattern` | string | Pattern match (`claude-sonnet*`, `gpt-4*`) |
| `input_price` | float64 | Giá per 1M input tokens (USD) |
| `output_price` | float64 | Giá per 1M output tokens (USD) |
| `currency` | string | `USD` |

**Pricing Lookup Logic:**

```
1. Exact match: "kiro/claude-opus-4.6" → pricing where model_pattern == "claude-opus-4.6"
2. Pattern match: "claude-opus-4.6" → pricing where "claude-*" matches
3. Provider-specific first, wildcard fallback
4. Longer pattern wins over shorter: "claude-opus-*" > "claude-*"
```

### 6.4 Output

| Kết quả | HTTP Status | Response |
|---------|-------------|----------|
| Sufficient credits | — | Tiếp tục bước tiếp theo |
| Insufficient credits | `402` | `{ "error": { "message": "Insufficient credits", "type": "credits_error", "balance": 0.05, "required": 0.12 } }` |
| Unknown pricing | — | Skip check, proceed (cost calculated post-execution) |

### 6.5 Edge Cases

| Case | Behavior |
|------|----------|
| `credit_limit=0, credit_balance=0` | Unlimited credits, skip check |
| Model không có trong pricing table | Skip check, proceed, log warning |
| Request `max_tokens` rất lớn | Estimate cost cao, có thể reject dù balance đủ |
| Credit balance = 0.001 (rất nhỏ) | Reject nếu estimated_cost > 0.001 |

---

## 7. Step 4: Smart Model Resolver

### 7.1 Mục đích

Resolve model name thành danh sách candidate providers, không yêu cầu client biết prefix.

### 7.2 Architecture: 3-Tier Lookup

```
┌─────────────────────────────────────┐
│  Tier 1: Discovery Cache (1h TTL)  │ ← Hit rate ~95%
│  Auto-populated, in-memory LRU     │
└─────────────┬───────────────────────┘
              │ miss
              ▼
┌─────────────────────────────────────┐
│  Tier 2: Static Registry (config)  │ ← Hit rate ~4%
│  Admin-configured exact + patterns │
└─────────────┬───────────────────────┘
              │ miss
              ▼
┌─────────────────────────────────────┐
│  Tier 3: Live Discovery (probe)    │ ← Hit rate ~1%
│  Parallel HTTP probe all providers │
└─────────────────────────────────────┘
```

### 7.3 Input Processing

#### 7.3.1 Parse Model String

```
Input: normalized model string từ Step 2

IF contains "/":
  split on first "/"
  prefix = parts[0]  // "kr", "kiro", "oai", "openai"
  model = parts[1]   // "claude-opus-4.6"
  
  Resolve prefix:
    "kr"   → "kiro"
    "kiro" → "kiro"
    "oai"  → "openai"
    "glm"  → "glm"
    "mm"   → "minimax"
    "qw"   → "qwen"
    "ant"  → "anthropic"
    ... (full alias map)
  
  → Direct route: chỉ tìm connections của provider này
  
ELSE:
  No prefix → phải Smart Resolve
  → Query 3-tier registry
```

#### 7.3.2 Check Alias

```
IF model string exists trong alias map:
  resolved = alias_map[model]
  // ví dụ: "sonnet" → "kiro/claude-sonnet-4.5"
  
  Re-parse resolved value (có thể có prefix)
  → Return resolved provider + model
```

#### 7.3.3 Check Combo

```
IF model string exists trong combo map:
  combo = combo_map[model]
  // ví dụ: "fast-fallback" → ["kr/claude-opus-4.6", "oai/gpt-4o"]
  
  Return combo handler mode:
  → Không resolve provider ở đây
  → Defer đến Combo Executor (Section 7.6)
```

### 7.4 Tier 1: Discovery Cache

**Lý do tồn tại:** Hầu hết requests dùng lại models đã dùng trước. Cache kết quả discovery tránh repeat lookup.

| Parameter | Value |
|-----------|-------|
| Type | LRU Cache |
| TTL | 1 hour |
| Max entries | 10,000 |
| Hit rate (expected) | ~95% |

**Cache Key:** `model_string` (e.g., `"claude-opus-4.6"`)

**Cache Value:**

| Field | Type | Ý nghĩa |
|-------|------|---------|
| `original_model` | string | Model name client gửi |
| `providers` | []string | Providers hỗ trợ model này |
| `match_type` | string | `"cache_hit"` |
| `resolved_at` | time.Time | Khi nào cache entry được tạo |

**Invalidation:**

| Event | Action |
|-------|--------|
| TTL expired (1h) | Remove entry, next request triggers Tier 2/3 |
| Admin adds/removes connection | Invalidate all entries for affected provider |
| Admin updates static registry | Invalidate affected entries |
| Admin updates `connection.supported_models` | Invalidate all Tier 1 entries for that provider |
| Connection health change | NO invalidation (health check ở Step 7) |

### 7.5 Tier 2: Static Registry

**Lý do tồn tại:** Admin có thể pre-configure common models để tránh discovery latency.

**Data source:** `config.json` → `model_registry` section

#### 7.5.1 Exact Match

```
model_registry.exact_match = {
  "gpt-4o": ["openai"],
  "claude-opus-4.6": ["kiro", "anthropic"],
  "claude-sonnet-4.5": ["kiro", "anthropic"],
  "glm-4-plus": ["glm"]
}
```

**Logic:** Lookup `model_string` trong exact_match map. Nếu found → return providers list.

**Ưu điểm:** O(1) lookup, guaranteed đúng.
**Nhược điểm:** Admin phải manually update.

#### 7.5.2 Pattern Match

```
model_registry.patterns = [
  { pattern: "claude-*", providers: ["kiro", "anthropic"], priority: 1 },
  { pattern: "gpt-*",    providers: ["openai"],             priority: 2 },
  { pattern: "glm-*",    providers: ["glm"],                priority: 3 },
  { pattern: "*",        providers: ["openai-compatible"],  priority: 999 }
]
```

**Logic:**

1. Sort patterns by priority (lower = check first)
2. For each pattern, check if model name matches
3. First match wins
4. Return providers list

**Match rules:**

| Pattern | Matches |
|---------|---------|
| `*` | Everything |
| `claude-*` | Bắt đầu bằng `claude-` |
| `*-4.6` | Kết thúc bằng `-4.6` |
| `gpt-4*` | Bắt đầu bằng `gpt-4` |
| `*turbo*` | Chứa `turbo` |

**Ưu điểm:** Cover models chưa có trong exact match.
**Nhược điểm:** Có thể false positive (pattern match nhưng provider không thực sự support).

**Nếu Tier 2 có kết quả:**
→ Cache kết quả vào Tier 1 (TTL 1h)
→ Return providers

### 7.6 Tier 3: Live Discovery

**Lý do tồn tại:** Model mới ra, chưa có trong cache hay static registry. Tự động detect.

#### 7.6.1 Parallel Probe

**Deduplication (singleflight):** Concurrent requests cùng resolve model chưa có trong cache → chỉ 1 discovery goroutine chạy, các request khác await cùng result. Tránh thundering herd dội provider. Dùng `singleflight.Group` keyed by normalized model_string.

```
result, _ = discoveryGroup.Do(model_string, func() {
  FOR each healthy connection (1 per provider):
    GO probe(connection, model_string)

  Wait all probes complete (max 5s timeout)

  return [provider_id for each probe that returned success]
})
providers = result
```

#### 7.6.2 Probe Logic

**Cách 1: Model List API (recommended)**

```
GET {provider_base_url}/models
→ Parse response, check if model_string exists in list
→ Zero cost, không tạo actual request, không consume quota
→ Response có thể cache locally
```

**Cách 2: Minimal Request (fallback cho providers không có /models)**

```
POST {provider_base_url}/chat/completions
Headers: { Authorization, Content-Type }
Body: {
  "model": "{model_string}",
  "messages": [{"role": "user", "content": "hi"}],
  "max_tokens": 1,
  "stream": false
}

Status codes:
  200  → Model exists, provider supports it ✅
  400  → Check error message:
         "model not found" / "invalid model" → NOT supported ❌
         Other 400 → Model might exist, different error → MAYBE supported ⚠️
  401/403 → Auth issue, can't determine → SKIP
  404  → Model not found ❌
  429  → Rate limited but model exists → MAYBE supported ⚠️
  500+ → Server error, can't determine → SKIP
```

**⚠️ Lưu ý Cách 2:** Tạo actual request → tốn tokens/cost trên paid providers, consume rate limit. Chỉ dùng khi Cách 1 không available.

**Probe Strategy:**

| Provider | Method | Endpoint | Lý do |
|----------|--------|----------|-------|
| OpenAI | Model list ✅ | GET /v1/models | Zero cost |
| GLM | Model list ✅ | GET /v4/models | Zero cost |
| Qwen | Model list ✅ | GET /v1/models | Zero cost |
| OpenAI-Compatible | Model list ✅ | GET /v1/models | Zero cost (try first, fallback to minimal) |
| Kiro | Minimal request | POST generateAssistantResponse | No model list API |
| Anthropic | Minimal request | POST /v1/messages | No model list API |
| MiniMax | Minimal request | POST /v1/text/chatcompletion | No model list API |

#### 7.6.3 Result Processing

```
IF found providers:
  → Cache result into Tier 1 (TTL 1h)
  → Return providers

IF no providers found:
  miss_count = increment(model_string + ":miss")  // in-memory counter, reset on cache hit
  ttl = min(1min * 2^(miss_count-1), 15min)  // 1m → 2m → 4m → 8m → 15m (capped)
  → Cache negative result vào Tier 1 (TTL = ttl)
  → Return 404 Model Not Found
```

**Negative caching:** Cache "not found" để tránh repeat expensive probes. TTL ngắn (1 min) vì model có thể được deploy bất cứ lúc nào (development workflow).

**Manual cache invalidation:** Admin có thể clear cache qua `POST /api/cache/invalidate` (xem Section 13.4).

### 7.7 Resolver Output

**ResolvedModel:**

| Field | Type | Ý nghĩa |
|-------|------|---------|
| `original_model` | string | Model name client gửi (`"claude-opus-4.6"`) |
| `resolved_model` | string | Provider-specific model name (có thể khác original) |
| `providers` | []string | Candidate providers (`["kiro", "anthropic"]`) |
| `match_type` | string | `"prefix"`, `"exact"`, `"pattern"`, `"discovery"`, `"alias"` |
| `is_combo` | bool | `true` nếu là combo name |
| `combo_models` | []string | Model list (nếu is_combo) |

---

## 8. Step 5: Check Permissions

### 8.0.1 Mục đích

Kiểm tra API key có quyền dùng model/provider đã resolved. Chạy **sau** Smart Resolver vì cần biết model thuộc provider nào.

### 8.0.2 Logic

#### Model Permission

```
IF allowed_models == ["*"]
  → PASS (tất cả models)
ELSE
  → Check resolved_model against allowed_models list
  → Support wildcard: "claude-*" matches "claude-opus-4.6"
```

**Match rules:**

| Pattern | Matches | Not matches |
|---------|---------|-------------|
| `*` | Everything | — |
| `claude-*` | `claude-opus-4.6`, `claude-sonnet-4.5` | `gpt-4o` |
| `gpt-4*` | `gpt-4o`, `gpt-4-turbo` | `gpt-3.5` |
| `claude-opus-4.6` | `claude-opus-4.6` (exact) | `claude-opus-4.7` |

#### Provider Permission

```
IF allowed_providers == ["*"]
  → PASS (tất cả providers)
ELSE
  → Check resolved_providers against allowed_providers
  → Must have at least 1 provider in common
  → Filter resolved_providers to only allowed ones
```

#### Expiry Check

```
IF expires_at != nil AND now > expires_at
  → REJECT
```

### 8.0.3 Output

| Kết quả | HTTP Status | Response |
|---------|-------------|----------|
| Pass | — | Tiếp tục bước 6 (Pre-flight Credit Check) |
| Model không allowed | `403` | `{ "error": { "message": "Model not allowed for this key", "type": "permission_error" } }` |
| Provider không allowed | `403` | `{ "error": { "message": "Provider not allowed for this key", "type": "permission_error" } }` |
| Key hết hạn | `403` | `{ "error": { "message": "API key expired", "type": "permission_error" } }` |



## 9. Step 7: Connection Discovery

### 9.1 Mục đích

Tìm tất cả connections available cho resolved providers, filter theo health, và xếp hạng theo priority.

### 9.2 Logic

#### 9.2.1 Collect Connections

```
FOR each provider in resolved_providers:
  connections = config.connections.filter(c => c.provider_id == provider)
  candidates.extend(connections)
```

#### 9.2.2 Filter: IsActive

```
IF connection.is_active == false:
  SKIP
```

#### 9.2.3 Filter: Supports Model

```
IF connection.supported_models == ["*"]:
  PASS (supports everything)

FOR each pattern in connection.supported_models:
  IF match(model, pattern):
    PASS

IF no pattern matches:
  SKIP (this connection doesn't support this model)
```

#### 9.2.4 Filter: Cooldown Check

```
IF connection.cooldown_until != nil:
  IF now.Before(connection.cooldown_until):
    SKIP (still in cooldown)
  ELSE:
    PASS (cooldown expired)
ELSE:
  PASS (not in cooldown)
```

#### 9.2.5 Filter: Model Lock Check

```
IF model exists in connection.model_locks:
  lock_until = connection.model_locks[model]
  IF now.Before(lock_until):
    SKIP (model locked on this connection)
  ELSE:
    PASS (lock expired, auto-clear)
ELSE:
  PASS (no lock for this model)
```

#### 9.2.6 Auto-Refresh Token

```
IF connection.auth_type == "oauth":
  IF connection.credentials.expires_at != nil:
    IF now.Add(5 * time.Minute).After(connection.credentials.expires_at):
      // Token expires within 5 minutes
      TRY refresh_token(connection):
        SUCCESS → update credentials in-memory
        FAIL → log warning, continue with current token
                (might fail at execution, will trigger fallback)
```

**Token refresh:** Background scheduler (`token-refresh-scheduler.go`) xử lý proactive refresh (5-min buffer trước expiry). Sync refresh trong selection flow chỉ là fallback nếu background scheduler missed (e.g. sau process restart). Sync refresh thường <100ms — không phải tiêu chuẩn latency thông thường nhưng acceptable cho edge case này.

#### 9.2.7 Sort by Priority

```
SORT candidates BY priority ASC (lower number = higher priority)

IF same priority:
  SORT BY consecutive_use_count ASC (prefer less-used)
```

### 9.3 Output

**Ranked connection list**, ví dụ:

```
1. kiro/conn-1    (priority=1, healthy, token fresh)
2. anthropic/conn-1 (priority=2, healthy)
3. kiro/conn-2    (priority=5, healthy)
```

### 9.4 Edge Cases

| Case | Behavior |
|------|----------|
| No connections found | Return `503 No Available Connections` |
| All connections in cooldown | Return `503` with info about when cooldowns expire |
| Token refresh fails | Still include connection, let execution fail naturally → triggers fallback |
| Connection vừa bị delete | In-memory cache stale for max 10s (acceptable) |

---

## 10. Step 8: Execution Loop

### 10.1 Mục đích

Thực hiện request trên ranked connections, fallback tự động khi fail, với bounded retry.

### 10.2 Loop Parameters

| Parameter | Value | Ý nghĩa |
|-----------|-------|---------|
| `max_attempts` | `len(ranked_connections)` | Không retry quá số connections available |
| `exclude_ids` | `map[string]bool` | Connections đã thử và fail |
| `last_error` | `error` | Error từ lần thử gần nhất |

### 10.3 Loop Logic

```
exclude_ids = {}
last_error = nil

FOR attempt = 0; attempt < max_attempts; attempt++:

  // Pick next candidate (skip excluded)
  candidate = first connection NOT in exclude_ids
  IF candidate == nil:
    BREAK (all exhausted)

  // Execute request
  response, error = execute(candidate, model, body)

  IF error == nil:
    // SUCCESS
    clear_cooldown(candidate)
    clear_model_lock(candidate, model)
    reset_backoff(candidate)
    RETURN response  ← EXIT LOOP

  // FAILURE
  classification = classify_error(error)

  IF classification == NON_RETRYABLE:
    RETURN error  ← EXIT LOOP (don't waste attempts)

  IF classification == RETRYABLE:
    mark_unhealthy(candidate, error)
    exclude_ids[candidate.id] = true
    last_error = error
    CONTINUE  ← Try next candidate

// All attempts exhausted
RETURN last_error (wrapped: "all connections failed")
```

### 10.4 Error Classification

#### 10.4.1 NON_RETRYABLE (Return ngay, không fallback)

| Condition | HTTP Status | Lý do |
|-----------|-------------|-------|
| Client error | `400` | Lỗi từ client, mọi connection đều sẽ fail |
| Method not allowed | `405` | API format sai |
| Payload too large | `413` | Request quá lớn |
| Unprocessable | `422` | Format request sai |
| Content type error | `415` | Sai content type |
| Client error hints | Any | Error text chứa: `"invalid api key"`, `"model not found"`, `"invalid request"` |

**Logic:** Nếu lỗi là do request本身, thì retry connection khác cũng sẽ y chang. Skip.

#### 10.4.2 RETRYABLE (Fallback, thử connection khác)

| Condition | Cooldown | Backoff | Model Lock |
|-----------|----------|---------|------------|
| `401 Unauthorized` | 5 min | Level +1 | No |
| `403 Forbidden` | 5 min | Level +1 | No |
| `402 Payment Required` | 5 min | Level +1 | No |
| `429 Too Many Requests` | 1 min | Level +1 | Yes |
| `500 Internal Server` | 2s | Level +1 | Yes |
| `502 Bad Gateway` | 2s | Level +1 | Yes |
| `503 Unavailable` | 2s | Level +1 | Yes |
| `504 Gateway Timeout` | 2s | Level +1 | Yes |
| Timeout (no response) | 2s | Level +1 | Yes |
| Connection refused | 2s | Level +1 | Yes |
| Rate limit keywords in error text | 1 min | Level +1 | Yes |
| Unknown status (default) | 2s | Level +1 | Yes |

**Note:** Default behavior là fallback (fail-open), vì tốt hơn là thử connection khác thay vì reject.

**Auth/Payment (401/402/403) không dùng Model Lock** vì đây là connection-level issue (token expired, quota exhausted), không phải model-specific. Model lock chỉ có nghĩa khi 1 connection không support model cụ thể (e.g. rate limit cho riêng model đó).

#### 10.4.3 Rate Limit Keywords (trong error text)

Được match case-insensitive:

```
"rate limit"
"too many requests"
"quota exceeded"
"capacity"
"overloaded"
"request not allowed"
```

### 10.5 Cooldown Mechanism

#### 10.5.1 Cooldown Duration

```
IF classification.use_fixed_cooldown:
  cooldown = classification.cooldown  // Fixed, no backoff
ELSE:
  base_duration = classification.cooldown
  backoff_multiplier = 2 ^ backoff_level
  cooldown = min(base_duration * backoff_multiplier, MAX_COOLDOWN)

MAX_COOLDOWN = 2 minutes
```

**Ví dụ (transient error, base=2s, exponential backoff):**

| Level | Cooldown |
|-------|----------|
| 0 | 2s |
| 1 | 4s |
| 2 | 8s |
| 3 | 16s |
| 4 | 32s |
| 5 | 64s |
| 6 | 120s (capped) |
| 7+ | 120s (capped) |

**Auth/Payment errors (401/402/403) — Fixed cooldown, NO backoff:**

| Error | Fixed Cooldown | Lý do |
|-------|---------------|-------|
| 401 Unauthorized | 5 min | Token expired/invalid — cần human intervention hoặc auto-refresh |
| 402 Payment Required | 5 min | Quota exhausted — cần admin top-up |
| 403 Forbidden | 5 min | Permission issue — cần admin fix |

Auth errors dùng fixed cooldown (không exponential) vì:
1. `base=5min > MAX_COOLDOWN(2min)` → exponential backoff vô nghĩa
2. Auth issues thường cần human intervention, backoff không giúp tự heal
3. Sau cooldown, nếu vẫn fail → lại cooldown 5min, đủ để admin notice

#### 10.5.2 Model Lock

```
Ngoài connection-level cooldown, còn có model-level lock:
  connection.model_locks[model] = cooldown_until

Lý do: Connection có thể bị rate limit cho model cụ thể,
nhưng vẫn available cho models khác.
```

#### 10.5.3 Success Recovery

```
Khi request thành công:
  connection.cooldown_until = nil
  connection.backoff_level = 0
  delete(connection.model_locks, model)
  connection.status = "healthy"
```

### 10.6 Combo Execution

Khi Step 5 phát hiện request là combo:

#### 10.6.1 Fallback Strategy

```
models = combo.models  // ["kr/claude-opus-4.6", "oai/gpt-4o"]
FOR each model in models:
  resolve = smart_resolve(model)
  connections = find_available(resolve)
  
  FOR each connection in connections:
    response, error = execute(connection, model, body)
    
    IF error == nil:
      RETURN response  ← SUCCESS
    
    IF classification == NON_RETRYABLE:
      BREAK  ← Skip to next model
    
    // Classification == RETRYABLE
    mark_unhealthy(connection, error)
    CONTINUE  ← Try next connection for this model
  
  // All connections failed for this model
  IF is_transient_error(last_error):
    SLEEP 2 seconds  ← Back off before next model
  
  CONTINUE  ← Try next model

// All models failed
RETURN error ("all combo models failed: ...")
```

#### 10.6.2 Round-Robin Strategy

```
models = combo.models
start_index = get_rotation_index(combo.name)

rotated_models = rotate(models, start_index)
// Ví dụ: [A, B, C] start=1 → [B, C, A]

// Execute same as fallback, nhưng với rotated order
// On success: increment rotation_index for next request
```

**Rotation state:**

```
rotation_state sync.Map
// key: combo_name
// value: current_index (int)

Increment on each successful request:
  rotation_index = (rotation_index + 1) % len(models)
```

**Note:** Round-robin là approximately fair under concurrency. Không guarantee strictly fair vì non-atomic read-increment-write. Acceptable trade-off.

### 10.7 Output

| Kết quả | HTTP Status | Behavior |
|---------|-------------|----------|
| Success | `200` | Return SSE stream |
| All connections failed | `502` | `{ "error": { "message": "All connections failed: ...", "type": "upstream_error" } }` |
| Non-retryable error | Original status | Forward provider error to client |
| No connections | `503` | Service unavailable |
| Combo all failed | `502` | `{ "error": { "message": "All combo models failed: ...", "type": "combo_error" } }` |

### 10.8 Streaming Partial Failure

Khi connection **fail mid-stream** (sau khi đã bắt đầu trả SSE response cho client):

#### 10.8.1 Problem

- Client đã nhận partial data (some SSE chunks sent)
- HTTP status 200 đã trả → không thể thay đổi
- Không thể retry với connection khác (response đã partial, context khác)
- Phải notify client về failure trong SSE stream

#### 10.8.2 Behavior

```
IF streaming started AND connection fails mid-stream:
  1. Emit error event:
     data: {"error":{"message":"Upstream connection lost","type":"stream_error","code":"connection_lost"}}
     // Khi lỗi do server shutdown gracefully: dùng code "server_shutdown" để client phân biệt và auto-retry
  
  2. Emit done signal:
     data: [DONE]
  
  3. Close SSE connection
  
  4. Log as partial failure:
     - status = "partial_failure"
     - tokens counted = usage.total_tokens nếu provider đã gửi usage event (thường chunk cuối)
                       ELSE character-estimate từ streamed content: len(streamed_text) / 4
     - cost = based on actual tokens; nếu usage chưa arrive thì dùng estimate (log rõ là estimated)
  
  5. Mark connection unhealthy (same as execution loop failure)
  
  6. Do NOT retry with another connection
```

#### 10.8.3 Client Handling Guidance

Clients nên handle `stream_error` event:
- Nếu partial response hữu ích → display what was received
- Nếu cần full response → retry toàn bộ request

---

## 11. Step 9: Post-Execution (Async)

### 11.1 Mục đích

Thực hiện các tác vụ không cần block response: deduct credits, log request.

### 11.2 Credit Deduction

#### 11.2.1 Calculate Actual Cost

```
usage = response.usage  // từ provider response

pricing = lookup_pricing(resolved_provider, resolved_model)

IF pricing exists:
  input_cost = usage.input_tokens / 1_000_000 * pricing.input_price
  output_cost = usage.output_tokens / 1_000_000 * pricing.output_price
  total_cost = input_cost + output_cost
ELSE:
  total_cost = 0  // unknown pricing, no deduction
```

#### 11.2.2 Deduct Balance

```
ASYNC:
  config.mu.Lock()
  key.credit_balance -= total_cost
  key.updated_at = now
  config.mu.Unlock()
  
  saveQueue <- saveRequest{}  // enqueue vào dedicated writer goroutine (serialized)
  // ⚠️ config.save() KHÔNG được gọi ngoài mutex — concurrent deductions có thể override nhau khi write
```

#### 11.2.3 Credit Transaction Log

```
ASYNC:
  transaction = CreditTransaction {
    id:             generate_id("txn"),
    api_key_id:     key.id,
    type:           "deduct",
    amount:         -total_cost,
    balance_before: old_balance,
    balance_after:  new_balance,
    request_log_id: request_id,
    created_at:     now,
  }
  // Append to credit_transactions in config
```

#### 11.2.4 Edge Cases

| Case | Behavior |
|------|----------|
| Balance becomes negative | Accept (overdraft), next request will fail at pre-flight |
| Pricing not found | No deduction, log warning |
| Deduction fails (concurrent write) | Log error, continue (manual reconciliation later) |
| Process crash during deduction | Balance not updated, but response already sent → drift (acceptable) |

### 11.3 Request Logging

#### 11.3.1 Log Entry

| Field | Source |
|-------|--------|
| `id` | Generated UUID |
| `api_key_id` | From Step 1 |
| `model` | Original model string from request |
| `combo_name` | If combo was used |
| `provider_id` | Resolved provider |
| `account_id` | Connection that handled the request |
| `resolved_model` | Actual model sent to provider |
| `started_at` | Request start timestamp (ms) |
| `duration_ms` | Total request duration |
| `input_tokens` | From provider response |
| `output_tokens` | From provider response |
| `total_tokens` | Sum |
| `input_cost` | Calculated |
| `output_cost` | Calculated |
| `total_cost` | Calculated |
| `status_code` | HTTP status from provider |
| `error` | Error message if failed |
| `request_id` | Correlation ID |
| `user_agent` | From request header |

#### 11.3.2 Async Batch Writer

```
Queue: buffered channel (capacity 2048)

Flush conditions (whichever first):
  - Batch size >= 100 entries
  - Time since last flush >= 1 second

On flush:
  INSERT INTO request_logs VALUES (...), (...), (...) (batch)

Backpressure:
  IF queue full:
    DROP entry
    Log warning to stderr

Shutdown (SIGTERM/SIGINT):
  1. Stop accepting new requests (close listener)
  2. Wait for in-flight requests to complete (max 30s timeout)
  3. After timeout: force-close remaining SSE streams với `stream_error` (code: `server_shutdown`)
  4. Drain log queue (max 5s)
  5. Final flush to SQLite
  6. Flush pending credit deductions to db.json
  7. Close SQLite connection
  8. Exit
```

### 11.4 Rate Limit TPM Update (Post-Flight)

```
ASYNC:
  actual_tokens = usage.total_tokens
  bucket = get_bucket(key_id + ":tpm")
  bucket.tokens -= actual_tokens  // Deduct actual usage
  // Note: bucket.tokens can go negative → next request will be rejected
```

---

## 12. Response Format

### 12.1 Success Response

```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1713189261,"model":"claude-opus-4.6","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1713189261,"model":"claude-opus-4.6","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

...

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1713189261,"model":"claude-opus-4.6","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}

data: [DONE]
```

### 12.2 Error Response Format

```json
{
  "error": {
    "message": "Human-readable error message",
    "type": "error_type",
    "code": "error_code"
  }
}
```

### 12.3 Error Types

| Type | Code | HTTP Status | Khi nào |
|------|------|-------------|---------|
| `auth_error` | `invalid_api_key` | 401 | API key không hợp lệ |
| `auth_error` | `api_key_disabled` | 401 | Key bị disable |
| `auth_error` | `api_key_expired` | 401 | Key hết hạn |
| `permission_error` | `model_not_allowed` | 403 | Key không có quyền dùng model |
| `permission_error` | `provider_not_allowed` | 403 | Key không có quyền dùng provider |
| `credits_error` | `insufficient_credits` | 402 | Không đủ credit |
| `rate_limit_error` | `rpm_exceeded` | 429 | Vượt RPM limit |
| `rate_limit_error` | `tpm_exceeded` | 429 | Vượt TPM limit |
| `model_error` | `model_not_found` | 404 | Model không tồn tại |
| `model_error` | `no_available_connections` | 503 | Không có connection available |
| `upstream_error` | `all_connections_failed` | 502 | Tất cả connections fail |
| `upstream_error` | `provider_error` | 502 | Provider trả lỗi |
| `combo_error` | `all_models_failed` | 502 | Tất cả combo models fail |
| `stream_error` | `connection_lost` | — (in SSE) | Connection fail mid-stream |
| `stream_error` | `server_shutdown` | — (in SSE) | Server shutdown gracefully, client nên auto-retry |

---

## 13. Connection Health State Machine

### 13.1 States

```
                    ┌──────────┐
          ┌────────│  Healthy  │◄──────────────┐
          │        └─────┬─────┘               │
          │              │                     │
          │    Request fails                   │
          │    (retryable error)               │
          │              │                     │
          │              ▼                     │
          │        ┌──────────┐               │
          │        │ Cooldown │───────────────┤
          │        └─────┬─────┘  Cooldown     │
          │              │       expires       │
          │              │                     │
          │              ▼                     │
          │        ┌──────────┐               │
          │        │ Backoff  │───────────────┤
          │        │ Level N  │  Backoff       │
          │        └─────┬─────┘  reset        │
          │              │  on success         │
          │              │                     │
          │    Request fails again             │
          │    (retryable error)               │
          │              │                     │
          │              ▼                     │
          │        ┌──────────┐               │
          │        │ Backoff  │               │
          │        │ Level N+1│               │
          │        └─────┬─────┘               │
          │              │                     │
          │              │ ...repeat...         │
          │              │                     │
          │              │ Max backoff reached │
          │              ▼                     │
          │        ┌──────────┐               │
          │        │ Disabled │               │
          │        │ (Manual) │               │
          │        └──────────┘               │
          │                                    │
          │    Request succeeds                │
          └────────────────────────────────────┘
```

### 13.2 State Transitions

| From | To | Trigger | Action |
|------|----|---------|--------|
| Healthy | Cooldown | Retryable error | Set `cooldown_until`, increment `backoff_level`, set `model_lock` |
| Cooldown | Healthy | Request succeeds | Clear `cooldown_until`, reset `backoff_level`, clear `model_lock` |
| Cooldown | Cooldown | Another retryable error | Increase `backoff_level`, extend `cooldown_until` |
| Healthy | Disabled | Admin disables | Set `is_active = false` |
| Disabled | Healthy | Admin enables | Set `is_active = true`, clear `cooldown_until`, reset `backoff_level` |
| Any | Unauthorized | 401/403 error | Set `status = "unauthorized"`, needs admin intervention |

### 13.3 Auto-Recovery

| Condition | Recovery |
|-----------|----------|
| Cooldown expires | Connection eligible again for selection |
| Backoff level >= 7 | Cooldown capped at 2 minutes, no escalation |
| Successful request | Full reset: cooldown cleared, backoff = 0, locks cleared |
| Token refresh succeeds | Status → healthy (if was unauthorized due to expired token) |

---

## 14. Configuration Reference

### 14.1 db.json Structure

> **Note:** Persistence file is `~/.dntproxy/db.json` (not `config.json`).

```json
{
  "api_keys": [...],
  "provider_accounts": [...],
  "combos": [...],
  "model_aliases": {...},
  "model_registry": {
    "exact_match": {...},
    "patterns": [...]
  },
  "model_pricing": [...],
  "settings": {...}
}
```

### 14.2 Settings

| Setting | Type | Default | Ý nghĩa |
|---------|------|---------|---------|
| `port` | int | 20199 | HTTP listen port |
| `require_api_key` | bool | true | Có yêu cầu API key không |
| `max_retry_per_request` | int | 10 | Max fallback attempts |
| `combo_strategy` | string | "fallback" | Default combo strategy |
| `discovery_enabled` | bool | true | Bật/tắt live discovery |
| `discovery_cache_ttl` | string | "1h" | Discovery cache TTL |
| `discovery_probe_timeout` | string | "5s" | Probe timeout |
| `log_retention_days` | int | 30 | SQLite log retention |
| `tunnel_enabled` | bool | false | Cloudflare tunnel |

### 14.3 Cooldown Constants

| Constant | Value | Backoff | Khi nào |
|----------|-------|---------|--------|
| `COOLDOWN_TRANSIENT` | 2s | Exponential | 500/502/503/504/timeout |
| `COOLDOWN_RATE_LIMITED` | 60s | Exponential | 429 |
| `COOLDOWN_UNAUTHORIZED` | 300s | **Fixed** | 401/403 (needs human intervention) |
| `COOLDOWN_PAYMENT` | 300s | **Fixed** | 402 (needs admin top-up) |
| `COOLDOWN_NOT_FOUND` | 1s | Exponential | 404 |
| `BACKOFF_MAX` | 120s | — | Cap for exponential backoff |
| `BACKOFF_MAX_LEVEL` | 7 | — | Stop escalating |
| `TOKEN_REFRESH_BUFFER` | 5m | — | Refresh before expiry |
| `CREDIT_OVERDRAFT_TOLERANCE` | 0.01 | — | Allow small overdraft |
| `SHUTDOWN_REQUEST_TIMEOUT` | 30s | — | Wait for in-flight requests |
| `SHUTDOWN_DRAIN_TIMEOUT` | 5s | — | Drain log queue |
| `NEGATIVE_CACHE_TTL` | 1min | — | Cache "model not found" |

### 14.4 Cache Invalidation API

```
POST /api/cache/invalidate
Body: { "model": "deepseek-v3" }  // optional, omit to clear all

Response: { "cleared": 1 }
```

Clears discovery cache (Tier 1) entries. Useful when deploying new models to OpenAI-compatible backends.

---

## 15. Monitoring & Observability

### 15.1 Metrics (từ request_logs)

| Metric | Calculation |
|--------|-------------|
| Total requests | COUNT(*) WHERE time > now - 1h |
| Error rate | COUNT(error != '') / COUNT(*) |
| P50/P95/P99 latency | PERCENTILE(duration_ms, 50/95/99) |
| Tokens per key | SUM(total_tokens) GROUP BY api_key_id |
| Cost per key | SUM(total_cost) GROUP BY api_key_id |
| Per-provider error rate | COUNT(error) GROUP BY provider_id |
| Fallback rate | Requests that tried > 1 connection |
| Cache hit rate | Tier 1 hits / total resolves |

### 15.2 Alert Conditions

| Alert | Condition |
|-------|-----------|
| All connections down | 0 healthy connections for any provider |
| High error rate | > 50% errors in 5 minutes |
| Credit balance low | Any key balance < 1.00 |
| Discovery probe slow | Tier 3 latency > 2s |
| Queue full | Log queue drop count > 0 |
| Token refresh failing | Consecutive refresh failures > 3 |

---

## 16. End-to-End Examples

### 16.1 Example 1: Simple Request, Cache Hit

```
Client → POST /v1/chat/completions
         Authorization: Bearer pk-abc123
         { "model": "claude-opus-4.6", "messages": [...] }

Step 1: Validate pk-abc123 → key_hash match ✅
Step 2: Normalize "claude-opus-4.6" → "claude-opus-4.6" (no change)
Step 3: RPM=60, bucket has 45 tokens ✅
Step 4: Resolve "claude-opus-4.6"
        → Tier 1: CACHE HIT → providers=["kiro","anthropic"]
Step 5: allowed_models=["*"], allowed_providers=["*"] ✅
Step 6: balance=100.00, estimated=0.15 ✅
Step 7: Find connections
        → kiro/conn-1 (prio=1, healthy)
        → anthropic/conn-1 (prio=2, healthy)
Step 8: Attempt 1: kiro/conn-1 → SUCCESS ✅
Step 9: Credit -0.12, Log inserted

→ 200 SSE stream
```

### 16.2 Example 2: No Prefix, Discovery Needed

```
Client → POST /v1/chat/completions
         { "model": "deepseek-v3", "messages": [...] }

Step 1-3: All pass ✅
Step 4: Resolve "deepseek-v3"
        → No prefix
        → Tier 1: MISS (never seen before)
        → Tier 2: MISS (not in static registry)
        → Tier 3: Live Discovery (prefer Model List API)
           openai: GET /v1/models → not in list ❌
           glm: GET /v4/models → not in list ❌
           qwen: GET /v1/models → not in list ❌
           openai-compat: GET /v1/models → found ✅
           kiro: minimal request → 404 ❌
           anthropic: minimal request → 404 ❌
           minimax: minimal request → 404 ❌
        → providers=["openai-compatible"]
        → Cache "deepseek-v3" → ["openai-compatible"] for 1h
Step 5-6: Permissions + Credit check ✅
Step 7: Find connections for openai-compatible
        → conn-custom-1 (prio=10, healthy)
Step 8: Attempt 1: conn-custom-1 → SUCCESS ✅

→ 200 SSE stream
```

### 16.3 Example 3: Fallback Chain

```
Client → POST /v1/chat/completions
         { "model": "gpt-4o", "messages": [...] }

Step 1-3: All pass ✅
Step 4: Resolve "gpt-4o"
        → Tier 1: CACHE HIT → providers=["openai","kiro"]
Step 5-6: Permissions + Credit check ✅
Step 7: Find connections
        → kiro/conn-1 (prio=1, healthy)
        → openai/conn-1 (prio=3, healthy)
Step 8:
  Attempt 1: kiro/conn-1 → 401 Unauthorized ❌
    → shouldFallback? YES
    → cooldown 5min (fixed, no backoff), model lock "gpt-4o" → 5min
    → excludeIDs = {kiro-1}
  
  Attempt 2: openai/conn-1 → 429 Rate Limited ❌
    → shouldFallback? YES
    → cooldown 1min, backoff level 1, model lock → 1min
    → excludeIDs = {kiro-1, openai-1}
  
  All exhausted → RETURN 502 "All connections failed"

→ 502 { "error": { "message": "All connections failed: rate limited", "type": "upstream_error" } }
```

### 16.4 Example 4: Combo with Fallback

```
Client → POST /v1/chat/completions
         { "model": "fast-fallback", "messages": [...] }

Step 1-3: All pass ✅
Step 4: Resolve "fast-fallback"
        → Combo detected: models=["kr/claude-opus-4.6", "oai/gpt-4o"]
Step 5-6: Permissions + Credit check ✅
Step 7: Not applicable (combo handles its own routing)
Step 8: Combo Execution (fallback strategy):
  
  Model 1: "kr/claude-opus-4.6"
    → Resolve: providers=["kiro"]
    → Connections: kiro/conn-1 (prio=1)
    → Execute: 500 Internal Server Error ❌
    → Mark unhealthy, cooldown 2s
    → SLEEP 2s (transient error)
  
  Model 2: "oai/gpt-4o"
    → Resolve: providers=["openai"]
    → Connections: openai/conn-1 (prio=3)
    → Execute: SUCCESS ✅
    → Return response

→ 200 SSE stream
```

### 16.5 Example 5: Client Error (No Fallback)

```
Client → POST /v1/chat/completions
         { "model": "gpt-4o", "messages": [] }

Step 1-3: All pass ✅
Step 4: Resolve "gpt-4o" → providers=["openai","kiro"]
Step 5-6: Permissions + Credit check ✅
Step 7: kiro/conn-1 (prio=1), openai/conn-1 (prio=3)
Step 8:
  Attempt 1: kiro/conn-1 → 400 "messages is required" ❌
    → shouldFallback? NO (client error)
    → RETURN 400 immediately

→ 400 { "error": { "message": "messages is required", "type": "invalid_request_error" } }
```

### 16.6 Example 6: Rate Limited Key

```
Client → POST /v1/chat/completions
         Authorization: Bearer pk-limited
         { "model": "gpt-4o", "messages": [...] }

Step 1: Validate ✅
Step 2: Normalize ✅
Step 3: RPM check
        → bucket has 0 tokens (exhausted)
        → DENY

→ 429 { "error": { "message": "Rate limit exceeded", "type": "rate_limit_error", "retry_after": 8 } }
```

### 16.7 Example 7: Insufficient Credits

```
Client → POST /v1/chat/completions
         Authorization: Bearer pk-broke
         { "model": "gpt-4o", "messages": [...long prompt...] }

Step 1: Validate ✅
Step 2: Normalize ✅
Step 3: Rate limit ✅
Step 4: Resolve "gpt-4o" → providers=["openai"]
Step 5: Permissions ✅
Step 6: Credit check
        → balance=0.03, estimated_cost=0.15
        → INSUFFICIENT

→ 402 { "error": { "message": "Insufficient credits", "type": "credits_error", "balance": 0.03, "required": 0.15 } }
```

### 16.8 Example 8: Model Not Found

```
Client → POST /v1/chat/completions
         { "model": "nonexistent-model-xyz", "messages": [...] }

Step 1-3: All pass ✅
Step 4: Resolve "nonexistent-model-xyz"
        → No prefix
        → Tier 1: MISS
        → Tier 2: MISS
        → Tier 3: Live Discovery
           ALL providers → 404 ❌
        → No providers found
        → Negative cache for 1 min

→ 404 { "error": { "message": "Model not found: nonexistent-model-xyz", "type": "model_error", "code": "model_not_found" } }
```

---

## 17. Data Flow Summary

### 17.1 Hot Path (In-Memory Only)

```
Request → Key Hash Lookup → Normalize → Token Bucket (RPM)
        → Cache/Registry Lookup → Permission Scan → Credit Compare
        → Connection Filter/Sort → Execute → Response Stream

All reads from RAM. Zero DB queries. Zero disk I/O.
```

### 17.2 Async Path (Non-Blocking)

```
Success → go deduct_credits()
       → go log_request()
       → go update_rate_limit_bucket()
```

### 17.3 Rare Path (First Time / Infrequent)

```
New model → Live Discovery (200-800ms, cached for 1h)
Token refresh → OAuth flow (~100ms, during connection selection)
Config save → JSON file write (~5ms, async)
Log flush → SQLite batch insert (~10ms, every 1s)
```
