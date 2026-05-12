# Phase 03 - Stream Correctness And Limits

## Overview
- Priority: High
- Status: Complete
- Purpose: Fix partial success behavior in stream conversions and request limits.

## Key Insights
- `/v1/messages` uses `LimitReader(maxChatBodySize)` without overflow detection.
- Anthropic compatibility stream conversion uses `bufio.Scanner` with 1MB max token.
- Scanner errors are ignored, so truncated streams can still finish as successful responses.
- Chat handler read errors are logged as status 200 because stream wrapper only sees `Close()` error.

## Requirements
- `/v1/messages` should match `/v1/chat/completions` body limit behavior.
- Stream parser must surface scanner/read errors.
- Request logs should reflect stream failure/cancel where available.

## Related Code Files
- Modify: `internal/adapter/http/messages-handler.go`
- Modify: `internal/adapter/http/chat-handler.go`
- Modify: `internal/logger/reqlog.go`
- Optional: `internal/port/request-logger.go` if adding explicit stream failure method

## Implementation Steps
1. Update `/v1/messages` body read to use `maxChatBodySize+1` and return 413 on overflow.
2. Replace scanner-based SSE reads with a reader helper that can handle larger lines or reports `scanner.Err()`.
3. In streaming Anthropic conversion:
   - if headers already sent, emit an Anthropic `error` event before stopping when possible.
   - do not send normal `message_stop` after parser error.
4. In non-streaming Anthropic conversion:
   - if scan/read error occurs before response is written, return 502.
5. Add a way to mark request log stream failure before closing:
   - recommended: extend wrapper with `CloseWithStatus` helper local to handler, or add `RequestLog.FailStream`.
   - keep interface changes minimal.
6. Add tests for oversized body and scanner error paths where practical.

## Todo List
- [x] Add body overflow handling to `/v1/messages`.
- [x] Surface scanner/read errors.
- [x] Prevent false success message stop on stream parse errors.
- [x] Improve request log status for stream errors.

## Risk Assessment
- Once SSE headers are sent, HTTP status cannot change.
- Error event is best effort for streaming clients.

## Security Considerations
- Oversized request handling should be deterministic and bounded.
