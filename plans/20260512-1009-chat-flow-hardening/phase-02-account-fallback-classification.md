# Phase 02 - Account Selection And Fallback Classification

## Overview
- Priority: Critical
- Status: Complete
- Purpose: Fix incorrect combo stop/fallback behavior caused by policy filtering.

## Key Insights
- `SelectCredentials()` applies `allowedConnectionIDs` before `supportedCount`.
- If no connections survive policy filter, it can return `SelectionErrUnsupportedModel`.
- That maps to 400, and combo stops instead of trying a later model.
- Internal policy 403 currently looks like provider 403 and may fallback.

## Requirements
- Add distinct error classification for policy-denied/no-allowed-connection cases.
- Internal policy denials should be terminal when the requested model is explicitly forbidden.
- A combo member with no allowed connection should be skippable when other combo members remain.

## Related Code Files
- Modify: `internal/service/account-selector.go`
- Modify: `internal/service/chat-service.go`
- Modify: `internal/service/chat-service-error-routing.go`
- Modify: `internal/service/combo-handler.go` only if result metadata is needed

## Implementation Steps
1. Add an `AccountSelectionErrorKind` such as `SelectionErrNoAllowedConnection`.
2. Track `allowedByPolicyCount` separately from `supportedCount`.
3. When all provider connections are filtered out by `allowedConnectionIDs`, return `SelectionErrNoAllowedConnection`.
4. Map `SelectionErrNoAllowedConnection` to a retryable combo result if it is only for this combo member.
5. Represent internal policy denial separately from upstream 403:
   - option A: add `ComboResult.PolicyDenied bool`
   - option B: use a local sentinel error/status handling function.
6. Ensure pinned connection not allowed remains terminal for that attempted model and cannot fallback into disallowed models.
7. Add tests:
   - key allows only OpenAI connection; combo starts with Kiro then OpenAI; request reaches OpenAI.
   - pinned disallowed connection returns 403 and does not fallback to a disallowed model.

## Todo List
- [x] Add no-allowed-connection error kind.
- [x] Update selection counters.
- [x] Update mapping/fallback semantics.
- [x] Add combo connection policy tests.

## Risk Assessment
- Need preserve existing unsupported model behavior for real unsupported model errors.
- Tests should assert status code differences.

## Security Considerations
- Avoid treating policy errors as upstream transient failures.
