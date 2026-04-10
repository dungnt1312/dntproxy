# Codebase Summary

This document summarizes the current `dntproxy` codebase.

## Overall Scale
- Backend (Go): ~4500 LOC
- Frontend (React): ~1900 LOC
- Core files: ~60 (excluding dependencies and generated files)

## Directory Structure

### `cmd/`
Cobra entrypoint and CLI commands.
- `cmd/dntproxy/main.go`: app bootstrap and server startup.
- `cmd-auth.go`, `cmd-alias.go`, `cmd-combo.go`, `cmd-key.go`: CLI command handlers.

### `internal/`
Core application layers (Clean Architecture).
- `domain/`: business entities and core models.
- `port/`: interfaces for dependency inversion.
- `service/`: orchestration logic.
- `adapter/`: external integrations (`http`, `kiro`, `auth`, `storage`).

### `ui/`
Web admin UI (React, Vite, TypeScript).
- Uses `bun` as the preferred package manager.
- Features tools for managing connections, logs, settings, and aliases visually.
- Logs use a local SQLite database (`logs.db`) for 30-day structured request/provider history, connection filters, usage tokens, estimated cost summaries, and bounded response payload previews.
- Styled using Tailwind CSS (configuration assumed by environment).

## Root Files
- `CLAUDE.md`: project operating instructions.
- `README.md`: user-facing install and quick-start docs.
- `dev.sh`: local development helper.
- `install.sh`: Linux/macOS installer for release binaries.
- `install.ps1`: Windows installer for release binaries.

## Tooling Used
- Go 1.25+
- Gin
- Cobra
- React + Vite
- Bun (UI package manager)
