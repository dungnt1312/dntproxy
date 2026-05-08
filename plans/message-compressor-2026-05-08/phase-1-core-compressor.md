# Phase 1 — Core Compressor Package

**Status:** pending
**Priority:** P1 (foundation for all later phases)
**Estimated:** 1–2 days

## Context Links

- Brief: [`./plan.md`](./plan.md)
- Reference style: `internal/adapter/tunnel/cloudflared.go`, `internal/adapter/storage/json-db.go`

## Overview

Build the standalone `internal/adapter/compressor/` package. Pure Go, no external deps, no other internal packages — depends only on `encoding/json`, `regexp`, `strings`, `bufio`, `bytes`. Importable from the http adapter without creating cycles.

## Key Insights

- All 5 filters share the same shape: `func(in string) (out string, ok bool)`. `ok=false` means "I don't recognize this content, leave alone".
- Detector is heuristic, not perfect — every filter must self-validate via regex anchors and bail (`ok=false`) if surprised. **Fail-open is the contract.**
- Most token bloat in tool-using agent loops is `tool_call` / `tool_result` content. Iterating only those messages keeps cost ≤O(N\_tool_msgs · content_len), not O(full_body).

## Requirements

### Functional
- `Compress(body []byte) ([]byte, Stats)` rewrites `messages[*].content` (string or content-block array) only when:
  - role ∈ {`tool`, `user`, `assistant`}, AND
  - content length ≥ `MinContentLength` (default 500), AND
  - detected ContentType ≠ `ContentGeneric`.
- Original body returned verbatim on any error path (JSON parse fail, regex panic recover, unknown structure).
- `Stats` carries: `OriginalBytes`, `CompressedBytes`, `TokensSaved` (= `(orig-comp)/4`), `Detections map[string]int` (e.g. `{"go-test":3, "git-diff":1}`).

### Non-Functional
- Single-pass scan per filter — no nested re-tokenization.
- Total compressor latency ≤2ms for 100KB body on a single core.
- Each Go file ≤200 lines.

## Architecture

```
internal/adapter/compressor/
├── compressor.go           // public API: New(opts), Compress(body)
├── content-detector.go     // Detect(s string) ContentType
├── message-parser.go       // walkMessages(body, fn) — visits content strings
├── stats.go                // Stats struct + helpers
└── filters/
    ├── filter.go           // shared types: Filter func(string) (string, bool)
    ├── git-filter.go       // status / diff / log
    ├── test-filter.go      // go test / cargo test / pytest
    ├── ls-filter.go        // ls / tree / find
    ├── log-filter.go       // generic log dedup + truncation
    └── json-filter.go      // structure-only JSON
```

### Public API (`compressor.go`)

```go
package compressor

type Options struct {
    Enabled          bool
    MinContentLength int  // default 500
    LogSavings       bool
}

type Stats struct {
    OriginalBytes   int
    CompressedBytes int
    TokensSaved     int
    Detections      map[string]int
}

type Compressor struct {
    opts    Options
    filters map[ContentType]filters.Filter
}

func New(opts Options) *Compressor
func (c *Compressor) Compress(body []byte) ([]byte, Stats)  // never returns error — fail-open
```

### Detector Rules (`content-detector.go`)

Check in priority order (most specific first), short-circuit on first match:

| ContentType  | Detection (literal substring or regex anchor) |
|--------------|-----------------------------------------------|
| `ContentGitDiff`   | starts with `diff --git` OR contains `\n@@ -` AND `\n+++` |
| `ContentGitStatus` | contains `On branch ` AND (`Changes not staged` OR `nothing to commit` OR `Untracked files:`) |
| `ContentGitLog`    | matches `commit [a-f0-9]{40}` ≥2 times |
| `ContentGoTest`    | contains `--- PASS:` OR `--- FAIL:` OR matches `^FAIL\t` (multiline) |
| `ContentCargoTest` | contains `running ` AND `test result: ` AND ` passed; ` |
| `ContentPytest`    | contains `===== ` AND (`passed` OR `failed` OR `error`) AND `pytest` (case-insensitive) |
| `ContentLS`        | ≥3 lines starting with `[d-][rwx-]{9}` OR ≥5 lines starting with `├──`/`└──`/`│   ` |
| `ContentJSON`      | `strings.TrimSpace(s)` starts with `{` or `[` AND `json.Valid([]byte(s))==true` |
| `ContentLog`       | ≥10 lines, ≥30% match `^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}` (timestamp prefix) |
| `ContentGeneric`   | fallback — no compression applied |

