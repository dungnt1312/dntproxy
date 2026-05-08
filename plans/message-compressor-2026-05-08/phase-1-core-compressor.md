# Phase 1 — Core Compressor Package

**Status:** pending
**Priority:** P1 (foundation for all later phases)
**Estimated:** 1–2 days

## Context Links

- Brief: [`./plan.md`](./plan.md)
- Reference style: `internal/adapter/tunnel/cloudflared.go`, `internal/adapter/storage/json-db.go`

## Overview

Build the standalone `internal/adapter/compressor/` package. Pure Go, no external deps, no other internal packages — depends only on `encoding/json`, `regexp`, `strings`, `bufio`, `bytes`.

## Requirements

### Functional
- `Compress(body []byte) ([]byte, Stats)` rewrites tool result content only when:
  - Content source is a tool result (Shape A or B — see Message Parser section), AND
  - content length ≥ `MinContentLength` (default 500), AND
  - detected ContentType ≠ `ContentGeneric` AND ≠ `ContentCodeFile`, AND
  - compression ratio < 0.85 (at least 15% savings).
- **Skip block when any of these is true:**
  - `is_error: true` on the tool result block.
  - ContentType = `ContentCodeFile`.
  - Content has base64 line: any line matching `^[A-Za-z0-9+/]{60,}={0,2}$`.
  - After filter, `len(out) > len(in) * 0.85`.
- Original body returned verbatim on any error path (JSON parse fail, regex panic, unknown structure).
- `Stats` carries: `OriginalBytes`, `CompressedBytes`, `TokensSaved` (= `(orig-comp)/4`), `Detections map[string]int`, `Skipped int`.

### Non-Functional
- Single-pass scan per filter — no nested re-tokenization.
- Total compressor latency ≤ 2ms for 100KB body on a single core.
- Each Go file ≤ 200 lines.

## Architecture

```
internal/adapter/compressor/
├── compressor.go           // New(opts), Compress(body)
├── content-detector.go     // Detect(s string) ContentType
├── message-parser.go       // walkMessages — visits tool result content only
├── stats.go                // Stats + Options structs
└── filters/
    ├── filter.go           // type Filter func(string) (string, bool)
    ├── git-filter.go
    ├── test-filter.go
    ├── ls-filter.go
    ├── log-filter.go
    └── json-filter.go
```

### Public API (`compressor.go`)

```go
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
    Skipped         int
}

func New(opts Options) *Compressor
func (c *Compressor) Compress(body []byte) ([]byte, Stats)
```

### Content Detector (`content-detector.go`)

Check in this priority order, short-circuit on first match:

| # | ContentType | Rule |
|---|-------------|------|
| 1 | `ContentCodeFile` | **skip — no compress.** ≥ 3 distinct lines starting with any of: `package `, `import `, `func `, `class `, `def `, `const `, `type ` |
| 2 | `ContentGitDiff` | starts with `diff --git ` OR (`\n@@ -` AND `\n+++ ` both present) |
| 3 | `ContentGitStatus` | `On branch ` AND (`Changes not staged` OR `nothing to commit` OR `Untracked files:`) |
| 4 | `ContentGitLog` | `^commit [a-f0-9]{40}` (multiline) matches ≥ 2 times |
| 5 | `ContentGoTest` | `--- PASS:` OR `--- FAIL:` OR `=== RUN   ` OR `^FAIL\t` (multiline) |
| 6 | `ContentCargoTest` | `running ` AND `test result: ` AND ` passed; ` |
| 7 | `ContentPytest` | `=====` AND (`passed` OR `failed` OR `error`) AND `pytest` (case-insensitive) |
| 8 | `ContentLS` | ≥ 3 lines matching `^[d-][rwx-]{9}` OR ≥ 5 lines starting with `├──` / `└──` / `│` |
| 9 | `ContentLog` | ≥ 10 lines, ≥ 30% match `^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}` |
| 10 | `ContentJSON` | trimmed starts with `{` or `[` AND `json.Valid() == true` |
| 11 | `ContentGeneric` | fallback — no compression applied |

Compile all regexes once at package level with `regexp.MustCompile`.

### Filter Specs

#### `git-filter.go`

