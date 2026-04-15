# Frontend Next.js -> Vite Port Plan

Date: 2026-04-10
Status: approved

## Goal

Port `frontend/` (Next.js) into `ui/` (React + Vite), remove Next.js runtime dependence, and use only Go backend APIs (`/api`) as source of truth.

## Scope

1. Replace Next app entry (`layout.tsx` + `page.tsx`) with Vite entry (`main.tsx` + `App.tsx`)
2. Bring over shared UI layer (`components/ui`, `components/screens`, `hooks`, `stores`, `lib`)
3. Remove Next-only code paths (`'use client'`, `next/font`, Next API routes)
4. Standardize API calls through `go-api.ts` only
5. Ensure unsupported legacy Next route calls are replaced by Go-backed flows
6. Build + verify Vite app

## Execution Steps

### Phase 1 - Vite app shell
- Add Vite-compatible app bootstrap using `ThemeProvider`, `TooltipProvider`, and `Toaster`
- Preserve sidebar/page navigation behavior from Next `page.tsx`
- Copy global styles from `frontend/src/app/globals.css`

### Phase 2 - Source migration
- Copy `frontend/src/components/ui/*` to `ui/src/components/ui/*`
- Copy `frontend/src/components/screens/*` to `ui/src/components/screens/*`
- Copy `frontend/src/hooks/*`, `frontend/src/stores/*`, `frontend/src/lib/utils.ts`, `frontend/src/lib/go-api.ts`
- Remove Next API route dependencies from screen code

### Phase 3 - Backend alignment
- Update `go-api.ts` base URL and env var for Vite
- Replace direct `fetch('/api/...')` usages in screen files with `goApi` methods
- Align settings and API key screens with current Go API response models
- Simplify backup/log views where Next-only APIs are not available

### Phase 4 - Tooling + verification
- Update `ui/package.json` dependencies for migrated components
- Update `vite.config.ts` proxy to Go backend port `20199`
- Build and fix compile/runtime integration issues

## Done Criteria

- `ui` no longer imports from `next/*`
- `ui` builds with Vite
- primary screens load and call Go backend endpoints only
- no calls to removed Next API routes remain
