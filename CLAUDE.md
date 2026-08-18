# dntproxy

OpenAI-compatible proxy that routes requests to multiple AI providers.

## Overview

OpenAI-compatible proxy that routes requests to Kiro (AWS CodeWhisperer). Supports multi-account fallback, combo model chains, and all 4 Kiro auth methods.

## Tech Stack

- **Language:** Go 1.25+
- **HTTP:** Gin
- **CLI:** Cobra (ops only: serve / version / update)
- **Storage:** JSON file (`~/.dntproxy/db.json` on Linux/macOS, `%APPDATA%/dntproxy/db.json` on Windows)
- **File locking:** gofrs/flock

## Architecture

Clean architecture with 4 layers:

```
cmd/dntproxy/main.go          → Entry point, thin ops CLI (cobra)
internal/domain/               → Core types, no external deps
internal/port/                 → Interfaces (CredentialStore, ProviderExecutor, TokenRefresher)
internal/adapter/              → Implementations
  ├── http/                    → Gin router, handlers, SSE streaming
  ├── kiro/                    → Executor, EventStream parser, request/response translators
  ├── auth/                    → OAuth flows (Builder ID, IDC, Social, Import)
  └── storage/                 → JSON file DB with file locking
internal/service/              → Business logic orchestration
  ├── chat-service.go          → Resolve → credentials → execute → stream
  ├── model-resolver.go        → Alias, combo, provider/model parsing
  ├── account-selector.go      → Multi-account selection, cooldown, backoff
  └── combo-handler.go         → Fallback + round-robin strategies
```

## Build & Run

```bash
go build -o dntproxy ./cmd/dntproxy/
./dntproxy                     # Start on default port 20199
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
combo-name                     # Combo (fallback chain)
alias-name                     # Model alias
```

## Key Behaviors

### Request Flow
```
OpenAI request → model resolve → combo expand → account select → 
  OpenAI→Kiro translate → AWS EventStream call → 
  EventStream→OpenAI SSE transform → stream to client
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

## Config (db.json)

```json
{
  "providerConnections": [],
  "combos": [{"name": "my-combo", "models": ["kr/claude-sonnet-4-5-20250514", "kr/claude-haiku-4-5-20251001"]}],
  "modelAliases": {"sonnet": "kr/claude-sonnet-4-5-20250514"},
  "apiKeys": [],
  "settings": {
    "comboStrategy": "fallback",
    "requireApiKey": true,
    "port": 20199
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

### Phase 3: Dashboard / Management API — DONE
- React dashboard + `/api/*` for configuration (connections, combos, aliases, keys, etc.)
- CLI slimmed to ops only: `dntproxy` / `serve`, `version`, `update`

### Phase 4: Polish — TODO
- Request logging
- Graceful shutdown
- Cross-platform builds (Makefile)
