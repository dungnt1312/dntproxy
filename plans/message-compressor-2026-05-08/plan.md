---
title: "RTK-inspired Message Compressor"
description: "Pre-provider middleware that detects and compresses verbose command output (git/test/ls/log/json) inside chat messages — pure Go, no binary dependency."
status: done
priority: P2
effort: 3-5 days
branch: main
tags: [compression, middleware, tokens, performance]
created: 2026-05-08
---

# RTK-inspired Message Compressor — Implementation Plan

## Goal

Add a `MessageCompressor` that intercepts `/v1/chat/completions` and `/v1/messages` request bodies, detects verbose command output (git status/diff, go/cargo/pytest output, ls/tree, generic logs, JSON dumps), and rewrites those segments using compact, semantically-equivalent forms. Target ~60–80% byte reduction on detected segments, ~30–60% reduction on overall request body size in tool-heavy agent loops.

## Why Go-native (not RTK binary)

- RTK binary wraps CLI commands at execution time — not a content filter, can't post-process embedded message text.
- Windows is primary target (RTK native limitations).
- Go-native: <1ms overhead vs subprocess 20–50ms. No download/lifecycle to manage.

## Key Constraints

- Zero external Go deps — `encoding/json`, `regexp`, `strings`, `bufio` only.
- Each file ≤200 lines (project convention).
- Compression is **fail-open**: on parse error, compression bug, or unknown content → return original body unchanged.
- Compress only assistant `tool_calls` results / user `tool` role messages / large `user` content blocks. Never touch `system`, never touch text shorter than `MinContentLength` (default 500 chars).
- Streaming response path is untouched — compression is request-only.

## Architecture (Slot-In Middleware)

```
POST /v1/chat/completions
  → chat-handler (reads body)
  → MessageCompressor.Compress(body) []byte   ← NEW
  → chatService.HandleChat(compressedBody, …)
  → provider executor
```

The compressor is a single struct with one method `Compress(body []byte) (compressed []byte, stats Stats)`. It is constructed once in `router.go` from `Settings.Compression` and injected into both chat and messages handlers.

## Phases

| Phase | File | Description | Effort |
|-------|------|-------------|--------|
| 1 | [phase-1-core-compressor.md](./phase-1-core-compressor.md) | Compressor package + 5 filters + detector + parser | 1–2 days |
| 2 | [phase-2-settings-integration.md](./phase-2-settings-integration.md) | Domain `CompressionSettings`, JSON DB migration, settings handler | 0.5 day |
| 3 | [phase-3-pipeline-wiring.md](./phase-3-pipeline-wiring.md) | Inject compressor into chat-handler, messages-handler, router | 0.5 day |
| 4 | [phase-4-ui-toggle.md](./phase-4-ui-toggle.md) | React settings card with stats display | 0.5 day |
| 5 | [phase-5-tests.md](./phase-5-tests.md) | Unit tests for each filter + integration test | 1 day |

## Success Criteria

- `go build ./...` passes.
- All unit tests pass with real-world fixture inputs.
- A test request with 50KB `git diff` tool result gets compressed to ≤15KB, with assistant still able to parse semantic content.
- Disabling toggle in UI returns body unchanged within 1 request cycle.
- SQLite log row records `bodySize` (compressed) and `metadataJSON` carries `compressionStats` when `LogSavings: true`.

## Dependencies

- Existing: `internal/adapter/storage/sqlite-log-store-*.go` (`metadata_json` column already present).
- Existing: `internal/logger/reqlog.go` — `BodySize` field on `RequestLog`.
- New file count: 8 Go source files + 5 test files + 1 React component update.
