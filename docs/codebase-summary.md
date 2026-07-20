# Codebase Summary

This document summarizes the current `dntproxy` codebase.

## Overall Scale
- Backend (Go): ~28,000 source lines across 163 non-test files, plus ~5,500 test lines
- Frontend (React): ~26,400 source lines across 183 TypeScript/TSX files
- Core files: 163 non-test Go files, 183 TypeScript/TSX files

## Directory Structure

### `cmd/`
Thin ops CLI (Cobra) + server bootstrap.
- `cmd/dntproxy/main.go`: app bootstrap, provider registration, server startup, graceful shutdown; subcommands `serve`, `version`.
- `cmd-update.go`: self-update from GitHub releases.
- Configuration is managed via dashboard UI and `/api/*`, not via management CLI commands.

### `internal/`
Core application layers (Clean Architecture).
- `domain/`: business entities and core models (provider configs, model definitions with pricing).
- `port/`: interfaces for dependency inversion, including separate chat `ProviderExecutor` and image `ImageProvider`/`ImageProviderRegistry` contracts.
- `service/`: orchestration logic (chat, model resolver, model cache, account selector, combo handler, token refresh, tunnel).
- `adapter/`: external integrations:
  - `http/`: Gin router + split handlers (chat, messages, connections, combos, aliases, keys, settings, models, backup, usage, logs, tunnel, quota).
  - `kiro/`: Kiro executor + AWS EventStream binary parser.
  - `openai/`: OpenAI chat/image executor + Codex translator and image stream helpers.
  - `xai/`: xAI chat and image providers.
  - `minimax/`: MiniMax chat plus `image-01` generation/edit translation and business-error mapping.
  - `byteplus/`: ModelArk Seedream image-only provider, translator, client, and response parser.
  - `gemini/`: Gemini native image `generateContent` provider and translator.
  - `anthropic/`: Anthropic Messages API executor with bidirectional translation.
  - `auth/`: OAuth flows (Builder ID, IDC, Social, Import, token refresh).
  - `tunnel/`: Cloudflared download, lifecycle management, state persistence.
  - `custom/`: NoOpModelFetcher, NoOpQuotaChecker for providers without APIs.
  - `shared/`: StreamingHTTPClient, body sanitization, credentials conversion, and bounded SSRF-safe image loading.
  - `storage/`: JSON file DB + SQLite log store with schema and pricing data.
  - `provider/`: Independent thread-safe chat and image provider registries.
- `logger/`: Structured logging system (ring buffer + SQLite persistence, SSE streaming).

### `ui/`
Web admin UI (React, Vite, TypeScript).
- Uses `bun` as the preferred package manager.
- Features tools for managing connections, logs, settings, aliases, combos, and tunnel visually.
- **Modular Architecture**: All major screens refactored into subdirectories (2026-05-08):
  - `playground/`: Chat interface components (ModelSelector, ParameterControls, MessageList, InputArea)
  - `logs/`: Log viewer components (LogTable, LogFilters, LogStats, LogDetails)
  - `profiles/`: Profile management (ProfileList, ProfileForm, ProfileCard)
  - `api-keys/`: API key management, generation, edit dialog, and connection/model permission editor.
  - `dashboard/`: Dashboard widgets (StatsCard, QuickActions, RecentActivity)
  - `connections/`: Connection management (ConnectionCard, ConnectionForm, ConnectionList, QuotaPanel)
  - `layout/`: Shared layout components (Sidebar, Header, Footer)
- **Component Consistency**: All screens now use shadcn/ui components (Button, Switch, RadioGroup, Dialog, etc.)
- **Theme System**: Migrated from hardcoded dark colors to CSS variables for proper theme support
- **API Client**: Unified `goApi` usage across all components
- **File Size**: All screen orchestrators now <400 lines, component modules <200 lines
- **Connections UI**: Collapsible provider groups, grid layout, inline editing modal, quota panel with real-time fetching, logs viewer integration, provider logos.
- **API Keys UI**: Generate/edit dntproxy API keys with unrestricted access or connection/model allowlists.
- **Tunnel UI**: Enable/disable controls, real-time status polling, URL sharing, security warnings.
- **Playground UI**: Enhanced chat interface with model selection, parameter controls, message history.
- **Logs Dashboard**: Local SQLite database (`logs.db`) for 30-day structured request/provider history, connection filters, usage tokens, estimated cost summaries, and bounded response payload previews.
- Styled using Tailwind CSS with shadcn/ui components.

