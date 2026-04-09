# Codebase Summary

This document provides a summary of the `dntproxy` codebase structure, purpose, and scale.

## Overall Scale
- **Backend (Go)**: ~4500 Lines of Code (LOC)
- **Frontend (React)**: ~1900 Lines of Code (LOC)
- **Total Files**: ~60 core files (excluding tests & dependencies)

## Directory Structure
The repository is split between a Go backend and a modern web frontend.

### `cmd/`
Entry point for the application, built on top of `cobra`.
- `cmd/dntproxy/main.go`: Application initialization, flag parsing, and server startup.
- `cmd-auth.go`, `cmd-alias.go`, `cmd-combo.go`, `cmd-key.go`: Dedicated CLI subcommands and logic.

### `internal/`
Core application logic, following Clean Architecture principles.
- **`domain/`**: Core data models without external dependencies (`config.go`, `fallback.go`, `model.go`, `provider.go`). Defines how the application understands states.
- **`port/`**: Go Interface definitions facilitating dependency inversion (`CredentialStore`, `ProviderExecutor`, `TokenRefresher`).
- **`adapter/`**: Implementations to interact with the outside world.
  - `http/`: Gin web server containing standard HTTP routes and streaming endpoints.
  - `kiro/`: Request translating layer specific to Kiro logic logic and AWS EventStream parser.
  - `auth/`: Implementations of different OAuth and token acquisition flows.
  - `storage/`: Custom, lockable JSON DB strategy storing application preferences securely on-disk.
- **`service/`**: Application business logic orchestrating operations across multiple domain objects.
  - `chat-service.go`: Main flow logic for fetching credentials and routing a request.
  - `account-selector.go`: Fallback and round-robin core intelligence logic.
  - `model-resolver.go`: Mapping `combo` and `alias` representations to real resources.

### `ui/`
Web User Interface.
- Built using React, Vite, and TypeScript.
- Uses `bun` as the preferred package manager.
- Features tools for managing connections, logs, settings, and aliases visually.
- Styled using Tailwind CSS (configuration assumed by environment).

### Root Files
- `CLAUDE.md`: Active agent instructions and system architecture quick references.
- `dev.sh` & `install.sh`: Convenience utilities for setup and active development.

## Tooling Used
- **Go**: 1.25+ module
- **Gin Web Framework**: High speed HTTP routing
- **Cobra**: Developer friendly CLI commands
- **Vite/React**: SPA tooling for dynamic UIs.
