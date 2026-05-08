# Phase 3 — Pipeline Wiring

**Status:** pending
**Priority:** P1 (delivers user-visible behavior)
**Estimated:** 0.5 day

## Context Links

- Router: `internal/adapter/http/router.go`
- Chat handler: `internal/adapter/http/chat-handler.go`
- Messages handler: `internal/adapter/http/messages-handler.go`
- Reqlog: `internal/logger/reqlog.go`
- SQLite log schema: `internal/adapter/storage/sqlite-log-store-query.go`

## Overview

Construct the compressor in `router.go` from settings, inject it into both `/v1/chat/completions` and `/v1/messages` handlers, run it before `chatService.HandleChat`, and emit savings into the SQLite log row's `metadata_json` column when `LogSavings: true`.

## Key Insights

- Settings are loaded once at router-construction time from `store.GetSettings()`. The compressor needs to **re-read on each request** to honour live toggle changes from the dashboard. Lightweight: it's a `sync/atomic.Pointer[Options]` swap, not a full re-construct.
- The `messages-handler.go` already builds an OpenAI-shaped body via `translateAnthropicToOpenAI` before calling `chatService`. Compress *after* translation — the JSON shape there is identical to chat-handler input.
- `RequestLog.BodySize` is set in `Begin(...)` from `len(body)`. To record original size, capture it **before** compression; record compressed size in `metadata_json`. Simpler approach: keep `BodySize` as **compressed** (real over-the-wire to provider) and stash the original + savings in `MetadataJSON`.

## Requirements

### Functional
- `router.NewRouter` constructs a `*compressor.Compressor` instance (always — no `nil`), wraps a settings reader so the compressor can self-disable when `Enabled: false`.
- Both `chatHandler` and `messagesHandler` receive the compressor via closure capture and call `Compress(body)` on the eligible payload.
- When compression actually occurs (i.e. `len(out) < len(in)`), savings stats are merged into the `RequestLog`'s `MetadataJSON` as a JSON object: `{"compression": {"orig":N,"comp":M,"savedTokens":K,"detections":{...}}}`.
- When `Settings.Compression.Enabled: false` the compressor returns input unchanged in O(1) — no JSON parse, no work.

### Non-Functional
- Total added overhead at disabled state: ≤1µs (single atomic load + bool check).
- Total added overhead at enabled state on cold body: ≤2ms for ≤100KB.
- Live toggle: changing setting via dashboard takes effect within next request (no server restart).

## Architecture

```
router.NewRouter()
  ├── settingsLoader := func() domain.CompressionSettings { … }
  ├── comp := compressor.NewWithLoader(settingsLoader)
  ├── chatHandler(chatService, store, comp)
  └── messagesHandler(chatService, store, comp)

chatHandler:
  body := io.ReadAll(...)
  origSize := len(body)
  body, stats := comp.Compress(body)            // no-op if disabled
  reqlog.AttachCompression(origSize, stats)     // optional, only when LogSavings
  result := chatService.HandleChat(body, model, requestID)

messagesHandler:
  // … translation as today …
  openaiBody := translateAnthropicToOpenAI(...)
  origSize := len(openaiBody)
  openaiBody, stats := comp.Compress(openaiBody)
  // log + handoff identical
```

### Settings Loader Pattern

```go
// In router.go
loader := func() domain.CompressionSettings {
    s, err := store.GetSettings()
    if err != nil || s == nil { return domain.CompressionSettings{} }
    s.Compression.Normalize()
    return s.Compression
}
comp := compressor.NewWithLoader(loader)
```

`compressor.NewWithLoader` is a thin extension of phase-1's `New` that re-evaluates options on every `Compress` call. Implementation: cache the most recent `Options` in an `atomic.Pointer[Options]`; refresh once per second (timestamped) to avoid hammering `store.GetSettings()` for ultra-high-rate requests.

Inside compressor (extension of phase-1 file `compressor.go`):

```go
func NewWithLoader(load func() domain.CompressionSettings) *Compressor
// Compress() reads opts via atomic.Pointer; when refresh-due, re-loads.
```

### RequestLog Hook

Add a small method to `internal/logger/reqlog.go`:

```go
func (r *RequestLog) AttachCompression(origBytes int, stats compressor.Stats) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if r.Metadata == nil { r.Metadata = map[string]any{} }
    r.Metadata["compression"] = map[string]any{
        "orig":         origBytes,
        "comp":         stats.CompressedBytes,
        "savedTokens":  stats.TokensSaved,
        "detections":   stats.Detections,
    }
}
```

Field `Metadata map[string]any` already feeds `metadata_json` if present, OR add a `MetadataJSON` string field set via `json.Marshal(r.Metadata)` in `emitStructuredLogs()`. **Verify by reading `internal/logger/reqlog.go` — adapt to whichever serialization style is already present.** (The schema column `metadata_json TEXT` is confirmed in `sqlite-log-store-query.go`.)

