# dntproxy

Go-based OpenAI-compatible proxy that routes requests to multiple AI providers. Multi-provider, multi-account AI routing with tunneling, logging, and combo strategies.

## Overview

OpenAI-compatible proxy that routes requests to multiple AI providers (Kiro, OpenAI, GLM, MiniMax, Qwen, Anthropic). Supports multi-account fallback, combo model chains, multiple auth methods, Cloudflare tunneling, and structured SQLite logging.

## Tech Stack

- **Language:** Go 1.25+
- **HTTP:** Gin
- **CLI:** Cobra (ops only: serve / version / update)
- **Storage:** JSON file (`~/.dntproxy/db.json`) + SQLite logs (`~/.dntproxy/logs.db`)
- **File locking:** gofrs/flock
- **Tunneling:** cloudflared (auto-downloaded)

## Architecture

Clean architecture with 4 layers + interface-driven provider system:

```
cmd/dntproxy/main.go           → Entry point, thin ops CLI, provider registration
internal/domain/                → Core types, no external deps
internal/port/                  → Interfaces (contracts between layers)
  ├── provider-registry.go      → ProviderRegistry (dynamic provider lookup)
  ├── provider-executor.go      → ProviderExecutor (send request to provider)
  ├── chat-service.go           → ChatService + ChatResult
  ├── model-resolver.go         → ModelResolver
  ├── account-selector.go       → AccountSelector
  ├── credential-store.go       → CredentialStore (persistence)
  ├── token-refresher.go        → TokenRefresher
  ├── log-store.go              → LogStore (SQLite, 30-day retention)
  ├── model-fetcher.go          → ModelFetcher (fetch model lists)
  ├── quota-checker.go          → QuotaChecker (check usage/limits)
  └── tunnel-manager.go         → TunnelManager (cloudflare tunnel)
internal/adapter/               → Implementations
  ├── provider/                 → ProviderRegistry (map-based, thread-safe)
  ├── http/                     → Gin router + split handlers:
  │   ├── router.go             → Route setup, middleware
  │   ├── api-handler.go        → RegisterAPIRoutes (route wiring only)
  │   ├── chat-handler.go       → POST /v1/chat/completions
  │   ├── connection-handler.go → Connection CRUD + detect/test
  │   ├── connection-export-handler.go → Connection export/import
  │   ├── quota-handler.go      → Quota check (Kiro/OpenAI/Codex)
  │   ├── combo-api-handler.go  → Combo CRUD
  │   ├── alias-handler.go      → Alias CRUD
  │   ├── key-handler.go        → API Key CRUD
  │   ├── settings-handler.go   → Settings get/update
  │   ├── model-api-handler.go  → Model list + registry CRUD
  │   ├── backup-handler.go     → Backup export/import
  │   ├── usage-handler.go      → Usage/quota details
  │   ├── log-handler.go        → Log list/stream/clear
  │   └── tunnel-handler.go     → Cloudflare tunnel enable/disable/status
  ├── kiro/                     → Kiro executor + EventStream parser
  ├── openai/                   → OpenAI executor (API + OAuth/Codex)
  ├── custom/                   → NoOpModelFetcher, NoOpQuotaChecker
  ├── shared/                   → StreamingHTTPClient, body sanitization
  ├── auth/                     → OAuth flows (Builder ID, IDC, Social, Import)
  ├── tunnel/                   → Cloudflared download, lifecycle, state
  └── storage/                  → JSON file DB + SQLite logs
internal/service/               → Business logic orchestration
  ├── chat-service.go           → Resolve → credentials → execute → stream
  ├── model-resolver.go         → Alias, combo, provider/model parsing
  ├── account-selector.go       → Multi-account selection, cooldown, backoff, on-demand token refresh
  ├── combo-handler.go          → Fallback + round-robin strategies
  ├── tunnel-service.go         → Tunnel enable/disable/status
  ├── tools-service.go          → AI tool detection, configure, reset
  └── backup/                   → Backup/restore service
      ├── types.go              → BackupData, ConnectionExportData
      ├── export.go             → Full config export
      ├── import.go             → Full config import
      ├── connection-export.go  → Single/multiple connection export
      └── connection-import.go  → Single/multiple connection import
internal/logger/                → Structured logging (ring buffer + SQLite)
```

### Supported Providers

| Provider | ID | Auth Methods | Base URL | Protocol |
|----------|-----|--------------|----------|----------|
| Kiro (AWS CodeWhisperer) | `kiro` | oauth | codewhisperer.us-east-1.amazonaws.com | aws-eventstream |
| OpenAI | `openai` | apikey, oauth | api.openai.com | openai-chat |
| OpenAI Compatible | `openai-compatible` | apikey | configurable | openai-chat |
| GLM (Zhipu AI) | `glm` | apikey | api.z.ai | openai-chat |
| MiniMax | `minimax` | apikey | api.minimax.io | openai-chat |
| Qwen (Alibaba) | `qwen` | apikey, oauth | portal.qwen.ai | openai-chat |
| Anthropic | `anthropic` | apikey | api.anthropic.com | anthropic-msg |
| Gemini | `gemini` | apikey | generativelanguage.googleapis.com | openai-chat |
| Cline | `cline` | apikey | api.cline.bot | openai-chat |
| xAI / Grok | `xai` | oauth | api.x.ai | openai-chat (`/responses`) |
| BytePlus (image) | `byteplus` | apikey | ark.ap-southeast.bytepluses.com | image-only |

