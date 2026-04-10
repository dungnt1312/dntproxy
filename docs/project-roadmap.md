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

## Phase 4: Polish and Delivery (In Progress)
- [ ] Request logging for HTTP audits.
- [ ] Graceful shutdown for SIGTERM/SIGINT and active SSE connections.
- [x] Cross-platform install scripts for release binaries (`install.sh`, `install.ps1`).
- [ ] Cross-platform build packaging automation (CI/Makefile/Goreleaser).
- [ ] Official Docker image and compose setup.
- [ ] Usage metrics validation across the pipeline.
