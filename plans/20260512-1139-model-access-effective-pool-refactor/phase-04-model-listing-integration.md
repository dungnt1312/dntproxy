# Phase 04 - Model Listing Integration

## Context Links

- [Plan](./plan.md)
- [Model access service](./phase-02-effective-pool-service.md)
- [OpenAI models handler](../../internal/adapter/http/models-handler.go)
- [Dashboard model API handler](../../internal/adapter/http/model-api-handler.go)
- [Router policy extraction](../../internal/adapter/http/router.go)

## Overview

- Priority: P1
- Status: Pending
- Effort: 2h
- Goal: Use effective pool for all model list endpoints.

## Key Insights

- `/v1/models` recently patched policy locally; this should become thin adapter code.
- Dashboard `/api/models` may remain admin-style full view, or can support optional policy-aware mode.
- OpenAI-compatible clients expect `data[]` of model objects only.

## Requirements

- `/v1/models` calls `ModelAccessService.BuildPool(policy)`.
- Remove policy helper duplication from `models-handler.go`.
- Show:
  - direct models from pool
  - effective combos from pool
  - effective aliases from pool
- Dashboard `/api/models`:
  - default admin full model view remains if dashboard key is admin/unrestricted.
  - if dashboard is accessed with restricted API key, use same pool or expose permission summary clearly.
- Keep response shape backward compatible.

## Architecture

`modelsHandler` becomes:

```go
pool, err := service.NewModelAccessService(store).BuildPool(extractAPIKeyPolicy(c))
models := modelObjectsFromPool(pool)
```

Avoid service creation per request if preferred:

- Build in router once and inject handler dependencies.
- This is cleaner but touches router constructors.

Recommended: instantiate service in router and pass to handler.

## Related Code Files

- Modify: `C:\laragon\www\dntproxy\internal\adapter\http\router.go`
- Modify: `C:\laragon\www\dntproxy\internal\adapter\http\models-handler.go`
- Modify: `C:\laragon\www\dntproxy\internal\adapter\http\model-api-handler.go`
- Modify: `C:\laragon\www\dntproxy\internal\adapter\http\models-handler_test.go`

## Implementation Steps

1. Change `modelsHandler(store)` to `modelsHandler(modelAccess)`.
2. Convert pool refs to OpenAI model objects.
3. Delete local matcher helpers from `models-handler.go`.
4. Decide dashboard `/api/models` behavior:
   - minimal: leave admin full list unchanged.
   - better: add `?effective=true` or apply policy when `extractAPIKeyPolicy(c) != nil`.
5. Update tests for:
   - restricted `/v1/models`.
   - unrestricted `/v1/models`.
   - effective combo/alias visibility.

## Todo List

- [ ] Inject model access service into router handlers.
- [ ] Rewrite `/v1/models` as adapter over pool.
- [ ] Remove duplicate helpers.
- [ ] Decide and implement dashboard model API behavior.
- [ ] Update tests.

## Success Criteria

- `/v1/models` and chat route from identical pool.
- No model policy matcher remains in HTTP layer.
- OpenAI response shape unchanged.

## Risk Assessment

- Risk: dashboard model picker might hide admin choices.
- Mitigation: keep `/api/models` admin view by default; add effective mode only if needed.

## Security Considerations

- Restricted `/v1/models` must not leak hidden combo/alias names.
- Restricted key should not learn provider inventory outside its pool.

## Next Steps

- Phase 05 validates end-to-end behavior.

## Unresolved Questions

- Should `/api/models` be admin-full or policy-effective when dashboard login uses restricted key? Recommendation: admin-full only for unrestricted/admin key; effective for restricted key.