### Adding a New Provider
1. Add provider config in `internal/domain/provider-config.go`
2. Implement `port.ProviderExecutor` in `internal/adapter/<provider>/`
3. Register in `cmd/dntproxy/main.go`: `providers.RegisterExecutor("name", executor)`
4. Done — routing, fallback, combos all work automatically

## Build & Run

```bash
go build -o dntproxy ./cmd/dntproxy/
./dntproxy                     # Start on default port 20199
./dntproxy --port 8080         # Custom port
./dntproxy serve --db /path/to/db.json
```

## API Endpoints

### OpenAI Compatible
- `POST /v1/chat/completions` — OpenAI-compatible chat (streams SSE)
- `GET /v1/models` — List available models

### Management API
- `GET /health` — Health check
- `GET /api/connections` — List provider connections
- `POST /api/connections` — Add connection
- `POST /api/connections/import-file` — Import connection from file
- `POST /api/connections/import-multiple` — Import multiple connections from file
- `POST /api/connections/export-multiple` — Export multiple connections
- `GET /api/connections/:id/export` — Export single connection
- `DELETE /api/connections/:id` — Remove connection
- `POST /api/connections/:id/test` — Test connection
- `POST /api/connections/:id/detect` — Auto-detect models
- `GET /api/combos` — List combos
- `POST /api/combos` — Create combo
- `DELETE /api/combos/:name` — Remove combo
- `GET /api/aliases` — List aliases
- `POST /api/aliases` — Create alias
- `DELETE /api/aliases/:name` — Remove alias
- `GET /api/keys` — List API keys
- `POST /api/keys` — Generate API key
- `DELETE /api/keys/:id` — Remove API key
- `GET /api/settings` — Get settings
- `PUT /api/settings` — Update settings
- `GET /api/models` — List models + registry CRUD
- `GET /api/usage` — Usage/quota details
- `GET /api/logs` — Log list/stream/clear
- `POST /api/backup/export` — Export config
- `POST /api/backup/import` — Import config
- `POST /api/tunnel/enable` — Enable Cloudflare tunnel
- `POST /api/tunnel/disable` — Disable Cloudflare tunnel
- `GET /api/tunnel/status` — Get tunnel status
- `GET /api/quota` — Quota check

## Model Format

```
kr/<model>                     # Kiro provider
oai/<model>                    # OpenAI provider
glm/<model>                    # GLM provider
minimax/<model>                # MiniMax provider
qwen/<model>                   # Qwen provider
anthropic/<model>              # Anthropic provider
combo-name                     # Combo (fallback chain)
alias-name                     # Model alias

# NEW: Pinned account format
kr/<model>@connectionId        # Pin to specific connection
oai/<model>@conn-123           # Example: pin to conn-123
kr/<model>@auto                # Explicit auto-select (same as no suffix)
```

### Combo with Pinned Accounts

```json
{
  "name": "my-combo",
  "models": [
    "kr/claude-opus-4.6@conn-primary",    // Pin to specific connection
    "kr/claude-sonnet-4.5@conn-backup",   // Pin to different connection
    "oai/gpt-4"                            // Auto-select (default)
  ]
}
```

**Behavior:**
- Models with `@connectionId` will ONLY use that specific connection
- Models without suffix use weighted random selection (existing behavior)
- If pinned connection fails, move to next model (no retry on other connections)
- See `docs/combo-pinned-accounts.md` for detailed documentation

## Key Behaviors

### Request Flow
```
OpenAI request → model resolve → combo expand → account select →
  provider executor → translate & call → stream to client
```

### Multi-Account Fallback
- Accounts tried in priority order
- On error: exponential backoff cooldown (1s → 2s → 4s → ... → 2min max)
- Model-level locks prevent retrying failed model on same account
- On success: clear cooldown + reset backoff

### Combo Strategies
- `fallback` — try models in order until one succeeds
- `round-robin` — rotate starting model each request

### Kiro-Specific
- AWS EventStream binary protocol (not standard SSE)
- Frame format: 4B total length + 4B headers length + 4B prelude CRC + headers + payload + 4B message CRC
- Event types: `assistantResponseEvent`, `codeEvent`, `toolUseEvent`, `messageStopEvent`, `contextUsageEvent`, `meteringEvent`, `metricsEvent`
- Token usage: from `metricsEvent`, fallback to estimation from `contextUsageEvent`

### Cloudflare Tunnel
- Auto-downloads cloudflared binary (v2026.3.0) on first use
- Cross-platform: Windows, macOS, Linux (amd64/arm64)
- Persistent state: `~/.dntproxy/tunnel/state.json`
- Controlled via dashboard UI and `/api/tunnel/*` (no management CLI)

