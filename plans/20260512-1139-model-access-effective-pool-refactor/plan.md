---
title: "Model Access Effective Pool Refactor"
description: "Centralize API key model/connection policy into a computed effective pool used by model listing and chat routing."
status: completed
priority: P1
effort: 14h
branch: master
tags: [refactor, backend, api, auth]
created: 2026-05-12
---

# Model Access Effective Pool Refactor

## Overview

Refactor model access logic so each API key gets a computed effective pool from current config + policy. Use this single source for `/v1/models`, dashboard model visibility, aliases, combos, and chat execution.

## Design Decision

- Do not persist per-key pools.
- Do not cache in first implementation.
- Build pool from current config per request.
- Pool order: effective connections -> direct models -> model allowlist -> aliases/combos -> route attempts.

## Phases

| # | Phase | Status | Effort | Link |
|---|-------|--------|--------|------|
| 1 | Define model access contracts | Done | 2h | [phase-01-model-access-contracts.md](./phase-01-model-access-contracts.md) |
| 2 | Build effective pool service | Done | 4h | [phase-02-effective-pool-service.md](./phase-02-effective-pool-service.md) |
| 3 | Route chat through pool | Done | 3h | [phase-03-chat-routing-integration.md](./phase-03-chat-routing-integration.md) |
| 4 | Use pool for model listing APIs | Done | 2h | [phase-04-model-listing-integration.md](./phase-04-model-listing-integration.md) |
| 5 | Tests and docs sync | Done | 3h | [phase-05-tests-and-docs.md](./phase-05-tests-and-docs.md) |

## Dependencies

- Existing `port.APIKeyPolicy`
- Existing `domain.ProviderConnection`, `domain.Combo`, `domain.AliasMap`
- Existing `ModelResolver`, `AccountSelector`, `ComboHandler`
- Current `/v1/models` policy patch remains temporary until service integration

## Target Flow

```text
AppConfig + APIKeyPolicy
  -> ModelAccessService.BuildPool
  -> effective connections/models/aliases/combos
  -> ListModels or ResolveRoute
  -> ComboHandler + AccountSelector execute attempts
```

## Success Criteria

- `/v1/models` and chat execution always agree.
- Combo visibility uses effective combo members, not raw combo only.
- Alias visibility uses resolved target access.
- Pinned `provider/model@connectionId` respects API key pool.
- No duplicated policy matcher in HTTP and service layers.
- `go test ./...` and `bun run build` pass.

## Cook Handoff

```powershell
/ck:cook --auto C:\laragon\www\dntproxy\plans\20260512-1139-model-access-effective-pool-refactor\plan.md
```
