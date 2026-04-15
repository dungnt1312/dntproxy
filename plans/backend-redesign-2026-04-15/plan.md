---
status: pending
priority: high
effort: 6 weeks
created: 2026-04-15
---

# Backend Architecture Redesign — DNTProxy

## Overview

Complete backend rewrite to simplify architecture, improve performance, and add API key-centric features. Moving from complex multi-layer architecture to streamlined JSON+in-memory design with proper domain modeling.

## Goals

1. **Simplify persistence** — JSON config + SQLite logs (no Postgres)
2. **Improve performance** — <50ms p99 latency, 100+ concurrent requests
3. **Add API key features** — credits, rate limits, model/provider whitelists, expiry
4. **Fix provider bugs** — Kiro EventStream CRC, Anthropic SSE, OpenAI Codex
5. **Clean architecture** — proper domain models, type-safe, testable
6. **Maintain features** — multi-provider, multi-account, combos, OAuth, tunnel

## Architecture Changes

### Current Architecture
```
- Complex port/adapter pattern with 15+ interfaces
- String-based everything (no type safety)
- Race conditions in concurrent access
- DB queries on hot path
- Inconsistent error handling
- 70+ identified issues
```

### Proposed Architecture
```
- Simplified 4-layer design
- In-memory config with RWMutex
- Proper domain models (APIKey, ProviderAccount, Combo)
- Zero DB latency on hot path
- Token bucket rate limiting
- Async credit deduction & logging
```

## Key Improvements

### Performance
- **In-memory config** — O(1) lookup, no DB hit
- **Async operations** — credits, logging don't block response
- **Token bucket rate limiter** — smooth, in-memory
- **Batch logging** — 100 logs or 1s flush
- **Target:** <50ms p99 (vs current ~100ms)

### Simplicity
- **JSON config** — single file, git-friendly, hot-reloadable
- **SQLite logs** — append-only, 30-day retention
- **No Postgres** — easier deployment
- **Fewer abstractions** — 4 layers vs 6

### Type Safety
- **Proper structs** — APIKey, ProviderAccount, Combo, ModelPricing
- **Enums** — AuthType, Strategy, Status
- **No stringly-typed** — compile-time safety

### Features
- **API key credits** — balance tracking, auto-deduction
- **API key rate limits** — per-key RPM/TPM
- **API key permissions** — model/provider whitelists
- **API key expiry** — time-based invalidation
- **Multi-account fallback** — priority-based routing
- **Combo strategies** — fallback, round-robin

## Migration Strategy

### Phase 1: Core Foundation (Week 1)
- Domain models (APIKey, ProviderAccount, Combo, ModelPricing)
- ConfigStore (in-memory + JSON file)
- LogStore (SQLite with batch writer)
- Unit tests for all models

### Phase 2: Service Layer (Week 2)
- RequestOrchestrator (main flow)
- RateLimiter (token bucket)
- CreditManager (balance tracking)
- ProviderRouter (account selection + fallback)
- ComboExecutor (strategy execution)
- Integration tests

### Phase 3: Provider Adapters (Week 3)
- Fix Kiro EventStream CRC validation
- Fix Anthropic SSE format
- Fix OpenAI Codex translation
- Refactor all 7 providers to new interface
- Provider-specific tests

### Phase 4: HTTP Layer (Week 4)
- Chat handler (/v1/chat/completions)
- Admin API handlers (keys, accounts, combos)
- Middleware (auth, rate limit, logging)
- API key authentication
- Error handling & status codes

### Phase 5: Features & Polish (Week 5)
- OAuth flows (Builder ID, IDC, Social, Import)
- Cloudflare tunnel integration
- File watcher (auto-reload config)
- Token refresh scheduler
- Performance testing (100+ concurrent)
- Load testing & optimization

### Phase 6: Dashboard & Deployment (Week 6)
- React UI refactor
- API key management UI
- Provider account management UI
- Usage analytics & charts
- Credit top-up UI
- Documentation update
- Deployment guide

## Success Criteria

### Performance
- [ ] <50ms p99 latency for chat completions
- [ ] 100+ concurrent requests without degradation
- [ ] <10ms config lookup (in-memory)
- [ ] <5ms rate limit check (in-memory)

### Functionality
- [ ] All 7 providers working (Kiro, OpenAI, Anthropic, GLM, MiniMax, Qwen, OpenAI-Compatible)
- [ ] Multi-account fallback with cooldown
- [ ] Combo strategies (fallback, round-robin)
- [ ] API key credits with auto-deduction
- [ ] API key rate limits (RPM, TPM)
- [ ] API key permissions (model/provider whitelist)
- [ ] OAuth flows (4 methods)
- [ ] Cloudflare tunnel
- [ ] Structured logging with 30-day retention

### Quality
- [ ] 80%+ test coverage
- [ ] Zero race conditions (verified with -race)
- [ ] No memory leaks (verified with pprof)
- [ ] Clean architecture (4 layers, clear boundaries)
- [ ] Type-safe (no stringly-typed)
- [ ] Documented (godoc + README)

### Deployment
- [ ] Single binary deployment
- [ ] Config hot-reload (fsnotify)
- [ ] Graceful shutdown
- [ ] Docker support
- [ ] Install scripts (Linux/macOS/Windows)

## Timeline

- **Week 1:** Phase 1 — Core Foundation
- **Week 2:** Phase 2 — Service Layer
- **Week 3:** Phase 3 — Provider Adapters
- **Week 4:** Phase 4 — HTTP Layer
- **Week 5:** Phase 5 — Features & Polish
- **Week 6:** Phase 6 — Dashboard & Deployment

**Total:** 6 weeks (30 working days)

## Risks & Mitigation

### Risk: Config Corruption
- **Mitigation:** Atomic writes (.tmp → rename), JSON validation on load, backup on every save

### Risk: Credit Drift
- **Mitigation:** Async save with retry, periodic reconciliation, acceptable for single admin

### Risk: Rate Limit Reset
- **Mitigation:** Persist buckets to disk (optional), acceptable for restart scenario

### Risk: Log Loss
- **Mitigation:** Backpressure with drop policy, monitor queue depth, acceptable for high load

### Risk: Breaking Changes
- **Mitigation:** Feature flag for new backend, parallel run, gradual migration

## Dependencies

- Go 1.25+
- Gin (HTTP)
- Cobra (CLI)
- fsnotify (file watcher)
- SQLite (logs)
- bcrypt (key hashing)
- React (dashboard)

## References

- Design document: `docs/brainstorm-2026-04-15-backend-redesign.md`
- Current codebase: `internal/`
- Phase details: `phase-1.md` through `phase-6.md`
