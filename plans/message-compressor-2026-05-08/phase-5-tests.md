# Phase 5 — Tests

**Status:** pending
**Priority:** P1 (gate before merge)
**Estimated:** 1 day

## Context Links

- Phase 1: [`phase-1-core-compressor.md`](./phase-1-core-compressor.md)
- Phase 3: [`phase-3-pipeline-wiring.md`](./phase-3-pipeline-wiring.md)
- Existing tests pattern: search `*_test.go` in `internal/` for tone/style.

## Overview

Cover three test layers:

1. **Unit tests per filter** — every filter has table-driven tests against real-world fixture inputs in `testdata/`.
2. **Compressor end-to-end test** — verifies content-block traversal, fail-open behavior, and stats accuracy.
3. **HTTP integration test** — boots a fake provider, posts a chat request through `chatHandler`, asserts compressed body reaches the upstream stub.

Coverage target: ≥80% on `internal/adapter/compressor/...`.

## Key Insights

- Go's `testing` + `testdata/` is enough — no need for testify or other fixture libraries (preserves "stdlib only" rule).
- Each filter's test file should ship **one fixture per detected sub-type** (e.g. `git-filter_test.go` ships fixtures for status, diff, log).
- Fail-open is a contract: integrate a "garbage input" test for every filter that asserts `ok=false` and the original string is returned by the dispatcher.

## Requirements

### Functional
- Each filter has ≥3 fixture cases (typical, edge, malformed).
- Detector test covers all 9 `ContentType` rules + `ContentGeneric` fallback.
- Compressor test covers: empty body, body without `messages`, OpenAI string content, Anthropic content-blocks, role filtering (`system` skipped), `MinContentLength` threshold.
- Integration test starts a `httptest.Server` stub provider, registers it as a fake `openai-compatible` connection, sends a request with a 5KB `git status` block, asserts the body received by the stub is at least 30% smaller than the original.

### Non-Functional
- All tests in this package run in <2s on a developer laptop.
- No network calls, no actual AWS/OpenAI.

## Architecture

```
internal/adapter/compressor/
├── compressor_test.go
├── content-detector_test.go
├── message-parser_test.go
├── filters/
│   ├── git-filter_test.go
│   ├── test-filter_test.go
│   ├── ls-filter_test.go
│   ├── log-filter_test.go
│   └── json-filter_test.go
└── testdata/
    ├── git-status.txt
    ├── git-diff.txt
    ├── git-log.txt
    ├── go-test-output.txt
    ├── cargo-test-output.txt
    ├── pytest-output.txt
    ├── ls-l.txt
    ├── tree-output.txt
    ├── log-app.txt
    └── api-response.json

internal/adapter/http/
└── chat-handler_compression_integration_test.go
```

## Related Code Files

### Files to create
- All test files listed above.
- All `testdata/*` fixtures (real captured output, ≤8KB each).

### Files to modify
- None (tests are additive).

## Implementation Steps

### Step 1 — Filter unit tests (table-driven)

Template (one per filter):

```go
func TestGitDiffFilter(t *testing.T) {
    cases := []struct{
        name      string
        inputFile string
        wantOK    bool
        wantRatio float64 // max len(out)/len(in)
        wantContains []string // must remain in output
        wantAbsent   []string // must be removed from output
    }{
        {
            name: "small diff",
            inputFile: "testdata/git-diff.txt",
            wantOK: true,
            wantRatio: 0.4,
            wantContains: []string{"diff --git", "@@"},
            wantAbsent:   []string{"index "},
        },
        // …
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            in := mustReadFile(t, tc.inputFile)
            out, ok := GitDiffFilter(in)
            if ok != tc.wantOK { t.Fatalf("ok=%v want %v", ok, tc.wantOK) }
            if ratio := float64(len(out))/float64(len(in)); ratio > tc.wantRatio {
                t.Errorf("ratio=%.2f exceeded %.2f", ratio, tc.wantRatio)
            }
            for _, s := range tc.wantContains {
                if !strings.Contains(out, s) { t.Errorf("missing %q", s) }
            }
            for _, s := range tc.wantAbsent {
                if strings.Contains(out, s) { t.Errorf("should not contain %q", s) }
            }
        })
    }
}
```

Add a `TestXxxFilter_FailOpen` for each, asserting `ok=false` on garbage input and an empty/equal output by the dispatcher.

### Step 2 — Detector tests

```go
func TestDetect(t *testing.T) {
    cases := []struct{
        file string
        want ContentType
    }{
        {"testdata/git-diff.txt", ContentGitDiff},
        {"testdata/git-status.txt", ContentGitStatus},
        {"testdata/git-log.txt", ContentGitLog},
        {"testdata/go-test-output.txt", ContentGoTest},
        {"testdata/cargo-test-output.txt", ContentCargoTest},
        {"testdata/pytest-output.txt", ContentPytest},
        {"testdata/ls-l.txt", ContentLS},
        {"testdata/tree-output.txt", ContentLS},
        {"testdata/log-app.txt", ContentLog},
        {"testdata/api-response.json", ContentJSON},
    }
    // …
}

func TestDetect_GenericFallback(t *testing.T) {
    got := Detect("hello world this is just prose")
    if got != ContentGeneric { t.Fatalf("got %v", got) }
}
```

