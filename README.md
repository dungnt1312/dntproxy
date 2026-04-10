# dntproxy

`dntproxy` is a Go-based OpenAI-compatible proxy that routes requests to Kiro (AWS CodeWhisperer).

## Documentation
- [Project Overview & PDR](docs/project-overview-pdr.md)
- [Codebase Summary](docs/codebase-summary.md)
- [Code Standards](docs/code-standards.md)
- [System Architecture](docs/system-architecture.md)
- [Project Roadmap](docs/project-roadmap.md)

## Installation (Release Binaries)

### Linux/macOS
Install latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/dungnt/dntproxy/main/install.sh | bash
```

Install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/dungnt/dntproxy/main/install.sh | bash -s -- --version v0.1.0
```

### Windows (PowerShell)
Install latest release:

```powershell
$script = Join-Path $env:TEMP "dntproxy-install.ps1"
Invoke-WebRequest https://raw.githubusercontent.com/dungnt/dntproxy/main/install.ps1 -OutFile $script
& $script
```

Install a specific version:

```powershell
$script = Join-Path $env:TEMP "dntproxy-install.ps1"
Invoke-WebRequest https://raw.githubusercontent.com/dungnt/dntproxy/main/install.ps1 -OutFile $script
& $script -Version v0.1.0
```

### Expected release asset names
- `dntproxy-linux-amd64.tar.gz`
- `dntproxy-linux-arm64.tar.gz`
- `dntproxy-darwin-amd64.tar.gz`
- `dntproxy-darwin-arm64.tar.gz`
- `dntproxy-windows-amd64.zip`
- `dntproxy-windows-arm64.zip`

## Quick Start
After install:

```bash
dntproxy auth add
dntproxy serve
```

Health check:

```bash
curl http://127.0.0.1:20128/health
```

## Local Development
Prerequisites:
- Go 1.25+
- Bun (for UI)

Run locally:

```bash
go build -o dntproxy ./cmd/dntproxy/
./dntproxy serve
```

## Features
- Multi-account fallback with exponential backoff.
- `fallback` and `round-robin` combo strategies.
- OpenAI `/v1/chat/completions` to Kiro EventStream translation.
- CLI and UI for settings management.
- Local on-disk JSON storage (`db.json`) with file locking.

## Architecture
- `internal/domain`: core business types.
- `internal/port`: service interfaces.
- `internal/service`: orchestration logic.
- `internal/adapter`: HTTP, auth, Kiro, and storage adapters.
