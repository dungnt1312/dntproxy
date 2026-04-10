# Project Roadmap

This roadmap captures the progress and next milestones for `dntproxy`.

## Phase 1: Core Proxy Engine (Completed)
- [x] Configure Gin routing.
- [x] Build Kiro executor and EventStream parser.
- [x] Implement OpenAI to Kiro request/response translation.
- [x] Add JSON DB storage with file locking.
- [x] Add combo handling (fallback and round-robin).
- [x] Add model aliases.

## Phase 2: Authentication Flows (Completed)
- [x] AWS Builder ID device flow.
- [x] AWS IAM Identity Center flow.
- [x] Google/GitHub social login with PKCE.
- [x] Manual token import.
- [x] Auto-refresh access tokens.

## Phase 3: CLI and UI (Completed)
- [x] CLI commands for `auth`, `combo`, `alias`, `key`.
- [x] React UI for configuration and management.

## Phase 4: Polish & Telemetry (🚧 In Progress)
- [x] **Request Logging**: Persist structured HTTP/provider logs to local SQLite with 30-day retention and connection-level filtering.
- [x] **Logs UX**: Render logs as a request timeline with connection filters, usage/cost badges, and bounded response payload previews.
- [ ] **Graceful Shutdown**: Listen for SIGTERM / SIGINT and handle open persistent SSE connections gracefully.
- [x] **Install scripts**: Cross-platform release installers (`install.sh`, `install.ps1`).
- [ ] **Cross-Platform Delivery**: Enhance build configuration (Makefiles, CI, Goreleaser).
- [ ] **Dockerization**: Provide an official `Dockerfile` and `docker-compose.yml` pattern.
- [x] **Metrics Validation**: Capture provider/SSE usage tokens and estimate cost from configurable local model price profiles.