### Step 3 — Compressor test

```go
func TestCompress_FailOpenOnInvalidJSON(t *testing.T) {
    c := New(Options{Enabled: true, MinContentLength: 1, LogSavings: false})
    in := []byte("{not-json")
    out, stats := c.Compress(in)
    if !bytes.Equal(in, out) { t.Fatalf("body mutated") }
    if stats.OriginalBytes != 0 { t.Fatalf("stats should be empty") }
}

func TestCompress_DisabledShortCircuits(t *testing.T) {
    c := New(Options{Enabled: false})
    body := mustReadFile(t, "testdata/openai-request-with-git-diff.json")
    out, _ := c.Compress(body)
    if !bytes.Equal(body, out) { t.Fatalf("body mutated when disabled") }
}

func TestCompress_AnthropicContentBlocks(t *testing.T) {
    c := New(Options{Enabled: true, MinContentLength: 100})
    body := mustReadFile(t, "testdata/anthropic-request-with-test-output.json")
    out, stats := c.Compress(body)
    if len(out) >= len(body) { t.Fatalf("no compression occurred") }
    if stats.Detections["go-test"] == 0 { t.Fatalf("expected go-test detection") }
}
```

### Step 4 — HTTP integration test

```go
// internal/adapter/http/chat-handler_compression_integration_test.go
func TestChatHandler_CompressesBeforeUpstream(t *testing.T) {
    // 1. Start a httptest.Server that captures the request body.
    var captured []byte
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        b, _ := io.ReadAll(r.Body)
        captured = b
        // emit minimal SSE [DONE] so handler completes
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprintf(w, "data: [DONE]\n\n")
    }))
    defer upstream.Close()

    // 2. Build a stub `port.CredentialStore` with a fake openai-compatible connection
    //    pointing at upstream.URL, plus Settings.Compression.Enabled = true.
    // 3. Build a real router via NewRouter and POST a request with git-diff body.
    // 4. Assert len(captured) < 0.7 * len(originalBody).
}
```

(Use existing test helpers if a stub `CredentialStore` is already present in the repo; otherwise this test can be marked `// TODO: integration` and gated behind a build tag for v1, with the unit tests carrying the bulk of confidence.)

### Step 5 — Run & coverage

```
go test ./internal/adapter/compressor/...           -count=1
go test ./internal/adapter/compressor/... -cover    -count=1
go test ./internal/adapter/http/... -run Compress   -count=1
```

Target: `cover ≥ 0.80` on the compressor package.

## Todo

- [ ] Capture & save 10 testdata fixtures (real outputs from local commands; redact PII/paths).
- [ ] Write detector tests (10 cases + generic fallback).
- [ ] Write each filter's unit tests (typical + edge + fail-open).
- [ ] Write compressor end-to-end tests (5+ cases).
- [ ] Write HTTP integration test (or mark TODO with build tag).
- [ ] Run full `go test ./...` clean.
- [ ] Verify `go test ./internal/adapter/compressor/... -cover` ≥80%.

## Success Criteria

- All tests pass on Linux + Windows.
- Coverage ≥80% on the compressor package.
- Each filter's "fail-open on garbage" test passes.
- Integration test (or its TODO equivalent) demonstrates real over-the-wire compression on at least one realistic fixture.

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Test fixtures contain machine-specific paths or secrets | Manual scrub before commit; testdata files reviewed via `git diff` checklist. |
| Coverage gate too strict for filter edge cases | Allow `wantRatio` to be set per-fixture; coverage ≥80% is on lines, not on every branch. |
| Integration test flaky due to httptest port reuse | Use `httptest.NewServer` (not `NewUnstartedServer`); always defer Close. |

## Security Considerations

- Fixtures must not contain real OAuth tokens, API keys, or repo paths revealing infrastructure.
- Add a one-line CI grep for `sk-`, `aws_`, `xoxb-` patterns in `testdata/` before commit.

## Next Steps

- Once all 5 phases land, update `AGENTS.md` "Implementation Status" with a new bullet under Phase 5/6: "Token compression middleware — DONE".
- Optional follow-up: a benchmark file `compressor_bench_test.go` to track latency regression as filters evolve.

## Unresolved Questions

1. Should the integration test be in the same package as `chat-handler.go` (white-box) or a `_test` package? **Recommendation:** Same package — needs access to unexported `chatHandler` constructor, simpler.
2. Do we need a fuzz test (`go test -fuzz`) on the JSON filter? **Recommendation:** Nice-to-have; not blocking.