## Related Code Files

### Files to modify
- `internal/adapter/http/router.go` — construct compressor, plumb to handlers.
- `internal/adapter/http/chat-handler.go` — accept compressor param, call `Compress`.
- `internal/adapter/http/messages-handler.go` — same.
- `internal/adapter/compressor/compressor.go` — add `NewWithLoader`.
- `internal/logger/reqlog.go` — add `AttachCompression` (and potentially `Metadata` field if not present).

### Files to create
- None.

## Implementation Steps

1. **Verify `RequestLog` metadata channel.** `Read internal/logger/reqlog.go` end-to-end. If `Metadata` is absent, add it as `map[string]any` plus serialization in `emitStructuredLogs()` (and field on `domain.LogEntry` if needed).
2. **Extend compressor with loader.** Add `NewWithLoader(load func() domain.CompressionSettings) *Compressor` to `compressor.go`. Keep `New(opts Options)` for tests. Internally guard with `atomic.Pointer[Options]` + a debounce timestamp.
3. **Modify `chatHandler` signature** to `func chatHandler(chatService, store, comp *compressor.Compressor) gin.HandlerFunc`. Insert compression after `io.ReadAll`/size check and before `json.Unmarshal(body, &partial)`.
4. **Modify `messagesHandler` signature** identically; compress after `translateAnthropicToOpenAI`.
5. **Update `router.NewRouter`:** build loader, build compressor, pass into both handlers.
6. **Wire savings into reqlog.** After successful `Compress`, if `Options.LogSavings == true && stats.OriginalBytes != stats.CompressedBytes`, allocate a `*logger.RequestLog`-equivalent hook (or a deferred attach: chat-service creates the reqlog inside `executeOnProvider`, so we pass the stats *via context.Context* and attach inside `executeOnProvider`'s reqlog.Begin). **Simpler alternative:** pass stats via `gin.Context` keys and attach in chat-service. **Recommended:** add a side log entry from the handler via `logger.Get().Add("COMPRESS", "INFO", fmt.Sprintf("orig=%d comp=%d saved=%d", ...))` — keeps the change small in v1; defer per-request metadata wiring to v2.
7. **Compile & smoke-test:** start the server, POST a small request, observe no behavior change with compression off; flip toggle, POST a request containing a fake `git diff` block, verify reduced provider-side body size in logs.

## Todo

- [ ] Confirm `RequestLog.Metadata` plumbing or add it.
- [ ] Implement `compressor.NewWithLoader` with atomic options refresh (≤1s debounce).
- [ ] Modify `chatHandler` to accept compressor + call `Compress`.
- [ ] Modify `messagesHandler` to accept compressor + call `Compress` post-translation.
- [ ] Update `NewRouter` to build & inject compressor.
- [ ] Add savings logging path (either log line or metadata).
- [ ] `go build ./...` clean.
- [ ] Manual smoke test with `curl` + crafted fixture.

## Success Criteria

- Disabled state: a request body of 50KB returns to provider as 50KB. No log entry mentions compression.
- Enabled state: same body containing recognisable `git diff` output is reduced ≥30% over the wire.
- Toggling the setting via `PUT /api/settings` takes effect within 1s for subsequent requests.
- Provider responses behave identically (no upstream errors triggered by compression).

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Compressor breaks JSON shape, provider returns 400 | Phase 1 fail-open guarantee + phase 5 integration test that round-trips through `chatService` against a stub provider. |
| `store.GetSettings()` called per request adds latency | Atomic pointer + 1s debounce in `NewWithLoader`. |
| Compressed body size tracked wrongly in SQLite | Either record original *and* compressed in metadata, or document `BodySize` as the post-compression bytes (provider-side wire size). Pick one and document. **Recommendation:** `BodySize` = post-compression; `metadata.compression.orig` = pre-compression. |
| `messages-handler` translation already mutates body shape | Compress *after* translation — phase-1 detector works on text content regardless of OpenAI vs Anthropic envelope. |

## Security Considerations

- Compression must not leak content into logs. Stats only — no message text in any log line.
- The settings loader runs on the request goroutine — `store.GetSettings()` must be safe for concurrent reads (it already is — see existing `apiKeyMiddleware`).

## Next Steps

- Phase 4 builds the React toggle.
- Phase 5 adds the integration test that exercises this entire wiring against fixtures.

## Unresolved Questions

1. Where to attach savings stats — handler-side log line vs end-of-request `MetadataJSON`? **Recommendation:** v1 = log line (simpler), v2 = full metadata once we standardise reqlog metadata API.
2. Should we compress the `system` field separately (currently outside `messages`)? **Out of scope** — see phase 1 question.
