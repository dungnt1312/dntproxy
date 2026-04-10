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
React + Vite + TypeScript admin UI for proxy configuration.

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
