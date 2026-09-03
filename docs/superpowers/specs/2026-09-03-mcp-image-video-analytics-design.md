# MCP Server — Image/Video Analytics Design

Date: 2026-09-03
Status: Approved (brainstorming session)

## Goal

Expose an MCP (Model Context Protocol) server from dntproxy so MCP clients
(ZCode, Claude Desktop, Cursor, ...) can analyze images and videos through the
proxy's vision-capable providers, reusing the existing multi-account pipeline
(account selection, cooldown/backoff, combos, SQLite logging).

## Decisions (from brainstorming)

| Question | Decision |
|----------|----------|
| Feature shape | MCP server embedded in dntproxy |
| Transport | Streamable HTTP at `POST /mcp` (SDK-managed SSE) |
| Tools | Minimal 2: `analyze_image`, `analyze_video` |
| Video strategy | Server-side frame extraction via ffmpeg (works on every vision model) |
| Model selection | Settings default + per-call `model` param override |
| Architecture | In-process: MCP handler → `port.ChatService` (option A) |

Rejected alternatives: loopback HTTP self-call (extra serialization, internal
key management, no benefit); hand-rolled JSON-RPC (protocol drift risk);
native-video-only providers (limits analyze_video to Qwen/GLM/Gemini).

## Architecture

```
MCP client ──POST /mcp──► mcp SDK (streamable HTTP, auth via API key)
                            │ tools/call: analyze_image | analyze_video
                            ▼
                      mcp executor
                        ├─ image: download (≤20 MB) → inline base64 data URL
                        └─ video: download → ffprobe → ffmpeg N frames → base64
                            ▼
                      build OpenAI chat body (text + image_url parts, stream:true)
                            ▼
                      port.ChatService.HandleChat   ← unchanged pipeline:
                            │                         resolve, accounts, cooldown,
                            ▼                         combos, reqlog, retry
                      aggregate OpenAI SSE → text
                            ▼
                      MCP tool result { content: [{type:"text", text}] }
```

The MCP layer is just another client of `ChatService`, same as
`/v1/chat/completions`. No port interface is added for MCP itself; the ffmpeg
extractor is injected into the mcp package as a consumer-side interface so it
can be faked in tests.

## New Components

| File | Responsibility |
|------|----------------|
| `internal/adapter/mcp/server.go` | Create go-sdk server, register tools, return streamable HTTP handler |
| `internal/adapter/mcp/tools.go` | Tool JSON schemas + arg parsing/validation |
| `internal/adapter/mcp/executor.go` | Tool call orchestration: media load → chat body → HandleChat → aggregate → text |
| `internal/adapter/ffmpeg/ensure.go` | Locate or auto-download ffmpeg/ffprobe (cache `~/.dntproxy/bin/`, same pattern as cloudflared), Windows/macOS/Linux amd64+arm64 |
| `internal/adapter/ffmpeg/extract.go` | Duration probe + N evenly spaced JPEG frames, scaled to ≤1280px wide |
| `internal/adapter/http/mcp-handler.go` | Mount `/mcp` route behind existing auth middleware |

New dependency: `github.com/modelcontextprotocol/go-sdk` (official SDK).

Targeted refactor: move `aggregateChatCompletion` (+ its chunk types) from
`internal/adapter/http/chat-handler.go` to `internal/adapter/shared/` so both
the chat handler and the MCP executor aggregate OpenAI SSE with one
implementation.

## Endpoint & Auth

- Route prefix `/mcp` is added to the API-key auth middleware path check in
  `router.go` (alongside `/api/` and `/v1/`). No CORS requirement for
  non-browser MCP clients, but the existing CORS layer may cover it harmlessly.
- Clients send `Authorization: Bearer <api-key>`. Any valid API key works
  (MCP consumes tokens like `/v1/*`; dashboard access is not required).
- Requests flow through `HandleChat`, so requestID is generated as
  `mcp-<random>` and existing request logging, usage, and cost tracking apply
  unchanged.

## Tool Schemas

```jsonc
// analyze_image
{
  "image_url": "https://... | data:image/png;base64,...", // required
  "prompt": "string",   // optional, default "Describe this image in detail."
  "model": "string"     // optional; model id, alias, or combo name
}

// analyze_video
{
  "video_url": "https://...", // required (http/https)
  "prompt": "string",   // optional, default "Describe this video in detail."
  "model": "string",    // optional
  "max_frames": 8       // optional, 1–16, default from settings
}
```

Result: `{ content: [{ "type": "text", "text": "<analysis>" }] }`. Errors are
returned as MCP tool errors (or `isError: true` results) with a specific,
actionable message.

## Chat Request Building

- Message: single `user` message, content parts ordered `[text(prompt),
  image_url(frame1), ..., image_url(frameN)]`.
- Media is always inlined as base64 data URLs (image download, or extracted
  video frames), so behavior is identical across all provider translators —
  none of them have to fetch remote URLs.
- `stream: true` is always set; the executor aggregates the OpenAI SSE stream
  into the final assistant text via the shared aggregation helper.
- Model resolution: tool `model` param → settings `mcpDefaultModel` → error if
  neither is set. Resolution uses the existing `ModelResolver` semantics
  (model id, alias, combo all accepted).

## Video Pipeline (ffmpeg)

1. Validate scheme `http/https`; download to a temp dir with limits
   **150 MB max size, 2 min timeout**.
2. `ffprobe` duration; extract up to N frames at evenly spaced timestamps
   (`-ss <t> -frames:v 1`), JPEG quality 4, scaled to ≤1280px width
   (`scale='min(1280,iw)':-2`).
3. Frames become base64 `image_url` data URLs.
4. Temp dir removed via defer.

ffmpeg/ffprobe binaries are auto-downloaded on first `analyze_video` call if
missing (cached under `~/.dntproxy/bin/`); if download fails the tool returns
an error telling the user to check network/proxy.

## Settings & UI

- `domain.Settings` additions with backward-compatible `Normalize()`:
  - `mcpDefaultModel string` (json `mcpDefaultModel`)
  - `mcpVideoMaxFrames int` (json `mcpVideoMaxFrames`, zero → 8)
- Settings screen: new **"MCP Analytics"** card with:
  - Default vision model dropdown (models advertising `vision` capability from
    `/api/models`)
  - Max frames input (1–16)
  - Endpoint URL display + copyable client config snippet:

```jsonc
{ "mcpServers": { "dntproxy": {
    "url": "http://<host>:<port>/mcp",
    "headers": { "Authorization": "Bearer <api-key>" } } } }
```

## Error Handling

| Case | Behavior |
|------|----------|
| No default model set and no `model` param | Tool error: "No default vision model configured — set it in Settings → MCP Analytics" |
| Unknown model / no healthy connection | Propagated pipeline errors (existing cooldown/fallback semantics) |
| Video > 150 MB, download timeout, ffmpeg failure | Tool error with the specific reason |
| Image > 20 MB | Tool error |
| ffmpeg binary missing | Auto-download; on failure, actionable error |

## Testing

- Unit: tool schema validation, frame timestamp computation, chat body
  building (content order), model resolution (param vs settings), SSE
  aggregation (shared helper regression).
- Integration: `httptest` MCP handshake (`initialize`, `tools/list`,
  `tools/call`) against a fake `ChatService`.
- ffmpeg extraction: conditional test (skips when binary unavailable), small
  generated fixture video.

## Out of Scope (YAGNI for now)

- stdio transport / `dntproxy mcp` subcommand
- Generic `ask_media` tool, compare/OCR/chart tools
- Native `video_url` passthrough per provider
- Audio analysis
