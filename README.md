# dntproxy

`dntproxy` is a Go-based OpenAI-compatible proxy that routes chat and image requests to multiple AI providers.

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
| **Anthropic** | API Key | Claude models through the Messages API |
| **ClinePass** | API Key | Subscription models from multiple providers |
| **Google Gemini** | API Key | Chat plus native Gemini image generation/editing |
| **BytePlus ModelArk** | API Key | Seedream image generation and reference editing |
| **xAI** | OAuth | Grok chat and image models |

## Quick Start

### 1. Install

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/dungnt1312/dntproxy/master/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/dungnt1312/dntproxy/master/install.ps1 -OutFile "$env:TEMP\install.ps1"; & "$env:TEMP\install.ps1"
```

**Windows (Bash/Git Bash):**
```bash
curl -fsSL https://raw.githubusercontent.com/dungnt1312/dntproxy/master/install.sh | bash
```

Or download binaries directly from [Releases](https://github.com/dungnt1312/dntproxy/releases).

The binary includes the web UI — no extra files needed. Just download and run.

**Install from local source:**
```bash
git clone https://github.com/dungnt1312/dntproxy.git
cd dntproxy
bash ./install-local.sh
```

This builds the UI and Go binary from the current source tree, then installs `dntproxy` to `~/.local/bin`.

### 2. Run the Server
```bash
dntproxy
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

## CLI (ops only)
Configuration is managed via the **dashboard UI** and **management API**. The CLI is limited to server ops:

```bash
dntproxy                     # Start server (default port 20199)
dntproxy serve --port 8080   # Start with custom port
dntproxy version             # Print version
dntproxy update              # Self-update to latest release
dntproxy update --force      # Force update even if already on latest
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
- **Multi-Provider Support**: 11 configured providers with separate chat and image execution capabilities.
- **Image Provider Registry**: OpenAI/OpenAI-compatible, xAI, MiniMax, BytePlus Seedream, and Gemini image adapters behind one capability-driven interface.
- **OpenAI-Compatible Images**: `/v1/images/generations` and `/v1/images/edits`; discover model-specific edit, mask, reference, format, and size limits with `/v1/models?type=image`.
- **Multi-Account Fallback**: Exponential backoff with automatic degradation on rate limits.
- **Combo Strategies**: `fallback` and `round-robin` model rotation across providers.
- **Request Translation**: OpenAI `/v1/chat/completions` to provider-specific protocols (EventStream, Chat API).
- **Dashboard & Management API**: Full configuration via React web UI and `/api/*` endpoints. CLI is ops-only (`serve`, `version`, `update`).
- **API Key Permissions**: Dashboard can restrict proxy API keys by allowed connections and models.
- **Cloudflare Tunnel**: One-command public URL exposure with auto-downloaded cloudflared binary.
- **Structured Logging**: SQLite request logs with 30-day retention, usage tracking, and cost estimation.
- **Model Registry**: 30+ pre-configured models with context windows and pricing data.
- **Local Storage**: Zero-infra JSON + SQLite storage with file locking for safety.

## Architecture
- `internal/domain`: core business types.
- `internal/port`: service interfaces, including the separate `ImageProvider` and `ImageProviderRegistry` contracts.
- `internal/service`: orchestration logic.
- `internal/adapter`: HTTP, auth, provider executors, tunnel, and storage adapters.
- `internal/logger`: structured logging with ring buffer + SQLite persistence.
