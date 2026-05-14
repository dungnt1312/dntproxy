# CLI Tools Configuration

## Overview
Add `dntproxy tools` CLI command that auto-configures AI coding tools (Claude Code, Cursor, Windsurf, OpenCode, Cline, Continue, Gemini CLI) to use dntproxy as their backend proxy.

## Status: DONE
- Start: 2026-05-13
- Completed: 2026-05-13
- Phases: 3 (all complete)

## Problem
Users must manually configure each AI tool's config files to point at dntproxy. This is error-prone, varies per tool, and requires knowledge of each tool's config format/location.

## Solution
A `dntproxy tools` command with subcommands:
- `dntproxy tools list` — show supported tools and their detection status
- `dntproxy tools configure <tool>` — auto-configure a tool to use dntproxy
- `dntproxy tools reset <tool>` — revert tool config to direct provider access
- `dntproxy tools status` — show which tools are currently configured

## Phases

### Phase 1: Tool Registry and Domain Types [DONE]
- Define `ToolConfig` struct in domain layer (tool ID, name, config paths per OS, config format)
- Create built-in tool registry with detection logic
- Supported tools: claude-code, cursor, windsurf, opencode, cline, continue, gemini-cli

### Phase 2: Tool Configuration Service [DONE]
- `internal/service/tools-service.go` — detect, configure, reset logic
- Config read/write for each tool format (JSON, YAML, env)
- Backup original config before modification
- Cross-platform path resolution

### Phase 3: CLI Command + Tests [DONE]
- `cmd/dntproxy/cmd-tools.go` — cobra commands
- Register in main.go
- Unit tests for detection and config generation

### Phase 4: API Endpoints [DONE]
- `internal/adapter/http/tools-handler.go` — REST API for tools management
- GET /api/tools, POST /api/tools/:id/configure, POST /api/tools/:id/reset
- Bulk configure/reset endpoints

### Phase 5: Dashboard UI [DONE]
- `ui/src/components/screens/tools-screen.tsx` — React screen
- Registered in App.tsx with route and nav item
- API client methods in go-api.ts

## Tool Configuration Details

| Tool | Config Location | Format | Key Fields |
|------|----------------|--------|------------|
| Claude Code | `~/.claude.json` | JSON | `apiUrl` |
| Cursor | `~/.cursor/config.json` | JSON | `openaiBaseUrl` |
| Windsurf | `~/.codeium/windsurf/config.json` | JSON | `apiBaseUrl` |
| OpenCode | `~/.config/opencode/config.json` | JSON | `provider.baseURL` |
| Cline | VS Code settings | JSON | `cline.apiBaseUrl` |
| Continue | `~/.continue/config.json` | JSON | `models[].apiBase` |
| Gemini CLI | env `GEMINI_API_BASE` | ENV | base URL override |

## Design Decisions
- Backup original config before any modification (`.bak` suffix)
- Use tunnel URL if tunnel is active, otherwise localhost:port
- Include API key in config if `requireApiKey` is enabled
- Tool detection = check if config directory/binary exists
