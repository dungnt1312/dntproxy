# Project Overview & PDR (Product Development Requirements)

## Project Overview
`dntproxy` is an open-source, high-performance Go application that serves as an OpenAI-compatible proxy, routing requests to multiple AI providers (Kiro, OpenAI, OpenAI-Compatible, GLM, MiniMax, Qwen, Anthropic). It acts as a bridge, allowing developers to use any supported LLM using familiar OpenAI tooling and libraries. It includes a robust multi-account fallback system, combo model chains, Cloudflare tunneling, structured logging with usage tracking, quota checking, dynamic model fetching, and a dashboard UI plus management API to configure multiple provider keys seamlessly.

## Business Goals & Requirements
- **Multi-Provider Proxy Capabilities**: Translate standard OpenAI-compatible `/v1/chat/completions` API calls into provider-specific protocols (AWS EventStream for Kiro, standard Chat API for OpenAI/GLM/MiniMax/Qwen, Anthropic Messages API for Claude models).
- **Resilience**: Implement multi-account fallback to naturally switch keys dynamically when hit with rate limits or authorization errors.
- **Model Flexibility**: Implement Combo Strategies (Fallback & Round-robin) to automatically rotate between models seamlessly across providers.
- **Authentication Integration**: Provide integration for multiple auth methods (OAuth device flows, API keys, social login, manual token importing) via the dashboard UI and management API.
- **Portability**: Support lightweight data storage utilizing a custom local SQLite/JSON DB storage (`db.json`) instead of requiring heavy database infrastructure. Complete cross-platform capabilities (Windows, Linux, macOS).
- **Observability**: Structured request logging with 30-day retention, usage tracking, cost estimation, and live SSE streaming.
- **Public Access**: One-command Cloudflare tunnel exposure for sharing local proxy publicly.
- **Quota Management**: Real-time quota checking with provider-specific implementations and bucket-based results.
- **Model Discovery**: Dynamic model fetching with TTL caching to reduce API calls.

## Target Audience
- Developers trying to utilize multiple AI provider APIs seamlessly with standard desktop applications or agents using the OpenAI API spec.
- Organizations seeking to share LLM usage across multiple accounts and providers with automated fallback redundancy.
- Teams needing a unified interface for experimenting with different AI models without changing tooling.

## Key Features
- Clean architecture with 4 layers (Domain, Port, Adapter, Service) + Logger.
- OpenAI ↔ provider request/response structure translation (7 providers supported).
- Anthropic Messages API bidirectional translation with tool calling support.
- EventStream to SSE binary translation for seamless real-time responses (Kiro).
- Thin ops CLI built with Cobra (`serve`, `version`, `update`); configuration is not managed via CLI.
- Dashboard UI built with React to configure services (React/Vite/TypeScript) plus management API under `/api/*`.
- Cloudflare tunnel integration for public URL exposure with auto-download and lifecycle management.
- Structured SQLite logging with usage tracking and cost estimation.
- Model registry with 30+ pre-configured models and pricing data.
- Dynamic model fetching with TTL caching and singleflight deduplication.
- Flexible quota checking system with provider-specific implementations.
- Enhanced connections UI with collapsible groups, inline editing, quota panel, logs viewer, provider logos.

## Non-Functional Requirements
- **Performance**: High throughput and low latency routing layer leveraging Go's concurrency.
- **Reliability**: Exponential backoff implementations when accounts fail. Atomic writes for internal JSON configurations. Auto-recovery from transient failures.
- **Usability**: Must be easy to start and run via the Go binary (`dntproxy` / `serve`), and easy to configure via the dashboard UI and management API. Easy mapping or aliasing of models.
- **Security**: No credentials logged or stored in plain text. Body sanitization redacts sensitive fields. OAuth PKCE flows for public clients.
- **Observability**: Request metadata, usage tokens, and estimated costs persisted to SQLite with 30-day retention. Live log streaming via SSE.
