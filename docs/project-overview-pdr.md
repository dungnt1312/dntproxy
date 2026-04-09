# Project Overview & PDR (Product Development Requirements)

## Project Overview
`dntproxy` is an open-source, high-performance Go application that serves as an OpenAI-compatible proxy, routing requests to Kiro (AWS CodeWhisperer). It acts as a bridge, allowing developers to use Kiro LLMs using familiar OpenAI tooling and libraries. It includes a robust multi-account fallback system, combo model chains, and complete CLI tooling to manage multiple provider keys seamlessly.

## Business Goals & Requirements
- **Proxy Capabilities**: Translate standard OpenAI-compatible `/v1/chat/completions` API calls into provider-specific (Kiro) calls.
- **Resilience**: Implement multi-account fallback to naturally switch keys dynamically when hit with rate limits or authorization errors.
- **Model Flexibility**: Implement Combo Strategies (Fallback & Round-robin) to automatically rotate between models seamlessly.
- **Kiro Authentication Integration**: Provide integration for all 4 Kiro auth methods (AWS Builder ID, AWS Identity Center, Google/GitHub Social, and manual token importing) directly via a CLI.
- **Portability**: Support lightweight data storage utilizing a custom local SQLite/JSON DB storage (`db.json`) instead of requiring heavy database infrastructure. Complete cross-platform capabilities (Windows, Linux, macOS).

## Target Audience
- Developers trying to utilize Kiro APIs seamlessly with standard desktop applications or agents using the OpenAI API spec.
- Organizations seeking to share LLM usage across multiple accounts with automated fallback redundancy.

## Key Features
- Clean architecture with 4 layers (Domain, Port, Adapter, Service).
- OpenAI ↔ Kiro request/response structure translation.
- EventStream to SSE binary translation for seamless real-time responses.
- Complete CLI configuration tooling built with Cobra.
- User Interface built with React to configure services (React/Vite).

## Non-Functional Requirements
- **Performance**: High throughput and low latency routing layer leveraging Go's concurrency.
- **Reliability**: Exponential backoff implementations when accounts fail. Atomic writes for internal JSON configurations.
- **Usability**: Must be easy to start, run, and configure either through the unified Go binary (using `serve`) or using the provided shell commands. Easy mapping or aliasing of models.