Compile all regexes once (package-level `var = regexp.MustCompile(...)`).

### Filter Specs

#### `git-filter.go`
- **status:** keep first `On branch` line; collapse "Changes not staged"/"Changes to be committed" sections to `Modified: a.go, b.go (3 files)`; drop hint lines starting with `  (use "git ...")`.
- **diff:** keep `diff --git`, `--- a/`, `+++ b/`, `@@` headers, and `+`/`-` lines; drop unchanged context lines (no leading `+`/`-`); drop `index abc..def` and `Binary files differ` notes by replacing with single placeholder.
- **log:** keep `commit <sha>`, `Author: ...`, `Date: ...` (compacted to one line: `commit abc1234 by Alice 2026-05-01`), drop blank lines and indented body except first 80 chars of subject.

#### `test-filter.go`
- **go test:** drop every line containing `--- PASS:` and following indented run output until next `=== `, `--- `, `FAIL`, `PASS`, or end. Keep `--- FAIL:`, `panic:` and next 10 lines, final `FAIL`/`ok` summary line.
- **cargo test:** drop lines matching `^test .* ... ok$`. Keep `failed`/`FAILED`, `test result:` summary, `running N tests`.
- **pytest:** keep lines starting with `FAILED `, `ERROR `, `===== ` (separators), and final summary line; drop the rest.

#### `ls-filter.go`
- **ls -l:** group by directory; replace runs of `[d-][rwx-]{9} ... filename` with `<dirname>/ (N files, M dirs)`.
- **tree:** for each subtree of >5 entries, print first 3 + `... (N more)` + last entry.
- **find:** dedup by directory prefix; if >10 results in same dir, print `<dir>/ (N matches)`.

#### `log-filter.go`
- Sliding-window dedup: identical adjacent lines replaced with `<line> ×N`.
- Strip ISO/RFC3339 timestamp prefixes from leading 19 chars on every line where regex matches (saves ~22 chars/line).
- If after dedup still >200 lines, keep first 50 + `... [<N> lines elided] ...` + last 50.

#### `json-filter.go`
- Parse with `json.Decoder`. Emit a structural skeleton:
  - object → `{"key1": "...", "key2": 0, "nested": {...}}`
  - array → `[{...} ×N]` with element-count summary
  - depth limit: 3 levels; truncate beyond.
- If parse fails mid-way, return original (`ok=false`).

### Message Parser (`message-parser.go`)

Walk the body via `map[string]json.RawMessage` to preserve order and unknown fields:

```go
func walkAndCompress(body []byte, c *Compressor) ([]byte, Stats) {
    var raw map[string]json.RawMessage
    if err := json.Unmarshal(body, &raw); err != nil { return body, Stats{} }
    msgsRaw, ok := raw["messages"]
    if !ok { return body, Stats{} }
    var msgs []map[string]json.RawMessage
    if err := json.Unmarshal(msgsRaw, &msgs); err != nil { return body, Stats{} }
    // for each msg: inspect role, mutate "content" if eligible
    // ...
    raw["messages"], _ = json.Marshal(msgs)
    out, err := json.Marshal(raw)
    if err != nil { return body, Stats{} }
    return out, stats
}
```

Content shapes to handle:
- `"content": "string..."` — plain string.
- `"content": [{"type":"text", "text":"..."}, ...]` — Anthropic-style content blocks; only `text` blocks rewritten.
- Any other shape → skipped silently.

## Related Code Files

