# Plan: Telegram Bot Integration

**Status:** completed  
**Created:** 2026-05-15  
**Priority:** high

## Overview

Embedded Telegram bot for real-time alerts and interactive commands. Single-owner, start/stop from UI, deduplication, two-way interaction.

## Architecture

```
internal/adapter/telegram/
├── bot.go              → Bot lifecycle (Start/Stop), long-polling goroutine
├── commands.go         → Command handlers (/status, /usage, /mute, /connections, /help)
├── alerter.go          → Alert engine: subscribe logger, deduplicate, send
├── formatter.go        → Message formatting (Telegram MarkdownV2)
└── types.go            → Alert types, dedup state, config

internal/port/
└── notifier.go         → Notifier interface

domain/
└── config.go           → TelegramSettings struct added to Settings
```

## Phases

### Phase 1: Domain & Port Layer
- [ ] Add `TelegramSettings` to `domain.Settings`
- [ ] Add `Notifier` port interface in `internal/port/notifier.go`
- [ ] Add alert type constants in domain

**Files changed:**
- `internal/domain/config.go` — add TelegramSettings struct + field in Settings + DefaultConfig
- `internal/port/notifier.go` — new file

### Phase 2: Telegram Bot Adapter
- [ ] `internal/adapter/telegram/types.go` — Alert types, dedup state struct
- [ ] `internal/adapter/telegram/bot.go` — Bot struct, Start/Stop, long-polling, owner auth middleware
- [ ] `internal/adapter/telegram/commands.go` — /status, /usage, /connections, /mute, /unmute, /help
- [ ] `internal/adapter/telegram/alerter.go` — Subscribe to logger, filter errors, deduplicate (30min), send alerts, send resolved
- [ ] `internal/adapter/telegram/formatter.go` — Format alerts and command responses for Telegram MarkdownV2

**Dependencies:** `github.com/go-telegram-bot-api/telegram-bot-api/v5`

### Phase 3: Service Integration
- [ ] Wire bot into `cmd/dntproxy/main.go` — create bot, start if enabled, stop on shutdown
- [ ] Alerter subscribes to `logger.Subscribe()` channel
- [ ] Alerter queries `port.CredentialStore` for connection state
- [ ] Alerter queries `port.LogStore` for usage stats (commands)

### Phase 4: HTTP API for Bot Management
- [ ] `internal/adapter/http/telegram-handler.go` — API endpoints:
  - `GET /api/telegram/status` — bot running?, connected?, owner info
  - `POST /api/telegram/test` — send test message to owner
  - Settings save via existing `PUT /api/settings` (telegram fields included)

### Phase 5: React UI Card
- [ ] Add Telegram Bot card to `ui/src/components/screens/settings-screen.tsx`
  - Enable/disable toggle (starts/stops bot)
  - Bot token input (password field)
  - Owner ID input (number)
  - Test button (sends test message)
  - Status indicator (connected/disconnected/error)
- [ ] Update `ui/src/lib/go-api.ts` — add telegram fields to settings payload
- [ ] Update settings TypeScript interface

### Phase 6: Alert Logic Detail

| Alert Type | Trigger Condition | Dedup Key |
|------------|-------------------|-----------|
| `quota_exhausted` | LogEntry with status 402/403 | `{connID}:quota` |
| `token_expired` | LogEntry with "token refresh failed" or "token expired" | `{connID}:token` |
| `connection_down` | LogEntry ERROR + check backoffLevel >= 4 | `{connID}:down` |
| `all_down` | All connections for a provider are rate-limited/locked | `{provider}:all_down` |
| `rate_limited` | LogEntry with status 429 | `{connID}:ratelimit` |
| `combo_exhausted` | All models in combo failed (logged as final error) | `{combo}:exhausted` |
| `connection_recovered` | Connection clears error after previous alert | `{connID}:recovered` |

**Dedup rules:**
- Same key suppressed for 30 minutes after first send
- `connection_recovered` clears dedup state for that connection
- `/mute` suppresses ALL alerts for specified duration

### Phase 7: Commands Detail

| Command | Response |
|---------|----------|
| `/status` | List all connections: name, provider, status (OK/error/rate-limited), last error time |
| `/usage` | Today: total requests, tokens (in/out), estimated cost |
| `/usage 7d` | Last 7 days daily breakdown |
| `/connections` | Detailed: each connection with models, backoff level, cooldown remaining |
| `/mute 2h` | Suppress alerts for 2 hours, confirm with unmute time |
| `/unmute` | Resume alerts immediately |
| `/help` | List all commands with descriptions |

## Technical Decisions

- **Library:** `go-telegram-bot-api/telegram-bot-api/v5` — mature, well-maintained
- **Polling:** Long-polling (not webhook) — simpler, no public URL needed
- **Owner auth:** Every incoming message checked against `OwnerID`; ignore others silently
- **Lifecycle:** Bot goroutine managed by adapter; start/stop via settings toggle or API
- **Token storage:** Plain text in db.json alongside other secrets
- **No daily summary:** On-demand only via `/usage`
- **No token-expiring-soon alert:** Auto-refresh handles it

## Dependencies to Add

```
go get github.com/go-telegram-bot-api/telegram-bot-api/v5
```

## Risk & Considerations

- Bot token in db.json — same security model as API keys (acceptable per user decision)
- Long-polling goroutine must handle graceful stop (context cancellation)
- Telegram API rate limits: max 30 msg/sec to same chat — dedup handles this naturally
- If bot token invalid, Start() should return clear error shown in UI
