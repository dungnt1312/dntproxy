# Phase 2 — Settings Integration

**Status:** pending
**Priority:** P1 (blocks phase 3 wiring)
**Estimated:** 0.5 day

## Context Links

- Domain config: `internal/domain/config.go`
- Settings handler: `internal/adapter/http/settings-handler.go`
- UI mapper: `ui/src/lib/go-api.ts` (lines ~205–352)

## Overview

Add `CompressionSettings` to the domain `Settings` struct, expose it through the existing `GET /api/settings` and `PUT /api/settings` handlers, and ensure backward compatibility for existing `db.json` files (no migration script needed — Go's `json.Unmarshal` zero-fills).

## Key Insights

- `Settings` is persisted as part of `AppConfig` in `db.json`. Existing files lack `compression` — `omitempty` and Go zero-value semantics give us free defaults.
- `apiUpdateSettings` does **field-by-field** copy (not full overwrite) — we must explicitly carry the new field through, otherwise PUT requests will silently reset it.
- The UI `mapSettings`/`updateSettings` translates between Go's snake-ish JSON and React camelCase. We extend both.

## Requirements

### Functional
- `domain.Settings.Compression` is `CompressionSettings{Enabled, MinContentLength, LogSavings}`.
- Defaults when field is absent or zero-valued: `Enabled=false`, `MinContentLength=500`, `LogSavings=true`.
- `GET /api/settings` returns the compression block.
- `PUT /api/settings` persists changes, returns the updated settings.
- `DefaultConfig()` populates a sane compression default block.

### Non-Functional
- No DB migration script — JSON unmarshal handles missing fields.
- Zero behavior change for users who never enable compression.

## Architecture

```
domain.Settings (extended)
  └── Compression: CompressionSettings { Enabled, MinContentLength, LogSavings }

storage.JsonDB → unchanged (persists whole AppConfig)

http.apiUpdateSettings → adds explicit copy of req.Compression
http.apiGetSettings    → unchanged (returns whole struct)
```

## Related Code Files

### Files to modify
- `internal/domain/config.go` — add `CompressionSettings` type, field on `Settings`, default in `DefaultConfig()`.
- `internal/adapter/http/settings-handler.go` — propagate `req.Compression` in `apiUpdateSettings`.
- `ui/src/lib/go-api.ts` — extend `mapSettings` + `updateSettings` payload to round-trip `compression.*`.

### Files to create
- None.

## Implementation Steps

### Step 1 — Extend domain (`internal/domain/config.go`)

```go
// CompressionSettings controls request body compression middleware.
type CompressionSettings struct {
    Enabled          bool `json:"enabled"`
    MinContentLength int  `json:"minContentLength,omitempty"` // default 500
    LogSavings       bool `json:"logSavings"`
}

type Settings struct {
    // ... existing fields ...
    Compression CompressionSettings `json:"compression,omitempty"`
}

// In DefaultConfig():
Settings: Settings{
    // ...
    Compression: CompressionSettings{
        Enabled:          false,
        MinContentLength: 500,
        LogSavings:       true,
    },
},
```

Add a tiny normaliser to handle legacy/zero values when read from disk:

```go
// NormalizeCompression sets safe defaults for zero values loaded from JSON.
func (c *CompressionSettings) Normalize() {
    if c.MinContentLength <= 0 {
        c.MinContentLength = 500
    }
}
```

Call from `storage.JsonDB.Load()` *or* lazily inside the compressor `New` — preferred lazy (no storage edit needed for v1).

### Step 2 — Settings handler (`settings-handler.go`)

Append to `apiUpdateSettings`:

```go
cfg.Settings.Compression = req.Compression
cfg.Settings.Compression.Normalize()
```

(after the existing `RequireAPIKey`/`ComboStrategies` writes, before `updated = cfg.Settings`).

### Step 3 — UI mapper (`ui/src/lib/go-api.ts`)

In `mapSettings` (around line 205) add:

```ts
compressionEnabled: Boolean(go?.compression?.enabled),
compressionMinLength: Number(go?.compression?.minContentLength ?? 500),
compressionLogSavings: Boolean(go?.compression?.logSavings ?? true),
```

In `updateSettings` (around line 341) extend `payload`:

```ts
compression: {
  enabled: Boolean(data.compressionEnabled),
  minContentLength: Number(data.compressionMinLength ?? 500),
  logSavings: Boolean(data.compressionLogSavings ?? true),
},
```

(Phase 4 wires the React inputs that supply these fields.)

## Todo

- [ ] Add `CompressionSettings` struct to `internal/domain/config.go`.
- [ ] Add `Compression` field to `Settings`.
- [ ] Update `DefaultConfig()` with default compression block.
- [ ] Add `Normalize()` helper.
- [ ] Update `apiUpdateSettings` to copy compression.
- [ ] Extend `mapSettings` in `go-api.ts`.
- [ ] Extend `updateSettings` payload in `go-api.ts`.
- [ ] `go build ./...` clean.
- [ ] `cd ui && npm run build` clean (or `tsc --noEmit`).

## Success Criteria

- `GET /api/settings` returns a `compression` object even on a brand-new database.
- `PUT /api/settings` with `{"compression":{"enabled":true}}` persists across restart.
- Old `db.json` without `compression` field still loads — fields default to zero/false then `Normalize()` fills `MinContentLength=500`.
- React layer can read/write all three fields without a runtime cast error.

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| `apiUpdateSettings` partial-copy pattern silently drops field | Add explicit assignment + add an integration test in phase 5. |
| Old `db.json` deserializes `MinContentLength=0` and breaks compressor | Lazy `Normalize()` inside `compressor.New` and inside `Settings` Load path — defense in depth. |
| UI sends `null`/`undefined` and the JSON reflectively becomes empty struct | Default-fill via `??` in `updateSettings`. |

## Security Considerations

None new — settings are admin-only (already protected by existing `apiKeyMiddleware`).

## Next Steps

- Phase 3 reads these settings to construct the `compressor.Compressor` once at router startup.
- Phase 4 surfaces the toggle in the React Settings screen.

## Unresolved Questions

1. Should `MinContentLength` be exposed in the UI or hidden? **Recommendation:** Show it as advanced/collapsed setting; hard-code 500 in UI v1, expose later.
2. Should we add a `Strategies map[string]bool` per-content-type (e.g., disable JSON compression only)? **Recommendation:** No — YAGNI. Add when a real user asks.
