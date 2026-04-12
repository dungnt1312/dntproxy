# Codebase Summary

This document summarizes the current `dntproxy` codebase.

## Overall Scale
- Backend (Go): ~5500 LOC
- Frontend (React): ~1900 LOC
- Core files: ~80 (excluding dependencies and generated files)

## Directory Structure

### `cmd/`
Cobra entrypoint and CLI commands.
- `cmd/dntproxy/main.go`: app bootstrap, provider registration, server startup, graceful shutdown.
- `cmd-auth.go`, `cmd-alias.go`, `cmd-combo.go`, `cmd-key.go`, `cmd-tunnel.go`: CLI command handlers.

### `internal/`
Core application layers (Clean Architecture).
- `domain/`: business entities and core models (provider configs, model definitions with pricing).
- `port/`: interfaces for dependency inversion (ProviderExecutor, ModelFetcher, QuotaChecker, TunnelManager, etc.).
- `service/`: orchestration logic (chat, model resolver, account selector, combo handler, token refresh, tunnel).
- `adapter/`: external integrations:
  - `http/`: Gin router + split handlers (chat, connections, combos, aliases, keys, settings, models, backup, usage, logs, tunnel, quota).
  - `kiro/`: Kiro executor + AWS EventStream binary parser.
  - `openai/`: OpenAI executor + Codex translator + stream helpers.
  - `auth/`: OAuth flows (Builder ID, IDC, Social, Import, token refresh).
  - `tunnel/`: Cloudflared download, lifecycle management, state persistence.
  - `custom/`: NoOpModelFetcher, NoOpQuotaChecker for providers without APIs.
  - `shared/`: StreamingHTTPClient, body sanitization, credentials conversion utilities.
  - `storage/`: JSON file DB + SQLite log store with schema and pricing data.
  - `provider/`: Thread-safe provider registry.
- `logger/`: Structured logging system (ring buffer + SQLite persistence, SSE streaming).

### `ui/`
Web admin UI (React, Vite, TypeScript).
- Uses `bun` as the preferred package manager.
- Features tools for managing connections, logs, settings, aliases, and tunnel visually.
- Logs dashboard: local SQLite database (`logs.db`) for 30-day structured request/provider history, connection filters, usage tokens, estimated cost summaries, and bounded response payload previews.
- Tunnel management: enable/disable/status UI.
- Styled using Tailwind CSS.

## Root Files
- `CLAUDE.md`: project operating instructions.
- `AGENTS.md`: detailed architecture and implementation rules.
- `README.md`: user-facing install and quick-start docs.
- `dev.sh`: local development helper.
- `install.sh`: Linux/macOS installer for release binaries.
- `install.ps1`: Windows installer for release binaries.
- `config.example.json`: example configuration file.

## Tooling Used
- Go 1.25+
- Gin (HTTP)
- Cobra (CLI)
- gofrs/flock (file locking)
- SQLite (log persistence)
- cloudflared (tunneling, auto-downloaded)
- React + Vite + TypeScript (UI)
- Bun (UI package manager)
- Tailwind CSS (UI styling)
