# Project Changelog

## 2026-07-09

- P0 multi-account reliability: unified upstream failure taxonomy (`IsRetryableUpstream`) so account fallback and combo routing agree (e.g. “improperly formed request” is non-fallback).
- Per-model credential retry budget (`maxRetryCredentials`): counts distinct connections only; OAuth re-exec on the same connection is not double-counted; `0` keeps unlimited legacy behavior.
- Configurable cooldowns (`cooldownEnabled`, `transientCooldownSeconds`, `maxCooldownSeconds`) plus per-connection `providerSpecificData.disableCooling` to skip `RateLimitedUntil`.
- New `connectionStrategy=fill-first`: always pick the lowest Priority available connection (Settings UI dropdown option added).
- Optional session affinity (`sessionAffinityEnabled`, `sessionAffinityTTLSeconds`) with locked keys (`h:` from `X-Session-Id` / `X-Dntproxy-Session`, or `k:apiKey|provider|model`); hard `@connectionId` pin still wins.
- Defaults remain backward compatible when new settings are omitted/zero.

## 2026-05-13

- Added connection execution strategy setting: weighted random, primary-first priority fallback, and round-robin account rotation.
- Added connection `priority` field and editable priority/weight controls in the connection edit dialog.
- Removed the unused sticky round-robin UI setting and wired Settings to persist `connectionStrategy`.
- Added route tests for priority fallback and round-robin connection selection.

## 2026-05-12

- Redesigned Models page registry into a searchable table/mobile list with clearer provider, model ID, connection test, status, and log actions.
- Reworked alias creation into a dialog with registry model picker and validation.
- Reworked combo creation/editing into a clearer step builder with visible reorder/delete controls and pinned-account display.
- Added partial-load error handling for routing data so API failures no longer look like empty model, alias, or combo states.
- Removed unused legacy `ui/src/pages/models.tsx` implementation.
- Added dashboard API key permission management for connection and model allowlists.
- Added API client support for `allowedConnectionIds`, `allowedModels`, and key updates.
- Added backend API key handler validation/deduplication for allowed connection IDs.
- Added handler tests for API key permission create/update validation.
- Fixed `/v1/models` so API key connection/model allowlists also filter direct models, aliases, and combos.