### Files to create
- `internal/adapter/compressor/compressor.go`
- `internal/adapter/compressor/content-detector.go`
- `internal/adapter/compressor/message-parser.go`
- `internal/adapter/compressor/stats.go`
- `internal/adapter/compressor/filters/filter.go`
- `internal/adapter/compressor/filters/git-filter.go`
- `internal/adapter/compressor/filters/test-filter.go`
- `internal/adapter/compressor/filters/ls-filter.go`
- `internal/adapter/compressor/filters/log-filter.go`
- `internal/adapter/compressor/filters/json-filter.go`

### Files to modify
- None in this phase (package is self-contained).

## Implementation Steps

1. Create `internal/adapter/compressor/` directory.
2. Write `stats.go` (~30 lines) — pure data type.
3. Write `filters/filter.go` (~25 lines) — `type Filter func(string) (string, bool)` plus `ContentType` enum mirror.
4. Write `compressor.go` (~80 lines) — `New`, `Compress`, filter dispatch table. Use `defer recover()` around each filter call so a regex bug never panics the request thread.
5. Write `content-detector.go` (~120 lines) — package-level compiled regexes + `Detect(string) ContentType`.
6. Write `message-parser.go` (~150 lines) — `walkAndCompress` plus content-shape helpers.
7. Write each filter (`git`, `test`, `ls`, `log`, `json`) — each ≤200 lines, single-pass, no shared mutable state.
8. Run `go build ./internal/adapter/compressor/...` until clean.

## Todo

- [ ] Scaffold package directory and `filters/` subpackage.
- [ ] Define `ContentType` enum + `Filter` signature.
- [ ] Implement `Stats` + `Options` types.
- [ ] Implement detector with all 9 rules + tests inline against literal fixtures.
- [ ] Implement git filter (status / diff / log dispatched by sub-detection).
- [ ] Implement test filter (go / cargo / pytest dispatched by sub-detection).
- [ ] Implement ls filter (ls -l / tree / find dispatched by sub-detection).
- [ ] Implement log filter.
- [ ] Implement json filter.
- [ ] Implement message parser with both string and content-block shapes.
- [ ] Wire dispatcher in `compressor.go` with `recover()` guard.
- [ ] `go vet ./internal/adapter/compressor/...` clean.

## Success Criteria

- Package compiles standalone.
- `go vet` and `go build` clean.
- For every detector rule, a hand-crafted fixture flips it to the right `ContentType`.
- For every filter, `len(out) < len(in) * 0.7` on a representative fixture (git diff, go test, etc.).
- `Compress(invalidJSON)` returns the input bytes unchanged with empty Stats.
- `Compress` of a body with no `messages` key returns input unchanged.

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Filter accidentally drops semantic content | Self-validation: every filter checks invariants (e.g. test filter must keep at least one summary line); fail-open if invariant violated. |
| Regex panic on pathological input | `defer recover()` wrapper around filter dispatch in `compressor.go`. |
| JSON re-marshal reorders keys | Use `map[string]json.RawMessage` + careful re-assembly preserving original order where Go's stdlib allows; document key order is best-effort. |
| Unicode (CJK / emoji) breaks line counting | Use `bufio.Scanner` with `ScanLines`; rune-aware substring slicing where needed. |

## Security Considerations

- Compressor never logs message content (only sizes and detection counts).
- `recover()` swallows panics but logs `[compressor] recovered: <err>` to `internal/logger` — no message body in log line.
- Body must remain valid JSON post-compression; downstream `chatService.HandleChat` re-parses and would 400 on invalid JSON, defeating the proxy.

## Next Steps

- Phase 2 wires `Options` to `domain.Settings.Compression` so settings drive runtime behavior.
- Phase 5 brings the unit-test suite that exercises every fixture.

## Unresolved Questions

1. Should compression also run on the `system` message? **Recommendation:** No for v1 (system is short and prompt-engineered).
2. Should we expose `Options.AggressiveJSON` to drop array elements beyond first 3? **Recommendation:** Defer to v2 once savings telemetry is real.
3. Compress streaming responses too? **Out of scope** — this plan is request-only.
