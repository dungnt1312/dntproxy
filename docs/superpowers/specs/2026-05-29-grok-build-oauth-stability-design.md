# Grok Build OAuth and Stability Design

## Goal

Add Grok Build OAuth support to dntproxy without turning the project into a broad CLIProxyAPI competitor. The implementation should keep dntproxy stable, easy to operate, and consistent with its existing provider, auth, fallback, and logging patterns.

## Non-Goals

- Do not add Gemini/Claude/Codex/Grok multi-protocol surfaces beyond dntproxy's existing OpenAI-compatible API.
- Do not port CLIProxyAPI's full xAI feature set, including image/video endpoints, websocket execution, generic translator registry, payload rule engine, or SDK abstractions.
- Do not add Grok API-key support in this pass unless it falls out naturally from the OAuth executor design.

## Provider Scope

Add a new Grok/xAI provider backed by Grok Build OAuth tokens.

- Internal provider ID: `xai`.
- Public/model prefix: `grok`.
- Display name: `Grok Build (xAI)`.
- Auth method: `oauth`.
- Default API base URL: `https://api.x.ai/v1`.
- Upstream inference endpoint: `/responses`.
- Client-facing API remains OpenAI-compatible through `/v1/chat/completions` and existing model routing.

Example model IDs:

- `grok/grok-4.3`
- `grok/grok-3-mini`

## OAuth Flow

Use xAI OIDC discovery and PKCE authorization-code flow, following the minimal proven details from CLIProxyAPI.

- Discovery URL: `https://auth.x.ai/.well-known/openid-configuration`.
- OAuth client ID: `b1a00492-073a-47ea-816f-4c329264a828`.
- Scope: `openid profile email offline_access grok-cli:access api:access`.
- Redirect URL: `http://127.0.0.1:56121/callback`.
- Validate discovered authorization and token endpoints so they use HTTPS and their host is `x.ai` or a subdomain of `x.ai`.

Persist connections in the existing JSON DB using `ProviderConnection`:

- `Provider`: `xai`.
- `AuthType`: `oauth`.
- `AccessToken`, `RefreshToken`, `ExpiresIn`, `ExpiresAt`, `Email` from token response and ID token claims.
- `ProviderSpecificData.authMethod`: `xai-oauth`.
- `ProviderSpecificData.tokenEndpoint`: discovered token endpoint.
- `ProviderSpecificData.redirectURI`: redirect URI used during exchange.
- `ProviderSpecificData.idToken`: ID token if returned.
- `ProviderSpecificData.subject`: `sub` claim if present.

Expose management endpoints under the existing auth route style:

- `POST /api/auth/xai/start`
- `POST /api/auth/xai/exchange`

## Token Refresh

Extend `TokenRefreshService` to refresh `xai` OAuth connections using the saved refresh token and token endpoint.

- Refresh when `ExpiresAt` is within the existing token expiry buffer.
- Preserve the old refresh token if the refresh response omits a new one.
- Use singleflight through the existing `CheckAndRefresh` path.
- Persist refreshed connection through the existing store update path.

## Executor

Add `internal/adapter/xai` with an executor that implements `port.ProviderExecutor`.

The executor will translate OpenAI Chat Completions requests to xAI Responses requests and translate xAI Responses results back to OpenAI-compatible streaming output.

Supported request subset:

- `model`
- `messages` with `system`, `developer`, `user`, `assistant`, and `tool` roles where safely representable
- `stream`
- `temperature`
- `top_p`
- `max_tokens` or `max_completion_tokens`
- basic OpenAI function tools

Unsupported or ambiguous tool payloads should return a clear `400` rather than silently changing semantics.

The executor must:

- Use `Authorization: Bearer <access token>`.
- Use `Accept: text/event-stream` for streaming requests.
- Avoid client-level stream timeouts, following existing streaming client patterns.
- Use a large scanner buffer for SSE events.
- Record upstream request/response data via `RequestLogger`, with existing body sanitization.
- Extract usage from completed response events when present.

## Routing and Models

Add the provider to existing routing and model discovery conventions.

- Register the executor in `cmd/dntproxy/main.go`.
- Add provider config in `internal/domain/provider-config.go`.
- Add alias mapping so `grok/<model>` resolves to provider `xai`.
- Add default model definitions for stable Grok model names.
- Ensure `SupportsModel` and pinned account syntax continue to work, e.g. `grok/grok-4.3@conn-id`.

## Stability Enhancements

Implement a minimal upstream error classifier as part of the Grok work, scoped so it improves behavior without broad refactoring.

Classifier categories:

- `auth`: token expired/invalid or authorization failure.
- `rate_limit`: upstream rate limit or quota responses.
- `model_unsupported`: requested model unavailable for the account/provider.
- `transient`: network errors and upstream 5xx/408/429 where retry/fallback is appropriate.
- `client`: bad request that should not be retried.

Initial behavior:

- Grok executor should return errors/status codes that allow existing chat service fallback and cooldown behavior to make sensible decisions.
- Avoid retrying invalid client requests.
- Keep broader circuit breaker and active health probe for a later pass.

## Testing

Add unit tests for:

- xAI endpoint validation.
- xAI authorization URL construction.
- token exchange and refresh parsing using test HTTP servers.
- OpenAI chat to xAI Responses request translation.
- xAI SSE to OpenAI SSE translation.
- upstream error propagation and unsupported tool handling.

Run at minimum:

- `go test ./internal/adapter/auth ./internal/adapter/xai ./internal/service ./internal/adapter/http`
- `go test ./...`
- `go build -o /tmp/dntproxy ./cmd/dntproxy/`

## Deferred Work

- Full circuit breaker per connection/model.
- Active health probes.
- Request-level failover budget.
- xAI image/video endpoints.
- Generic protocol translator registry.
