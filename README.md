# dntproxy

`dntproxy` is a Go-based OpenAI-compatible proxy that routes requests to multiple AI providers (Kiro, OpenAI, GLM, MiniMax, Qwen, Anthropic).

## Documentation
- [Project Overview & PDR](docs/project-overview-pdr.md)
- [Codebase Summary](docs/codebase-summary.md)
- [Code Standards](docs/code-standards.md)
- [System Architecture](docs/system-architecture.md)
- [Project Roadmap](docs/project-roadmap.md)

## Supported Providers

| Provider | Auth Methods | Description |
|----------|--------------|-------------|
| **Kiro** (AWS CodeWhisperer) | OAuth (4 methods) | AWS Builder ID, IDC Enterprise, Social, Import |
| **OpenAI** | API Key, OAuth | GPT-4.1, GPT-4o, o3, o4 models |
| **OpenAI Compatible** | API Key | Any OpenAI-compatible API |
| **GLM** (Zhipu AI) | API Key | GLM-4.6, GLM-5, GLM-5.1 |
| **MiniMax** | API Key | MiniMax M2, M2.1, M2.5, M2.7 |
| **Qwen** (Alibaba) | API Key, OAuth | Qwen Coder, Coder-Plus, Plus, Turbo |
| **Anthropic** | API Key | Claude models (adapter TODO) |

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
curl http://127.0.0.1:20199/health
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
- **Multi-Provider Support**: Kiro, OpenAI, GLM, MiniMax, Qwen, Anthropic with extensible architecture.
- **Multi-Account Fallback**: Exponential backoff with automatic degradation on rate limits.
- **Combo Strategies**: `fallback` and `round-robin` model rotation across providers.
- **Request Translation**: OpenAI `/v1/chat/completions` to provider-specific protocols (EventStream, Chat API).
- **CLI & UI Management**: Full configuration via CLI commands and React web UI.
- **Cloudflare Tunnel**: One-command public URL exposure with auto-downloaded cloudflared binary.
- **Structured Logging**: SQLite request logs with 30-day retention, usage tracking, and cost estimation.
- **Model Registry**: 30+ pre-configured models with context windows and pricing data.
- **Local Storage**: Zero-infra JSON + SQLite storage with file locking for safety.

## Architecture
- `internal/domain`: core business types.
- `internal/port`: service interfaces.
- `internal/service`: orchestration logic.
- `internal/adapter`: HTTP, auth, provider executors, tunnel, and storage adapters.
- `internal/logger`: structured logging with ring buffer + SQLite persistence.
