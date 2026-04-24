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

## Quick Start

### 1. Download Binary
Download the latest release from [Releases](https://github.com/dungnt1312/dntproxy/releases):
- **Windows**: `dntproxy.exe`
- **Linux/macOS**: Build from source (see Development section)

The binary includes the web UI — no extra files needed. Just download and run.

### 2. Run the Server
```bash
# Windows
dntproxy.exe

# Linux/macOS
./dntproxy
```

The server starts on `http://127.0.0.1:20199` by default.

### 3. Configure Providers
Open the web UI at `http://127.0.0.1:20199/dashboard` to manage connections, combos, aliases, and settings.

### 4. Make Requests
Use the OpenAI-compatible endpoint:

```bash
curl http://127.0.0.1:20199/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "oai/gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### 5. Health Check
```bash
curl http://127.0.0.1:20199/health
```

## CLI Helper Commands
The CLI provides optional helper commands for configuration:

```bash
# Add authentication (interactive)
dntproxy auth add

# List connections
dntproxy auth list

# Create model combo
dntproxy combo add my-combo kr/claude-sonnet-4.5 oai/gpt-4o

# Set model alias
dntproxy alias set sonnet kr/claude-sonnet-4.5

# Start server with custom port
dntproxy serve --port 8080
```

## Development

### Prerequisites
- Go 1.25+
- Bun (for UI development)

### Build from Source (with embedded UI)
```bash
git clone https://github.com/dungnt1312/dntproxy.git
cd dntproxy

# Build UI first
cd ui && bun install && bun run build && cd ..

# Build binary (UI is embedded automatically)
go build -o dntproxy ./cmd/dntproxy/
./dntproxy
```

### Run UI Development Server
```bash
cd ui
bun install
bun run dev
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
