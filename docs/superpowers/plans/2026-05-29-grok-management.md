# Grok Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete Grok/xAI management behavior for connection testing, static model listing, and clear usage/quota handling.

**Architecture:** Keep xAI aligned with CLIProxyAPI: OAuth execution goes through `POST /v1/responses`, model lists are static registry data, and quota remains unsupported because no live xAI quota endpoint is known. Implement minimal provider-specific handling at the existing HTTP handler boundaries.

**Tech Stack:** Go 1.25, Gin HTTP handlers, existing domain provider/model registry, existing SQLite/local usage handlers, Vite/React build verification.

---

### Task 1: Fix xAI Connection Test URL

**Files:**
- Modify: `internal/adapter/http/connection-test-handler.go`
- Test: `internal/adapter/http/connection-test-handler_test.go`

- [ ] **Step 1: Write failing test for xAI URL keeping `/v1`**

Add a test that exercises the URL builder/helper used by `testProviderAPI` or introduce a small helper test if needed. Expected behavior: xAI base `https://api.x.ai/v1` plus chat path `/responses` returns `https://api.x.ai/v1/responses`.

- [ ] **Step 2: Run focused test**

Run: `/usr/local/go/bin/go test ./internal/adapter/http -run TestXAIConnectionTestURL`

Expected before implementation: FAIL because generic path strips `/v1`.

- [ ] **Step 3: Implement provider-specific URL handling**

In `testProviderAPI`, skip `domain.StripVersionSuffix` for provider `xai`; keep existing behavior for other providers.

- [ ] **Step 4: Run focused and package tests**

Run: `/usr/local/go/bin/go test ./internal/adapter/http -run TestXAIConnectionTestURL`

Run: `/usr/local/go/bin/go test ./internal/adapter/http`

Expected: PASS.

### Task 2: Expand xAI Static Model Registry

**Files:**
- Modify: `internal/domain/model-definition.go`
- Test: `internal/domain/model-definition_test.go` or existing closest model registry test file

- [ ] **Step 1: Write failing model registry test**

Assert `DefaultModelRegistry()` includes xAI definitions for `xai/grok-build-0.1`, `xai/grok-4.3`, `xai/grok-4.20-0309-reasoning`, `xai/grok-4.20-0309-non-reasoning`, `xai/grok-4.20-multi-agent-0309`, `xai/grok-3-mini`, and `xai/grok-3-mini-fast`.

- [ ] **Step 2: Run focused test**

Run: `/usr/local/go/bin/go test ./internal/domain -run TestDefaultModelRegistryIncludesXAIModels`

Expected before implementation: FAIL for missing model IDs.

- [ ] **Step 3: Add static model definitions**

Add model definitions with provider `xai`, model IDs without provider prefix, sensible display names, and context/pricing metadata following the existing xAI entries.

- [ ] **Step 4: Run focused test**

Run: `/usr/local/go/bin/go test ./internal/domain -run TestDefaultModelRegistryIncludesXAIModels`

Expected: PASS.

### Task 3: Improve xAI Quota Message

**Files:**
- Modify: `internal/adapter/http/quota-handler.go`
- Test: `internal/adapter/http/quota-handler_test.go` if practical; otherwise verify through package tests/build.

- [ ] **Step 1: Add xAI-specific unsupported quota response**

For `conn.Provider == "xai"`, return `hasData=false`, `buckets=[]`, and message `xAI does not expose live quota for Grok Build OAuth; usage appears after successful requests.`

- [ ] **Step 2: Run HTTP package tests**

Run: `/usr/local/go/bin/go test ./internal/adapter/http`

Expected: PASS.

### Task 4: Add xAI Usage Fallback Message

**Files:**
- Modify: `internal/adapter/http/usage-handler.go`
- Test: existing package tests where possible

- [ ] **Step 1: Inspect current usage response shape**

Use `usage-handler.go` existing structs and return conventions. Do not invent a new API shape unless required.

- [ ] **Step 2: Add xAI provider branch**

Return an empty usage/quota result with provider-specific message explaining xAI usage is local-log based and appears after requests if no aggregate is available.

- [ ] **Step 3: Run HTTP package tests**

Run: `/usr/local/go/bin/go test ./internal/adapter/http`

Expected: PASS.

### Task 5: Full Verification

**Files:**
- No additional files unless verification exposes failures.

- [ ] **Step 1: Run all Go tests**

Run: `/usr/local/go/bin/go test ./...`

Expected: PASS.

- [ ] **Step 2: Build UI**

Run from `ui/`: `npm run build`

Expected: PASS; chunk size warning is acceptable.

- [ ] **Step 3: Build binary**

Run: `/usr/local/go/bin/go build -o /tmp/dntproxy ./cmd/dntproxy/`

Expected: PASS.