**status input → output:**
```
# Before (many lines)
On branch main
Your branch is up to date with 'origin/main'.

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
        modified:   foo.go
        modified:   bar.go

# After
On branch main
Modified: foo.go, bar.go (2 files)
```
- Keep first `On branch` line.
- Collapse staged/unstaged sections to single `Modified: ..., ... (N files)` line.
- Drop all `(use "git ...")` hint lines and blank lines.

**diff input → output:**
```
# Before
diff --git a/foo.go b/foo.go
index abc1234..def5678 100644
--- a/foo.go
+++ b/foo.go
@@ -10,7 +10,7 @@ func Foo() {
 context line
-old line
+new line
 context line

# After
diff --git a/foo.go b/foo.go
--- a/foo.go +++ b/foo.go
@@ -10,7 +10,7 @@
-old line
+new line
```
- Keep `diff --git` line.
- Merge `--- a/` and `+++ b/` into one line.
- Keep `@@` headers, strip trailing context descriptor after `@@`.
- Keep `+` and `-` lines.
- Drop context lines (lines with no `+`/`-` prefix).
- Drop `index abc..def` lines.

**log:** compact each commit to one line: `abc1234 Alice 2026-05-01: first 80 chars of subject`.

#### `test-filter.go`

**go test input → output:**
```
# Before
=== RUN   TestFoo
=== RUN   TestFoo/sub1
--- PASS: TestFoo/sub1 (0.00s)
--- PASS: TestFoo (0.01s)
=== RUN   TestBar
--- FAIL: TestBar (0.05s)
    bar_test.go:42: expected 1 got 2
FAIL    github.com/foo/bar  0.06s

# After
--- FAIL: TestBar (0.05s)
    bar_test.go:42: expected 1 got 2
FAIL    github.com/foo/bar  0.06s
```
- Drop `=== RUN` lines.
- Drop `--- PASS:` lines and their indented output.
- Keep `--- FAIL:` + indented lines until next `=== ` / `--- ` / `FAIL` / `ok`.
- Keep final `FAIL`/`ok` summary line.

**cargo test:** drop lines matching `^test .* \.\.\. ok$`; keep `FAILED`, `test result:` summary, `running N tests`.

**pytest:** keep `FAILED `, `ERROR `, `=====` separators, and final summary; drop the rest.

#### `ls-filter.go`
- **ls -l:** replace runs of `[d-][rwx-]{9} ... filename` lines with `<dirname>/ (N files, M dirs)`.
- **tree:** for subtrees > 5 entries, print first 3 + `... (N more)` + last entry.
- **find:** dedup by directory prefix; > 10 results in same dir → `<dir>/ (N matches)`.

#### `log-filter.go`
- Sliding-window dedup: identical adjacent lines → `<line> ×N`.
- Strip ISO/RFC3339 timestamp prefix (leading 19–32 chars) on matched lines.
- If after dedup still > 200 lines, keep first 50 + `... [N lines elided] ...` + last 50.

#### `json-filter.go`
- Emit structural skeleton: object → `{"key": "...", "nested": {...}}`, array → `[{...} ×N]`.
- Depth limit: 3 levels, truncate beyond with `...`.
- If parse fails mid-way, return original (`ok=false`).

### Message Parser (`message-parser.go`)

Only visit tool result content — two shapes:

**Shape A — OpenAI tool role:**
```json
{"role": "tool", "tool_call_id": "...", "content": "string output"}
```
Target: `msg["content"]` string.

**Shape B — user message wrapping tool_result block:**
```json
{
  "role": "user",
  "content": [
    {"type": "tool_result", "tool_use_id": "...", "is_error": false, "content": "string output"}
  ]
}
```
Or content as array of text blocks:
```json
{"type": "tool_result", "content": [{"type": "text", "text": "string output"}]}
```

**Skip rules (checked before compress):**
1. Shape B block has `is_error == true` → skip block.
2. ContentType == `ContentCodeFile` → skip.
3. Content has base64 line → skip.
4. After filter, ratio > 0.85 → return original.

