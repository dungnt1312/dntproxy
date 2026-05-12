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

## Phase 3: CLI and UI (✅ Completed)
- [x] CLI commands for `auth`, `combo`, `alias`, `key`.
- [x] React UI for configuration and management.

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
- [x] Cloudflare tunnel integration (auto-download, lifecycle management, CLI + API).
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
- [ ] **Graceful Shutdown**: Listen for SIGTERM / SIGINT and handle open persistent SSE connections gracefully.
- [ ] **Cross-Platform Delivery**: Enhance build configuration (Makefiles, CI, Goreleaser).
- [ ] **Dockerization**: Provide an official `Dockerfile` and `docker-compose.yml` pattern.

## Future Considerations
- [ ] Request rate limiting and throttling
- [ ] Team/multi-user mode with role-based access
- [ ] Advanced analytics dashboard (usage trends, cost forecasting)
- [ ] Webhook integrations for monitoring/alerting
- [ ] OpenTelemetry tracing for distributed tracing compatibility
