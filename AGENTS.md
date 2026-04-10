# dntproxy

Go port of [9Router](https://github.com/decolua/9router) — multi-provider AI proxy.

## Overview

OpenAI-compatible proxy that routes requests to multiple AI providers (Kiro, OpenAI, custom). Supports multi-account fallback, combo model chains, and all 4 Kiro auth methods.

## Tech Stack

- **Language:** Go 1.25+
- **HTTP:** Gin
- **CLI:** Cobra
- **Storage:** JSON file (`~/.dntproxy/db.json` on Linux/macOS, `%APPDATA%/dntproxy/db.json` on Windows)
- **File locking:** gofrs/flock

## Architecture

Clean architecture with 4 layers + interface-driven provider system:

```
cmd/dntproxy/main.go           → Entry point, CLI, provider registration
internal/domain/                → Core types, no external deps
internal/port/                  → Interfaces (contracts between layers)
  ├── provider-registry.go      → ProviderRegistry (dynamic provider lookup)
  ├── provider-executor.go      → ProviderExecutor (send request to provider)
  ├── chat-service.go           → ChatService + ChatResult
  ├── model-resolver.go         → ModelResolver
  ├── account-selector.go       → AccountSelector
  ├── credential-store.go       → CredentialStore (persistence)
  ├── token-refresher.go        → TokenRefresher
  └── log-store.go              → LogStore
internal/adapter/               → Implementations
  ├── provider/                 → ProviderRegistry (map-based, thread-safe)
  ├── http/                     → Gin router + split handlers:
  │   ├── router.go             → Route setup, middleware
  │   ├── api-handler.go        → RegisterAPIRoutes (route wiring only)
  │   ├── chat-handler.go       → POST /v1/chat/completions
  │   ├── connection-handler.go → Connection CRUD + detect/test
  │   ├── quota-handler.go      → Quota check (Kiro/OpenAI/Codex)
  │   ├── combo-api-handler.go  → Combo CRUD
  │   ├── alias-handler.go      → Alias CRUD
  │   ├── key-handler.go        → API Key CRUD
  │   ├── settings-handler.go   → Settings get/update
  │   ├── model-api-handler.go  → Model list + registry CRUD
  │   ├── backup-handler.go     → Backup export/import
  │   ├── usage-handler.go      → Usage/quota details
  │   └── log-handler.go        → Log list/stream/clear
  ├── kiro/                     → Kiro executor + EventStream parser
  ├── openai/                   → OpenAI executor (API + OAuth/Codex)
  ├── auth/                     → OAuth flows (Builder ID, IDC, Social, Import)
  └── storage/                  → JSON file DB + SQLite logs
internal/service/               → Business logic orchestration
  ├── chat-service.go           → Resolve → credentials → execute → stream
  ├── model-resolver.go         → Alias, combo, provider/model parsing
  ├── account-selector.go       → Multi-account selection, cooldown, backoff
  ├── combo-handler.go          → Fallback + round-robin strategies
  └── token-refresh-scheduler.go → Background token refresh
```

### Adding a New Provider
1. Implement `port.ProviderExecutor` in `internal/adapter/<provider>/`
2. Register in `cmd/dntproxy/main.go`: `providers.RegisterExecutor("name", executor)`
3. Done — routing, fallback, combos all work automatically

## Build & Run

```bash
go build -o dntproxy ./cmd/dntproxy/
./dntproxy                     # Start on default port 20128
./dntproxy --port 8080         # Custom port
./dntproxy serve --db /path/to/db.json
```

## API Endpoints

- `POST /v1/chat/completions` — OpenAI-compatible chat (streams SSE)
- `GET /v1/models` — List available models
- `GET /health` — Health check

## Model Format

```
kr/<model>                     # Kiro provider
oai/<model>                    # OpenAI provider
combo-name                     # Combo (fallback chain)
alias-name                     # Model alias
```

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
    "requireApiKey": false,
    "port": 20128
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
- Auto token refresh before expiry (5min buffer, integrated into account selector)

### Phase 3: CLI Commands — DONE
- `dntproxy auth add` (interactive, all 4 methods)
- `dntproxy auth list/remove/test`
- `dntproxy combo add/list/remove`
- `dntproxy alias set/list/remove`
- `dntproxy key generate/list/remove`

### Phase 4: Architecture Refactoring — DONE
- Port interfaces: ProviderRegistry, ChatService, ModelResolver, AccountSelector
- ProviderRegistry pattern for dynamic provider registration
- Split api-handler.go (2320 lines → 8 focused files)
- ChatResult moved to port layer for cross-package usage

### Phase 5: Polish — TODO
- Request logging
- Graceful shutdown
- Cross-platform builds (Makefile)
