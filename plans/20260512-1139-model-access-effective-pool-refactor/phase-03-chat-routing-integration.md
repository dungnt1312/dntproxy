# Phase 03 - Chat Routing Integration

## Context Links

- [Plan](./plan.md)
- [Effective pool service](./phase-02-effective-pool-service.md)
- [Chat service](../../internal/service/chat-service.go)
- [Combo handler](../../internal/service/combo-handler.go)
- [Account selector](../../internal/service/account-selector.go)

## Overview

- Priority: P1
- Status: Pending
- Effort: 3h
- Goal: Make chat execution consume `RoutePlan` from `ModelAccessService`.

## Key Insights

- `ChatService.HandleChat` currently resolves model and applies policy itself.
- `AccountSelector` still should own cooldown, model lock, token refresh, weighted choice.
- Route plan should decide which connections are eligible; selector chooses among them.
- Avoid broad executor changes.

## Requirements

- Add `modelAccess *ModelAccessService` to `ChatService`.
- `HandleChat` calls `ResolveRoute(modelStr, policy)`.
- Convert `RouteAttempt` to current combo handler input or adapt combo handler to accept attempts.
- Keep fallback behavior:
  - direct model = one attempt.
  - combo = N attempts after policy filter.
  - round-robin still works on attempts.
- Remove old duplicated model policy filtering from `chat-service.go`.

## Architecture

Preferred minimal integration:

```go
plan, err := s.modelAccess.ResolveRoute(modelStr, policy)
attempts := plan.Attempts
result, err := s.comboHandler.HandleComboAttempts(attempts, comboName, strategy, execute)
```

If avoiding combo handler API change:

- Add `RouteAttempt.QualifiedModel`.
- Convert attempts to `[]string`.
- Keep `allowedConnIDsByQualifiedModel` map.
- Pass per-attempt `AllowedConnectionIDs` into `executeOnProvider`.

Recommended: small combo handler extension, clearer long-term.

## Related Code Files

- Modify: `C:\laragon\www\dntproxy\internal\service\chat-service.go`
- Modify: `C:\laragon\www\dntproxy\internal\service\combo-handler.go`
- Modify: `C:\laragon\www\dntproxy\internal\service\chat-service_test.go`
- Optional modify: `C:\laragon\www\dntproxy\internal\service\account-selector.go`

## Implementation Steps

1. Wire `ModelAccessService` into `NewChatService`.
2. Update `NewChatServiceWithDeps` or add test constructor variant.
3. Replace `resolver.ResolveRouting` + policy filtering with `modelAccess.ResolveRoute`.
4. Preserve combo strategy lookup by combo name.
5. Update execution closure to receive `RouteAttempt`.
6. Ensure pinned connection policy still enforced before selector.
7. Remove old `filterAllowedRoutingModels`, `modelsPolicyMatch`, duplicate `intersectConnectionIDs` if unused.
8. Update existing chat tests.

## Todo List

- [ ] Wire service dependency.
- [ ] Integrate `RoutePlan` in `HandleChat`.
- [ ] Extend combo handler or adapt attempts safely.
- [ ] Remove old duplicate policy helpers.
- [ ] Update tests.

## Success Criteria

- Existing chat tests pass.
- API key allowed model filtering is implemented only by model access service.
- Account selector still handles cooldown/backoff/model locks.
- No behavior regression for unrestricted requests.

## Risk Assessment

- Risk: combo round-robin state changes if attempt keys differ.
- Mitigation: keep combo name same; only attempts list changes.
- Risk: old tests assume raw combo length.
- Mitigation: update expectations to effective combo members.

## Security Considerations

- Forbidden direct model returns 403.
- Unknown model/combo still returns 400.
- Disallowed pinned connection never reaches provider executor.

## Next Steps

- Phase 04 moves model listing APIs to the same pool.

## Unresolved Questions

- None.