### Logging
- Structured request logs to SQLite (`~/.dntproxy/logs.db`)
- 30-day auto-retention
- Ring buffer (1000 entries) with SSE live-streaming
- Usage tracking: input/output/total tokens, estimated cost
- Model pricing profiles for cost estimation
- Raw body logging via `DNTPROXY_LOG_RAW_BODIES` env var
- Connection-level filtering

### Auth
- Dashboard (`/api/*`) and proxy (`/v1/*`) always require API keys (middleware ignores `settings.requireApiKey`)
- `GET /api/settings` is bootstrap-exempt and returns `{}` without a key; authenticated GET omits listen `port` and `requireApiKey`
- OAuth connection enrollment (`/api/auth/*`) and `fetch-models` require a dashboard-capable API key
- New connections inherit the creating key's `TenantID`
- UI login uses `localStorage` key + `POST /api/auth/validate-key` / `GET /api/auth/session`
- Keys with `dashboardAccess=false` can call `/v1/*` but cannot open the dashboard

## Code Conventions

- File naming: kebab-case (`chat-handler.go`, `request-translator.go`)
- Keep files under 200 lines where possible
- Domain types have no external dependencies
- Ports define interfaces, adapters implement them
- Services orchestrate adapters through port interfaces
- New providers just implement `ProviderExecutor` + register in main.go

## Config (db.json)

```json
{
  "providerConnections": [],
  "combos": [{"name": "my-combo", "models": ["kr/Codex-sonnet-4-5-20250514", "kr/Codex-haiku-4-5-20251001"]}],
  "modelAliases": {"sonnet": "kr/Codex-sonnet-4-5-20250514"},
  "apiKeys": [],
  "settings": {
    "comboStrategy": "fallback",
    "requireApiKey": true,
    "port": 20199,
    "tunnelEnabled": false,
    "tunnelURL": "",
    "tunnelProvider": "cloudflare"
  }
}
```

## Implementation Status

### Phase 1: Core Proxy — DONE
- Gin server, SSE streaming, CORS
- Kiro executor + AWS EventStream binary parser
- OpenAI ↔ Kiro request/response translation (text, tools, tool results)
- Multi-account selection with cooldown/backoff
- Combo handler (fallback + round-robin)
- Model resolver (aliases, combos)
- JSON DB with file locking + atomic writes

### Phase 2: Auth Flows — DONE
- AWS Builder ID (device code flow)
- AWS IAM Identity Center/IDC Enterprise (device code with custom startUrl/region)
- Google/GitHub social login (PKCE + manual callback)
- Import token (manual refresh token with auto-register)
- On-demand token refresh (integrated into account selector, no background scheduler)

### Phase 3: Dashboard / Management API — DONE
- React dashboard for connections, combos, aliases, keys, profiles, tools, tunnel, backup
- Management API under `/api/*` is the configuration control plane
- CLI slimmed to ops only: `dntproxy` / `serve`, `version`, `update`

### Phase 4: Architecture Refactoring — DONE
- Port interfaces: ProviderRegistry, ChatService, ModelResolver, AccountSelector, ModelFetcher, QuotaChecker
- ProviderRegistry pattern for dynamic provider registration
- Split api-handler.go (2320 lines → focused files)
- ChatResult moved to port layer for cross-package usage

### Phase 5: Multi-Provider + Polish — DONE
- 7 providers: Kiro, OpenAI, OpenAI-Compatible, GLM, MiniMax, Qwen, Anthropic
- Model registry with 30+ models + pricing data
- Structured logging (SQLite, 30-day retention, SSE streaming)
- Cloudflare tunnel integration
- Install scripts (Linux/macOS/Windows)
- React UI for management

### Phase 6: Next Steps — TODO
- [ ] Graceful shutdown (SIGTERM/SIGINT handling)
- [ ] Cross-platform builds (Makefile/Goreleaser)
- [ ] Dockerization (Dockerfile + docker-compose)
- [x] Anthropic adapter implementation (SSE [DONE] + system/tool translation remaining polish as needed)
- [ ] Metrics validation and monitoring

### Phase 7: Message Compressor — DONE
- `internal/adapter/compressor/` package: `Compressor`, `ContentType`, `Stats`, `Options`
- `NewWithLoader(func() Options)` — re-reads settings at most once/second (atomic, lock-free)
- 11-rule priority detector: CodeFile (skip) > GitDiff > GitStatus > GitLog > GoTest > CargoTest > Pytest > LS > Log > JSON > Generic
- 5 filters in `filters/` sub-package: `GitFilter`, `TestFilter`, `LsFilter`, `LogFilter`, `JSONFilter`
- Fail-open: any parse error / panic / ratio > 0.85 → return original body unchanged
- Injected into `chatHandler` and `messagesHandler` (post translation for `/v1/messages`)
- `domain.CompressionSettings` + `Normalize()` + backward-compatible JSON zero-fill
- Settings API + UI toggle (Token Compression card in Settings screen)
- 29 unit tests covering detector, all filters, and end-to-end compressor
