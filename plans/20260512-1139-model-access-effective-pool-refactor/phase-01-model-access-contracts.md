# Phase 01 - Model Access Contracts

## Context Links

- [Plan](./plan.md)
- [README](../../README.md)
- [Code standards](../../docs/code-standards.md)
- [Chat service](../../internal/service/chat-service.go)
- [Model resolver](../../internal/service/model-resolver.go)
- [Account selector](../../internal/service/account-selector.go)
- [Models handler](../../internal/adapter/http/models-handler.go)
- [API key policy](../../internal/port/chat-service.go)

## Overview

- Priority: P1
- Status: Pending
- Effort: 2h
- Goal: Define explicit types for effective pool and route attempts.

## Key Insights

- Current policy logic is duplicated between `chat-service.go` and `models-handler.go`.
- API key policy is request scoped, already extracted by middleware.
- Pool should be computed, not persisted.
- First version should avoid cache to prevent stale permissions.

## Requirements

- Add focused service-layer types:
  - `ModelRef`
  - `AliasRef`
  - `ComboRef`
  - `RouteAttempt`
  - `RoutePlan`
  - `EffectiveModelPool`
- Keep domain structs unchanged unless absolutely needed.
- Keep HTTP model response types inside adapter.
- Make policy matching functions reusable from service.

## Architecture

Create `internal/service/model-access-types.go`:

```go
type ModelRef struct {
  ID string
  Provider string
  Model string
  ConnectionIDs []string
}

type RouteAttempt struct {
  QualifiedModel string
  Provider string
  Model string
  PinnedConnectionID string
  AllowedConnectionIDs []string
}

type RoutePlan struct {
  RequestedModel string
  IsCombo bool
  ComboName string
  Attempts []RouteAttempt
}
```

## Related Code Files

- Create: `C:\laragon\www\dntproxy\internal\service\model-access-types.go`
- Create: `C:\laragon\www\dntproxy\internal\service\model-access-policy.go`
- Modify later: `C:\laragon\www\dntproxy\internal\service\chat-service.go`
- Modify later: `C:\laragon\www\dntproxy\internal\adapter\http\models-handler.go`

## Implementation Steps

1. Define service-layer types with concise comments.
2. Move/duplicate temporarily policy string normalization into `model-access-policy.go`.
3. Add functions:
   - `NormalizeModelPolicyString`
   - `ModelPolicyMatch`
   - `ConnectionAllowed`
   - `IntersectConnectionIDs`
4. Keep names exported only if needed by adapter.
5. Add unit tests for policy matching before integration.

## Todo List

- [ ] Add model access type file.
- [ ] Add central policy helpers.
- [ ] Add policy helper tests.
- [ ] Document semantics in comments.

## Success Criteria

- Policy helper tests cover aliases, provider aliases, duplicated provider prefix, and pinned connection.
- No HTTP import inside service.
- No domain behavior change yet.

## Risk Assessment

- Risk: helper naming conflicts with existing unexported functions.
- Mitigation: create new names and remove old helpers in later phases.

## Security Considerations

- Policy helpers must default deny only when policy has restriction.
- Nil/empty policy means unrestricted.

## Next Steps

- Phase 02 builds pool using these contracts.

## Unresolved Questions

- None.
