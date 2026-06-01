# Grok Management Design

## Goal

Complete Grok/xAI management support after OAuth succeeds: connection test should hit the correct xAI Responses endpoint, model fetch should expose the useful static Grok model set, and usage/quota screens should give accurate behavior rather than implying unsupported live quota is a bug.

## Scope

- Fix xAI connection testing so it uses `https://api.x.ai/v1/responses` with the existing OAuth token refresh path.
- Expand static xAI model definitions to match the known Grok Build model set used by CLIProxyAPI.
- Keep xAI model fetching static. No live xAI model endpoint is assumed.
- Improve usage behavior for xAI by returning local log-based usage summaries when available.
- Keep quota unsupported for xAI, but return a clear message that xAI does not expose live quota through this OAuth flow.

## Design

### Connection Test

The generic API test path currently strips `/v1` from provider base URLs before appending the chat path. This breaks xAI because the Responses endpoint is under `/v1/responses`.

Add an xAI-specific branch in the connection test path that sends a minimal Responses API request to `baseURL + "/responses"` without stripping `/v1`. The branch should use the same refreshed access token used by other OAuth tests and parse any non-2xx response into the existing test status/error model.

### Models

dntproxy should treat xAI models as static registry data, matching CLIProxyAPI's approach. Add these xAI model IDs to the registry and provider defaults:

- `grok-build-0.1`
- `grok-4.3`
- `grok-4.20-0309-reasoning`
- `grok-4.20-0309-non-reasoning`
- `grok-4.20-multi-agent-0309`
- `grok-3-mini`
- `grok-3-mini-fast`

Media models are intentionally excluded from chat routing for now because dntproxy's current executor is text/chat oriented.

### Usage And Quota

There is no known xAI live quota endpoint for the Grok Build OAuth flow. Usage should be local accounting based on request logs, not a live account quota query.

For `GET /api/usage/:connectionId`, add xAI to the local-log usage path if the existing log store exposes token aggregates. If no aggregate exists, return an empty usage response with a provider-specific message explaining that usage appears after successful requests.

For `POST /api/connections/:id/check-quota`, keep `hasData=false` but return a clearer xAI message.

## Tests

- Add/adjust tests proving xAI connection test URL keeps `/v1`.
- Add model registry tests for new Grok model IDs.
- Add usage/quota tests for the xAI unsupported-live-quota behavior where practical.

## Non-Goals

- Do not implement live xAI quota fetching.
- Do not add image/video Grok models to chat routing.
- Do not redesign provider management UI beyond surfacing the backend behavior correctly.
