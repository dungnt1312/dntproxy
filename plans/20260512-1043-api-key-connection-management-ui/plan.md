---
title: "API Key Connection Management UI"
description: "Hoan thien giao dien quan ly connection/model allowlist theo tung dntproxy API key."
status: completed
priority: P1
effort: 8h
branch: master
tags: [feature, frontend, api, auth]
created: 2026-05-12
---

# API Key Connection Management UI

## Overview

Hoan thien module API Keys de admin tao/sua/xem quyen theo connection va model. Backend da co policy fields va enforcement; viec con lai la expose trong UI/API client, them validation, test, docs.

## Phases

| # | Phase | Status | Effort | Link |
|---|-------|--------|--------|------|
| 1 | API client va type contract | Completed | 1.5h | [phase-01-api-client-and-types.md](./phase-01-api-client-and-types.md) |
| 2 | UI tao/sua API key permissions | Completed | 3.5h | [phase-02-api-key-permissions-ui.md](./phase-02-api-key-permissions-ui.md) |
| 3 | Backend validation va tests | Completed | 2h | [phase-03-validation-and-tests.md](./phase-03-validation-and-tests.md) |
| 4 | Docs va release checks | Completed | 1h | [phase-04-docs-and-release-checks.md](./phase-04-docs-and-release-checks.md) |

## Dependencies

- Backend endpoints existed: `GET/POST/PUT/DELETE /api/keys`
- Existing fields: `allowedConnectionIds`, `allowedModels`
- Existing UI primitives: Dialog, Checkbox, Switch, Badge, Table, Button
- Existing source of connections/models: `goApi.getConnections()`, `goApi.getModels()`

## Execution Notes

- Fast plan mode. No external research needed.
- Keep API Keys screen under 400 lines by extracting permission dialogs/components.
- Empty allowlist means unrestricted. UI must say this clearly.
- Do not expose provider credential API keys; only dntproxy API keys.

## Success Criteria

- Admin can create key with unrestricted or selected connections/models.
- Admin can edit existing key name, active state, connection allowlist, model allowlist.
- Table/mobile cards show permission summary.
- Requests using restricted key only route through allowed connections/models.
- UI build and Go tests pass.

## Completion

- Completed: 2026-05-12
- Verification: `go test ./...`; `cd ui; bun run build`

## Cook Handoff

Run after review:

```powershell
/ck:cook --auto C:\laragon\www\dntproxy\plans\20260512-1043-api-key-connection-management-ui\plan.md
```
