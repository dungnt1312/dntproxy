# Phase 02 - Effective Pool Service

## Context Links

- [Plan](./plan.md)
- [Model access contracts](./phase-01-model-access-contracts.md)
- [Domain model config](../../internal/domain/model.go)
- [Provider connection](../../internal/domain/provider.go)
- [Credential store port](../../internal/port/credential-store.go)

## Overview

- Priority: P1
- Status: Pending
- Effort: 4h
- Goal: Build computed effective pool from config and API key policy.

## Key Insights

- Effective connection pool must come first.
- Direct model availability comes from active connections and `SupportedModels`.
- Empty `SupportedModels` means provider can use all active registry models for that provider.
- Combo should expose effective members, not raw members.
- Alias should expose only if target is routable or alias explicitly allowed.

## Requirements

- Add `ModelAccessService`.
- `BuildPool(policy)` returns:
  - effective connections
  - direct models
  - aliases with target route availability
  - combos with filtered members
- `ResolveRoute(model, policy)` returns route attempts for chat.
- No cache in first implementation.
- No persisted pool.

## Architecture

Create `internal/service/model-access-service.go`:

```go
type ModelAccessService struct {
  store port.CredentialStore
}

func (s *ModelAccessService) BuildPool(policy *port.APIKeyPolicy) (*EffectiveModelPool, error)
func (s *ModelAccessService) ResolveRoute(model string, policy *port.APIKeyPolicy) (*RoutePlan, error)
```

Pool algorithm:

1. Load config once.
2. Filter active connections by API key connection allowlist.
3. Build model availability map from effective connections.
4. Apply API key model allowlist.
5. Resolve aliases against model map.
6. Resolve combos into effective attempts:
   - direct model member must be allowed.
   - pinned member must be allowed by connection pool.
   - combo `ConnectionIDs` intersects with API key connection policy.
7. Return pool.

## Related Code Files

- Create: `C:\laragon\www\dntproxy\internal\service\model-access-service.go`
- Create: `C:\laragon\www\dntproxy\internal\service\model-access-service_test.go`
- Modify: `C:\laragon\www\dntproxy\internal\service\model-resolver.go` only if shared normalization needed.

## Implementation Steps

1. Implement `NewModelAccessService(store)`.
2. Implement config load once per pool build.
3. Implement `buildEffectiveConnections`.
4. Implement `buildDirectModels`.
5. Implement `buildEffectiveAliases`.
6. Implement `buildEffectiveCombos`.
7. Implement `ResolveRoute` on top of pool.
8. Add table-driven tests:
   - unrestricted key sees all active models/combos/aliases.
   - GLM-only key sees only GLM lite.
   - combo partial allowed remains visible with one member.
   - combo with no allowed members hidden.
   - alias target disallowed hidden.
   - pinned connection disallowed rejected.

## Todo List

- [ ] Create service and constructor.
- [ ] Implement effective connection pool.
- [ ] Implement direct model pool.
- [ ] Implement alias pool.
- [ ] Implement combo pool.
- [ ] Implement route resolve.
- [ ] Add coverage tests.

## Success Criteria

- `BuildPool` is the only place that decides visibility.
- `ResolveRoute` can express direct, alias, combo, and pinned attempts.
- Tests prove list and route use same pool data.

## Risk Assessment

- Risk: changing combo semantics unexpectedly.
- Mitigation: explicit partial-combo tests and docs.
- Risk: model IDs from registry vs `SupportedModels` mismatch.
- Mitigation: normalize all model strings through one helper.

## Security Considerations

- Do not include disallowed connection IDs in route attempts.
- Do not leak hidden alias/combo names through model list.

## Next Steps

- Phase 03 swaps chat routing onto `ResolveRoute`.

## Unresolved Questions

- None. Default combo policy: visible if at least one effective member exists.
