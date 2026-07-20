# Multi-Tenancy (SaaS Mode)

dntproxy supports multi-tenancy for SaaS deployments where a single proxy
instance serves multiple customers (tenants) with **logical isolation** of
connections, combos, aliases, API keys, and logs.

## Overview

- **Isolation model**: Logical (single `db.json` + `logs.db`, filtered by
  `tenantId`). No physical separation is performed.
- **Tenant identification**: API keys carry a `tenantId`. When a request is
  authenticated, the resolved tenant is propagated through the request
  lifecycle.
- **Backward compatibility**: Existing single-tenant deployments continue to
  work unchanged. Records without a `tenantId` are treated as the **legacy /
  global** tenant (`""`), which sees everything.

## Data Model

Each scoping-aware resource now carries an optional `tenantId` field:

| Type | Field | Default |
|---|---|---|
| `domain.APIKey` | `tenantId` | `""` (legacy/global) |
| `domain.ProviderConnection` | `tenantId` | `""` |
| `domain.Combo` | `tenantId` | `""` |
| `domain.LogEntry` | `tenantId` | `""` |
| `domain.LogQuery` | `tenantId` | `""` (no filter) |

## Visibility Rules

| Requester tenant | Resource tenant | Visible? |
|---|---|---|
| `""` (legacy) | any | ✅ Yes (admin/global view) |
| `"global"` | any | ✅ Yes (treated same as `""`) |
| `"acme"` | `"acme"` | ✅ Yes (own resources) |
| `"acme"` | `"globex"` | ❌ No |
| `"acme"` | `""` (legacy) | ❌ No (strict isolation) |

Legacy resources (`tenantId == ""`) belong to the global namespace and are
NOT exposed to specific tenants. This prevents accidental cross-tenant data
leakage when migrating from single-tenant to multi-tenant.

## API Key Format

API keys encode the tenant as a prefix:

```
sk-dnt-{tenantSlug}-{random48hex}    # tenant-scoped key
sk-dnt-{random48hex}                 # legacy/global key
```

Example:
```
sk-dnt-acme-9f3c1b2a4d5e6f7...       # belongs to tenant "acme"
sk-dnt-globex-2a4d5e6f7b8c9d1...     # belongs to tenant "globex"
```

The `tenantId` is also stored on the `APIKey` record. The middleware reads it
from the resolved key, so clients don't need to send a separate header.

## Creating Tenant Resources

Include `tenantId` in the request body when creating API keys, combos, or
connections. Example — create an API key for tenant `acme`:

```bash
curl -X POST http://localhost:20199/api/keys \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ACME production key",
    "dashboardAccess": false,
    "tenantId": "acme"
  }'
```

Response:
```json
{
  "id": "uuid...",
  "name": "ACME production key",
  "key": "sk-dnt-acme-9f3c1b2a...",
  "tenantId": "acme"
}
```

## Request Flow

1. Client sends `Authorization: Bearer sk-dnt-acme-...`
2. `apiKeyMiddleware` looks up the key → resolves `tenantId: "acme"`
3. `GetTenantID(c)` exposes the tenant to handlers
4. Chat/messages handlers pass `TenantID` via `RequestMetadata`
5. `ChatService` → `ModelAccessService` → `AccountSelector` propagate it
6. Storage layer (`LoadForTenant`) returns only the tenant's connections,
   combos, etc.
7. Logs are tagged with `tenantId` on insert and filtered on read

## Management API

The dashboard API (`/api/*`) applies tenant scoping automatically when the
authenticated key has a `tenantId`:

- `GET /api/connections` — only this tenant's connections
- `GET /api/combos` — only this tenant's combos
- `GET /api/keys` — only this tenant's keys
- `GET /api/logs` — only this tenant's logs (incl. summary / daily stats)

Legacy/global keys (`tenantId == ""`) see everything (admin view).

## Migration

Existing databases are migrated automatically on first launch:

- `db.json`: existing records get `tenantId: ""` (legacy/global).
- `logs.db`: `tenant_id TEXT` column added via `ALTER TABLE`. A new index
  `idx_logs_tenant_time` is created for efficient tenant-filtered queries.

No manual migration step is required.

## Limitations / Future Work

- **Aliases** (`modelAliases`) are currently shared globally. Per-tenant
  aliasing would require extending the alias map structure.
- **Settings** (`settings`) and **model registry** are global. Per-tenant
  settings are not yet supported.
- **Physical isolation** (per-tenant DB files) is not implemented; this
  version uses logical filtering only.
- **Admin override**: Admin = global/legacy key (no `tenantId`). The Tenants
  management screen is visible only to admin sessions.

## Tenant Management (admin-only)

Tenants are first-class entities registered by an admin. Each tenant has:

| Field | Purpose |
|---|---|
| `id` | UUID (stable identifier) |
| `slug` | Unique lowercase slug, used as the API key prefix (`sk-dnt-{slug}-…`) |
| `name` | Display name |
| `status` | `active` or `disabled` |
| `notes` | Free-form admin notes |
| `createdAt` / `updatedAt` | RFC3339 timestamps |

### Admin API

All endpoints require an admin (global/legacy) API key. Non-admin keys get 403.

| Method & Path | Description |
|---|---|
| `GET /api/tenants` | List all tenants with aggregated resource counts |
| `POST /api/tenants` | Create a tenant (`{slug, name?, notes?}`) |
| `PUT /api/tenants/:id` | Update name/notes/status |
| `DELETE /api/tenants/:id` | Delete tenant (use `?cascade=true` to also delete its connections/combos/keys) |
| `POST /api/tenants/:id/keys` | Generate an API key pinned to the tenant |

### Slug rules

- 2-32 chars, lowercase alphanumeric + hyphens only
- Must start and end with an alphanumeric character
- Reserved words rejected: `global`, `admin`, `all`, `default`
- Normalized via `domain.NormalizeTenantSlug` (lowercase, trim, collapse separators)

### Disabling a tenant

When an admin sets `status: "disabled"`:

1. The `dashboardKeyMiddleware` / `apiKeyMiddleware` checks the tenant's
   status on every authenticated request (with a 5-second in-memory cache to
   amortize store lookups).
2. If the resolved `apiKey.TenantID` matches a disabled tenant, the request is
   rejected with **403 "Tenant is disabled"**.
3. Admin/legacy keys are never affected.
4. The cache is invalidated immediately when an admin updates a tenant's
   status, so enable/disable takes effect without waiting for the TTL.

Resources (connections, combos, keys, logs) belonging to a disabled tenant are
**preserved** — re-enabling the tenant restores access instantly.

### Provisioning keys for tenants

Admins generate tenant-scoped keys from the **Tenants** screen in the
dashboard (admin-only nav item) or via the API:

```bash
# 1. Create a tenant
curl -X POST http://localhost:20199/api/tenants \
  -H "Authorization: Bearer <admin-key>" \
  -H "Content-Type: application/json" \
  -d '{"slug":"acme","name":"ACME Corp"}'

# 2. Generate a key for the tenant (returns the key once)
curl -X POST http://localhost:20199/api/tenants/<tenant-id>/keys \
  -H "Authorization: Bearer <admin-key>" \
  -H "Content-Type: application/json" \
  -d '{"name":"ACME Production","dashboardAccess":true}'
# → {"id":"…","key":"sk-dnt-acme-9f3c1b…","tenantId":"acme"}
```

The admin then delivers the key to the tenant out-of-band (chat, email,
password manager). The tenant uses it as a normal Bearer token; the proxy
resolves the tenant from the key and scopes every request accordingly.

### UI

- The **Tenants** nav item appears in the sidebar only for admin sessions
  (`session.isAdmin === true`).
- Non-admin users never see the item; visiting `/tenants` directly redirects
  to the dashboard.
- The screen provides: create/edit dialogs, status toggle (enable/disable),
  per-tenant key generation dialog, and delete (with confirmation).

## Code Touchpoints

| Area | File |
|---|---|
| Domain types | `internal/domain/{config,provider,model,log-entry}.go` |
| Tenant entity | `internal/domain/tenant-entity.go` |
| Filter helpers | `internal/domain/tenant.go` |
| Port extension | `internal/port/credential-store.go`, `request-context.go` |
| Storage | `internal/adapter/storage/json-db.go`, `sqlite-log-store*.go` |
| Service layer | `internal/service/{chat-service,model-access-service,model-resolver,account-selector}.go` |
| HTTP middleware | `internal/adapter/http/router.go` (`GetTenantID`, `GetAPIKey`, tenant-disable cache) |
| Tenant handlers | `internal/adapter/http/tenant-handler.go` |
| Other handlers | `chat-handler.go`, `messages-handler.go`, `models-handler.go`, `connection-handler.go`, `combo-api-handler.go`, `key-handler.go`, `log-handler.go` |
| Logger | `internal/logger/reqlog.go` (`TenantID` field) |
| UI store | `ui/src/stores/app-store.ts` (session info) |
| UI API client | `ui/src/lib/go-api.ts` (validateKey/session/tenants) |
| UI components | `ui/src/components/layout/tenant-badge.tsx`, `ui/src/components/screens/tenants-screen.tsx`, `ui/src/components/screens/tenants/` |

## Tests

- `internal/domain/tenant_test.go` — visibility rules, filters, slug validation, disable check
- `internal/service/chat-service-tenant_test.go` — ChatService + combo isolation
- `internal/adapter/storage/sqlite-log-store-tenant_test.go` — log tenant scoping
