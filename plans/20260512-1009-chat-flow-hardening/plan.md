# Chat Flow Hardening Plan

## Status
- Phase 01: Complete - Policy semantics
- Phase 02: Complete - Account selection and fallback classification
- Phase 03: Complete - Stream correctness and request limits
- Phase 04: Complete - Tests, build, docs

## Context
- Backend chat path: `/v1/chat/completions`, `/v1/messages`
- Main files:
  - `internal/service/chat-service.go`
  - `internal/service/account-selector.go`
  - `internal/service/combo-handler.go`
  - `internal/service/chat-service-error-routing.go`
  - `internal/adapter/http/chat-handler.go`
  - `internal/adapter/http/messages-handler.go`
  - `internal/logger/reqlog.go`

## Goal
Fix policy bypass and incorrect fallback behavior in the chat runtime without changing provider executor contracts unless necessary.

## Key Decisions
- API key model allowlist must be enforced per actual model attempted, not only per original request string.
- Internal policy denials must be terminal. Upstream provider 403 may remain fallback-able.
- Connection allowlist filtering must distinguish "not permitted by policy" from "model unsupported".
- Stream read errors must not be logged as successful completions.

## Dependencies
- Existing `port.APIKeyPolicy`
- Existing combo strategy behavior
- Existing provider executor interface returning OpenAI-compatible SSE

## Success Criteria
- [x] Restricted API keys cannot reach disallowed combo members.
- [x] Allowed later combo members are still reachable when earlier members are not permitted by key connection policy.
- [x] `/v1/messages` oversized body returns 413, not truncated 400.
- [x] Scanner/read errors in Anthropic compatibility path return/log as failures where still possible.
- [x] `go test ./internal/service ./internal/adapter/http ./internal/logger` passes.
- [x] `go build -o $env:TEMP\dntproxy-review.exe ./cmd/dntproxy` passes.
- [x] `go test ./...` status documented: still fails in pre-existing `internal/adapter/storage` summary mismatch.
