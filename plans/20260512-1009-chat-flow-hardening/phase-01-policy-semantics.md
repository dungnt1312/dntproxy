# Phase 01 - Policy Semantics

## Overview
- Priority: Critical
- Status: Complete
- Purpose: Close API key allowlist bypass through combos.

## Key Insights
- Current `isModelAllowed()` returns true if the original combo name or any resolved model is allowed.
- `ComboHandler` still receives all resolved models after that check.
- This allows a restricted key to fallback into disallowed models.

## Requirements
- Enforce allowed models against every actual attempted model.
- Preserve aliases and direct provider/model behavior.
- Decide combo semantics:
  - Recommended: filter combo models to allowed models, reject if none remain.
  - Reason: lets a key use a shared combo safely without duplicating combos per key.

## Related Code Files
- Modify: `internal/service/chat-service.go`
- Modify/add tests: `internal/service/chat-service_test.go` or new focused test file

## Implementation Steps
1. Replace boolean-only `isModelAllowed()` use with a helper that returns filtered routing models.
2. For direct/alias request:
   - allow if original request string or resolved model matches allowlist.
   - reject 403 otherwise.
3. For combo request:
   - keep only resolved models allowed by policy.
   - if original combo name is allowed, keep all models only if that is intended admin semantics; otherwise avoid combo-name shortcut.
   - recommended: combo name does not bypass per-model policy.
4. Normalize comparisons by stripping `@connectionId` for clean model checks, but also permit exact pinned model entries.
5. Add tests:
   - combo contains allowed + disallowed; only allowed is attempted.
   - combo contains no allowed models; request returns 403.
   - direct alias allowed by alias name still works.

## Todo List
- [x] Implement model filtering helper.
- [x] Remove unsafe "any model allowed means whole combo allowed" behavior.
- [x] Add allowlist combo tests.

## Risk Assessment
- Existing users may expect allowing combo name to allow all combo members.
- Mitigation: document semantics clearly; per-model policy is safer.

## Security Considerations
- This is the primary security fix. Avoid fail-open behavior.
