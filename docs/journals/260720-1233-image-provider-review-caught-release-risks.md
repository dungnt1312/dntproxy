---
date: 2026-07-20 12:33
severity: high
component: image-provider-registry-byteplus-gemini
status: resolved
---

# Image Provider Review Caught Release Risks Before Deploy

**Date**: 2026-07-20 12:33  
**Severity**: High  
**Component**: Image provider registry, Gemini editing, OpenAI-compatible routing  
**Status**: Resolved

## What Happened

The registry refactor and BytePlus/Gemini integration worked functionally, but review found four release-grade defects before deployment: remote image loading did not block every special-use network, Gemini errors could echo signed image URLs, capability metadata did not describe the aggregate request envelope, and the OpenAI-compatible adapter rejected unknown image model names.

## The Brutal Truth

This was uncomfortably close to shipping a feature that looked clean architecturally while leaking credentials and breaking existing users. The frustrating part is that each defect came from optimizing the happy path: fetch a URL, expose metadata, validate a model. None was exotic. We nearly turned “provider abstraction” into a security regression and a compatibility regression in the same release.

## Technical Details

`internal/adapter/shared/image-input-loader.go` now rejects non-public and special-use ranges including `100.64.0.0/10`, `192.0.2.0/24`, `198.18.0.0/15`, `198.51.100.0/24`, `203.0.113.0/24`, and `2001:db8::/32`, in addition to loopback, private, link-local, and multicast addresses. DNS is resolved before dialing, every answer is validated, redirects are rechecked, and response size/time are bounded.

Gemini upstream errors now replace HTTP(S) and data-image URLs with `[redacted-url]`; inline image bytes are also removed from request/response logs. Capability metadata gained `MaxTotalInputBytes`, and Gemini enforces the advertised 7 MiB aggregate reference limit rather than only a per-image limit. `NewCompatibleImageProvider()` deliberately permits unknown model IDs so existing custom OpenAI-compatible endpoints keep working; strict model recognition remains for the native OpenAI registration.

Verification passed:

```text
go test ./internal/adapter/shared ./internal/adapter/gemini ./internal/adapter/byteplus ./internal/adapter/provider ./internal/adapter/http ./internal/adapter/openai
```

## What We Tried

The first implementation relied on `net.IP.IsGlobalUnicast`, surfaced sanitized provider envelopes only partially, and reused strict OpenAI model detection everywhere. Review rejected all three shortcuts because globally routable syntax is not the same as safe destination policy, provider messages are attacker-controlled, and compatible endpoints own their model namespace.

## Root Cause Analysis

We designed the registry around provider dispatch, then treated URL loading, observability, capability claims, and backward compatibility as adapter details. That boundary was wrong: they are public security and API contracts.

## Lessons Learned

Any feature that ingests user URLs needs an explicit SSRF threat model before code. Never log or return upstream text without redaction. Capability metadata must match executable limits. Compatibility adapters must be permissive about provider-owned identifiers while native adapters can remain strict.

## Next Steps

- Backend owner: keep SSRF, signed-URL redaction, aggregate-limit, and unknown-model cases as mandatory regression tests before every image-provider deploy.
- Reviewer: re-audit redirect/DNS behavior and log redaction whenever the shared loader changes.
- Release owner: run the full Go/UI/build suite and live provider smoke tests before restarting PM2.
