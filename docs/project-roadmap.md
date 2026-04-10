# Project Roadmap

This roadmap captures the progress and future plans for `dntproxy`. 

## Phase 1: Core Proxy Engine (✅ Completed)
- [x] Configure Gin framework standard routing.
- [x] Build AWS Kiro Executor & Custom Binary EventStream Parser.
- [x] Establish OpenAI ↔ Kiro translation models.
- [x] Implement JSON file DB system with `flock`.
- [x] Add combo feature for round-robin & automatic fallback support.
- [x] Model aliases layer.

## Phase 2: Authentication Flows (✅ Completed)
- [x] AWS Builder ID OAuth device code flow.
- [x] AWS IAM Identity Center (IDC).
- [x] GitHub & Google Social OAuth Integration mapping (PKCE flows).
- [x] Provide import mechanism for raw manual tokens.
- [x] Auto-refresh handler to manage decaying access tokens.

## Phase 3: Developer Utilities, UI & Admin CLI (✅ Completed)
- [x] Comprehensive Cobra-based CLI Commands for interactive proxy manipulation (`auth`, `combo`, `alias`, `key`).
- [x] Standup React User Interface to configure the application states via a simple Web GUI (`ui/`).

## Phase 4: Polish & Telemetry (🚧 In Progress)
- [x] **Request Logging**: Persist structured HTTP/provider logs to local SQLite with 30-day retention and connection-level filtering.
- [x] **Logs UX**: Render logs as a request timeline with connection filters, usage/cost badges, and bounded response payload previews.
- [ ] **Graceful Shutdown**: Listen for SIGTERM / SIGINT and handle open persistent SSE connections gracefully.
- [ ] **Cross-Platform Delivery**: Enhance build configuration using Makefiles.
- [ ] **Dockerization**: Provide an official `Dockerfile` and `docker-compose.yml` pattern.
- [x] **Metrics Validation**: Capture provider/SSE usage tokens and estimate cost from configurable local model price profiles.
