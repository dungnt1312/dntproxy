# dntproxy

`dntproxy` is a high-performance Go-based OpenAI-compatible proxy, designed to route incoming requests to Kiro (AWS CodeWhisperer). It supports complex multi-account fallback flows, rotation strategies, and works flawlessly across various tools and editors using standard OpenAI-like HTTP interfaces.

## 📚 Documentation
Comprehensive documentation can be found in the `docs/` directory:

- [Project Overview & PDR](docs/project-overview-pdr.md)
- [Codebase Summary](docs/codebase-summary.md)
- [Code Standards](docs/code-standards.md)
- [System Architecture](docs/system-architecture.md)
- [Project Roadmap](docs/project-roadmap.md)

## 🚀 Quick Start
### Prerequisites
- Go 1.25+
- Bun (for managing the UI component)

### Running Locally
To launch the proxy backend along with the associated configuration interface on its default ports:
```bash
go build -o dntproxy ./cmd/dntproxy/
./dntproxy serve
```

## 🛠 Features
- Multi-Account fallback management with automated backoff strategies.
- `fallback` and `round-robin` Combo capabilities.
- Translates core OpenAI `/v1/chat/completions` directly into Kiro AWS EventStreams natively.
- Full UI configuration and setup capabilities.
- 100% serverless data footprint utilizing lockable JSON disk files (`db.json`).

## 🧱 Architecture
The system employs Clean Architecture, split specifically between pure definitions and system infrastructure:
- `domain/`: Business logic representations and states without HTTP or DB bindings.
- `adapter/`: External HTTP implementations (`kiro` connector, Gin router).
- `service/`: Request processing orchestration logic.
- `port/`: Dependency definitions and interface rules.
