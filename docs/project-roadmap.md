# Project Roadmap

This roadmap captures the progress and next milestones for `dntproxy`.

## Phase 1: Core Proxy Engine (✅ Completed)
- [x] Configure Gin routing.
- [x] Build Kiro executor and EventStream parser.
- [x] Implement OpenAI to Kiro request/response translation.
- [x] Add JSON DB storage with file locking.
- [x] Add combo handling (fallback and round-robin).
- [x] Add model aliases.

## Phase 2: Authentication Flows (✅ Completed)
- [x] AWS Builder ID device flow.
- [x] AWS IAM Identity Center flow.
- [x] Google/GitHub social login with PKCE.
- [x] Manual token import.
- [x] Auto-refresh access tokens.

## Phase 3: Dashboard and Management API (✅ Completed)
- [x] React UI for configuration and management.
- [x] Management API under `/api/*` as the configuration control plane.
- [x] CLI limited to ops: `serve`, `version`, `update` (management CRUD removed from CLI).

## Phase 4: Multi-Provider Expansion (✅ Completed)
- [x] Provider config registry with 7 providers (Kiro, OpenAI, OpenAI-Compatible, GLM, MiniMax, Qwen, Anthropic).
- [x] Model registry with 30+ models and pricing data.
- [x] ModelFetcher interface for dynamic model list fetching.
- [x] QuotaChecker interface for usage/limit validation.
- [x] NoOp adapters for providers without APIs.

## Phase 5: Telemetry and Infrastructure (✅ Completed)
- [x] Structured request logging to SQLite with 30-day retention.
- [x] Logs UI: timeline view with connection filters, usage/cost badges, response previews.
- [x] Install scripts: Cross-platform release installers (`install.sh`, `install.ps1`).
- [x] Metrics validation: Usage token capture and cost estimation from model price profiles.
- [x] Cloudflare tunnel integration (auto-download, lifecycle management, API + dashboard).
- [x] Tunnel UI: enable/disable/status dashboard.

## Phase 6: Anthropic Integration (✅ Completed)
- [x] Anthropic Messages API executor with bidirectional translation.
- [x] Native `/v1/messages` endpoint support.
- [x] Tool calling and system message handling.
- [x] SSE streaming with proper event conversion.
- [x] Stop reason mapping and content block handling.

## Phase 7: Advanced Features (✅ Completed)
- [x] **Quota Checking System**: Flexible bucket-based quota checking with provider-specific implementations (OpenAI, Kiro, MiniMax).
- [x] **Model Fetching**: Dynamic model discovery with TTL caching and singleflight deduplication.
- [x] **Model Cache**: 5-minute TTL cache to reduce API calls.
- [x] **Enhanced Connections UI**: Collapsible provider groups, inline editing, quota panel, logs viewer, provider logos.

## Phase 8: Polish & Production (🚧 In Progress)
- [x] **Chat Flow Hardening**: Enforce API key model allowlists per combo member, classify connection-policy fallback correctly, and harden Anthropic-compatible stream/body-limit behavior.
- [x] **API Key Permission UI**: Add dashboard create/edit controls for connection and model allowlists on proxy API keys.
- [x] **Models Page Redesign**: Improve registry scanning, alias creation, combo editing, partial-load errors, and remove legacy Models page drift.
- [x] **Connection Execution Strategies**: Add weighted random, primary-first priority fallback, and round-robin account selection for chat execution.
- [x] **MiniMax Image Generation MVP**: Route `minimax/image-01` text-to-image requests from `POST /v1/images/generations` to MiniMax `POST /v1/image_generation`, with URL/Base64 responses, request validation, and business-error handling.
- [x] **MiniMax Image Editing**: Route JSON `POST /v1/images/edits` requests to MiniMax character-reference generation with exactly one PNG/JPEG HTTP(S) URL or Base64 data URL. Reject masks, multiple references, and multipart edits; constrain Playground uploads to one PNG/JPEG file up to 7 MB.
- [x] **Telegram Bot Integration**: Embedded two-way Telegram bot for real-time alerts (quota exhausted, token expired, connection down, rate limited, combo exhausted) and interactive commands (/status, /usage, /connections, /mute, /unmute). Single-owner auth, start/stop from UI, 30-min alert deduplication with auto-recovery notifications.
- [ ] **Graceful Shutdown**: Listen for SIGTERM / SIGINT and handle open persistent SSE connections gracefully.
- [ ] **Cross-Platform Delivery**: Enhance build configuration (Makefiles, CI, Goreleaser).
- [ ] **Dockerization**: Provide an official `Dockerfile` and `docker-compose.yml` pattern.

## Future Considerations
- [ ] Multi-account fallback for image generation requests
- [ ] Request rate limiting and throttling
- [ ] Team/multi-user mode with role-based access
- [ ] Advanced analytics dashboard (usage trends, cost forecasting)
- [ ] OpenTelemetry tracing for distributed tracing compatibility