Implementation skeleton:
```go
func walkAndCompress(body []byte, c *Compressor) ([]byte, Stats) {
    var raw map[string]json.RawMessage
    if err := json.Unmarshal(body, &raw); err != nil { return body, Stats{} }
    msgsRaw, ok := raw["messages"]
    if !ok { return body, Stats{} }
    var msgs []map[string]json.RawMessage
    if err := json.Unmarshal(msgsRaw, &msgs); err != nil { return body, Stats{} }

    var stats Stats
    for i, msg := range msgs {
        var role string
        _ = json.Unmarshal(msg["role"], &role)
        switch role {
        case "tool":
            msgs[i], stats = compressShapeA(msg, stats, c)
        case "user":
            msgs[i], stats = compressShapeB(msg, stats, c)
        }
    }

    raw["messages"], _ = json.Marshal(msgs)
    out, err := json.Marshal(raw)
    if err != nil { return body, Stats{} }
    return out, stats
}
```

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

1. Create `internal/adapter/compressor/` and `filters/` directories.
2. Write `stats.go` — `Stats` + `Options` structs.
3. Write `filters/filter.go` — `type Filter func(string) (string, bool)` + `ContentType` enum (11 values).
4. Write `content-detector.go` — 11 rules in priority order, all regexes compiled at package level.
5. Write `compressor.go` — `New`, `Compress`, filter dispatch table, `defer recover()` per filter call.
6. Write `message-parser.go` — `walkAndCompress` + `compressShapeA` + `compressShapeB` + all skip logic.
7. Write `filters/git-filter.go` — status / diff / log sub-detection.
8. Write `filters/test-filter.go` — go test / cargo test / pytest sub-detection.
9. Write `filters/ls-filter.go` — ls -l / tree / find sub-detection.
10. Write `filters/log-filter.go`.
11. Write `filters/json-filter.go`.
12. Run `go build ./internal/adapter/compressor/...` until clean.

## Todo

- [ ] Create package directories.
- [ ] `stats.go` — `Stats` (with `Skipped int`) + `Options`.
- [ ] `filters/filter.go` — `Filter` type + `ContentType` enum (11 values, `ContentCodeFile` first).
- [ ] `content-detector.go` — all 11 rules, `ContentCodeFile` at priority 1.
- [ ] `compressor.go` — dispatch + `recover()` guard.
- [ ] `message-parser.go` — Shape A, Shape B, `is_error` skip, base64 skip, ratio gate.
- [ ] `filters/git-filter.go` — status, diff, log.
- [ ] `filters/test-filter.go` — go test, cargo test, pytest.
- [ ] `filters/ls-filter.go` — ls, tree, find.
- [ ] `filters/log-filter.go` — dedup + truncate.
- [ ] `filters/json-filter.go` — skeleton output.
- [ ] `go vet ./internal/adapter/compressor/...` clean.

## Success Criteria

- Package compiles standalone, `go vet` clean.
- `ContentCodeFile` fires on Go/Python/TS source fixture → no compression.
- `is_error: true` tool result → passes through unchanged.
- Each detector rule has a fixture that resolves to the correct `ContentType`.
- Each filter: `len(out) < len(in) * 0.80` on representative fixture.
- `Compress(invalidJSON)` → input unchanged, empty Stats.
- `Compress` on body with no `messages` key → input unchanged.
- Ratio gate: if filter produces > 85% of original size → original returned.

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Filter drops semantic content (source code) | `ContentCodeFile` detection bails before any filter runs. |
| Filter drops error details | `is_error: true` skip rule, never compress error outputs. |
| Base64/binary output corrupted | Base64 line heuristic skips entire block. |
| Regex panic on pathological input | `defer recover()` in `compressor.go` dispatch. |
| JSON re-marshal reorders keys | `map[string]json.RawMessage` preserves most keys; document as best-effort. |
| Unicode breaks line counting | `bufio.Scanner` with `ScanLines` handles all line endings correctly. |

## Security Considerations

- Compressor never logs message content — only sizes and detection type counts.
- `recover()` logs `[compressor] recovered: <err>` with no message body.
- Output must be valid JSON; invalid output would cause downstream 400 errors.

## Next Steps

- Phase 2: wire `Options` into `domain.Settings` so the feature is runtime-configurable.
- Phase 5: unit-test suite with fixtures for every detector rule and filter.

## Unresolved Questions

1. Should `system` messages be compressed? **No for v1** — system prompts are short and prompt-engineered.
2. `Options.AggressiveJSON` (drop array elements > 3)? **Defer to v2** — wait for real savings telemetry.
3. Compress streaming responses? **Out of scope** — request-side only.