## Root Files
- `CLAUDE.md`: project operating instructions.
- `AGENTS.md`: detailed architecture and implementation rules.
- `README.md`: user-facing install and quick-start docs.
- `dev.sh`: local development helper.
- `install.sh`: Linux/macOS installer for release binaries.
- `install.ps1`: Windows installer for release binaries.
- `config.example.json`: example configuration file.
- `ecosystem.config.js`: PM2 process manager configuration.

## Tooling Used
- Go 1.25+
- Gin (HTTP)
- Cobra (ops CLI: serve / version / update)
- gofrs/flock (file locking)
- SQLite (log persistence)
- cloudflared (tunneling, auto-downloaded)
- React + Vite + TypeScript (UI)
- Bun (UI package manager)
- Tailwind CSS + shadcn/ui (UI styling)

## Key Features

### Multi-Provider Support
- 11 configured providers; chat and image execution capabilities are registered independently
- 30+ pre-configured models with pricing data
- Dynamic model fetching with TTL caching
- Provider-specific quota checking
- `ImageProviderRegistry` dispatches OpenAI/OpenAI-compatible, xAI, MiniMax, BytePlus, and Gemini without provider-name branches in the HTTP handler.
- OpenAI/Codex streaming and multipart edits remain optional adapter capabilities; xAI, MiniMax, BytePlus, and Gemini edits use JSON.
- MiniMax `image-01` supports generation plus one PNG/JPEG character reference up to 7 MiB; masks, multiple references, local paths, and multipart are rejected.
- BytePlus ModelArk uses `/api/v3/images/generations` for Seedream text generation and URL/data-URL reference editing; integrated models advertise 10 or 14 references.
- Gemini image models use native `models/{model}:generateContent`, return `b64_json`, and accept up to 14 PNG/JPEG/WebP references subject to a 7 MiB aggregate proxy envelope.
- Gemini remote references pass through a bounded SSRF-safe loader that blocks non-public destinations, caps redirects/time/bytes, and validates MIME against image content.
- `/v1/models?type=image` returns additive `image_capabilities`; the Image Playground uses it for edit, mask, multi-reference, format, and byte-limit controls.

### Authentication
- 4 OAuth flows: Builder ID, IDC, Social (Google/GitHub), Import
- Auto token refresh with 5-minute buffer
- Secure credential storage with file locking

### Request Routing
- Model aliases for short names
- Combo strategies (fallback, round-robin)
- Multi-account fallback with exponential backoff
- Model-level locks to prevent retry loops
- API key model allowlists are enforced per resolved model attempt; shared combos are filtered to allowed members instead of allowing every combo member by combo name.
- API key connection allowlists distinguish policy-denied connections from unsupported models so combos can skip unavailable members without reaching forbidden pinned connections.

### Observability
- Structured SQLite logging with 30-day retention
- Usage tracking (input/output/total tokens)
- Cost estimation with configurable pricing
- Live SSE log streaming
- Connection-based filtering

### Tunnel Integration
- Cloudflare quick tunnels for public access
- Auto-download cloudflared binary (cross-platform)
- Process lifecycle management
- Persistent state with PID tracking
- Dashboard UI and management API controls

### API Compatibility
- OpenAI Chat Completions (`/v1/chat/completions`)
- OpenAI Image Generations (`/v1/images/generations`) across the registered image providers
- OpenAI Image Edits (`/v1/images/edits`): JSON for every edit-capable provider and multipart where runtime capabilities allow it
- Anthropic Messages API (`/v1/messages`)
- `/v1/messages` enforces the same 10 MB request body limit as chat completions and reports stream read failures without emitting a normal stop event.
- Model listing (`/v1/models`), including capability-filtered `/v1/models?type=image`
- Health check (`/health`)

### UI Components
- Connections management with provider grouping
- API key permission management with connection/model allowlists
- Real-time quota display
- Inline connection editing
- Logs viewer with filtering
- Tunnel management dashboard
- Playground for testing models
