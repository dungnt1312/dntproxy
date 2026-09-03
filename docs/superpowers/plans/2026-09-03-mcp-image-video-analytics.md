# MCP Image/Video Analytics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose an MCP Streamable HTTP server at `/mcp` with `analyze_image` and `analyze_video` tools that route vision analysis through dntproxy's existing chat pipeline (multi-account fallback, cooldown, logging).

**Architecture:** MCP layer is a new client of `port.ChatService` — the executor downloads media, turns video into evenly spaced JPEG frames via ffmpeg, builds an OpenAI chat body with `image_url` parts, calls `HandleChat`, and aggregates the returned OpenAI SSE stream into text. Settings gains an `mcp` block (default vision model + max frames); the dashboard Settings screen gains an "MCP Analytics" card.

**Tech Stack:** Go 1.25, Gin, `github.com/modelcontextprotocol/go-sdk/mcp` (official SDK), `github.com/ulikunitz/xz` (linux tar.xz extraction), ffmpeg/ffprobe binaries (auto-downloaded, cached in `~/.dntproxy/bin/`), React + shadcn UI.

**Spec:** `docs/superpowers/specs/2026-09-03-mcp-image-video-analytics-design.md`

## Global Constraints

- Files kebab-case, keep under ~200 lines; domain types have no external deps; ports define interfaces, adapters implement them.
- `/mcp` always requires an API key (`Authorization: Bearer <key>`), same as `/v1/*` and `/api/*`.
- Limits (exact values): image download max **20 MB**; video download max **150 MB** with **2 minute** timeout; image download timeout **30 s**; frames clamp **1–16**, default **8**; frame scale ≤ **1280 px** width; JPEG quality `-q:v 4`.
- Model resolution order: tool `model` param → settings `mcp.defaultModel` → tool error `"No default vision model configured — set it in Settings → MCP Analytics"`.
- Chat body always sets `"stream": true`; executor aggregates OpenAI SSE via the shared helper.
- requestID prefix for MCP calls: `"mcp-"` + UUID.
- Media is always inlined as base64 data URLs (never pass remote URLs to providers).
- Tool errors return `*mcp.CallToolResult{IsError: true}` with an actionable message, not Go errors.
- Commands run from repo root `C:\Users\DungNguyen\workspace\dntproxy` (Git Bash on Windows).

---

### Task 1: Move SSE aggregation to `adapter/shared`

**Files:**
- Create: `internal/adapter/shared/aggregate.go`
- Create: `internal/adapter/shared/aggregate_test.go`
- Modify: `internal/adapter/http/chat-handler.go` (delete lines 257–425: aggregation types + `aggregateChatCompletion`; update the call at ~line 86)
- Modify: `internal/adapter/http/chat-handler_test.go` (move aggregate tests out)

**Interfaces:**
- Consumes: nothing new.
- Produces: `shared.AggregateChatCompletion(stream io.Reader, requestedModel, requestID string) (shared.ChatCompletionResponse, error)`; `shared.ChatCompletionResponse` with field `Choices []ChatCompletionChoice`, choice field `Message ChatCompletionMessage`, message field `Content string`. Task 6 consumes all of these.

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/shared/aggregate_test.go` — move the 4 test functions currently in `internal/adapter/http/chat-handler_test.go` (they call `aggregateChatCompletion`), renaming:
- package `http` → `shared`
- `aggregateChatCompletion(` → `shared.AggregateChatCompletion(` — inside package shared, drop the `shared.` prefix: `AggregateChatCompletion(`
- `chatCompletionResponse` → `ChatCompletionResponse`

First test after the move (the others follow the same rename):

```go
package shared

import (
	"strings"
	"testing"
)

func TestAggregateBasicContent(t *testing.T) {
	stream := strings.NewReader("data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hel\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n")

	completion, err := AggregateChatCompletion(stream, "fallback-model", "req-1")
	if err != nil {
		t.Fatalf("AggregateChatCompletion returned error: %v", err)
	}
	if got := completion.Choices[0].Message.Content; got != "Hello" {
		t.Fatalf("content = %q, want %q", got, "Hello")
	}
	if completion.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish_reason = %q, want stop", completion.Choices[0].FinishReason)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/shared/ -run TestAggregate -v`
Expected: FAIL — `undefined: AggregateChatCompletion`

- [ ] **Step 3: Write the implementation**

Create `internal/adapter/shared/aggregate.go` — copy the body of `aggregateChatCompletion` plus its supporting types (`chatCompletionResponse`, `chatCompletionChoice`, `chatCompletionMessage`, `chatToolCallEntry`, `chatToolCallFn`, `chatCompletionUsage`, `streamingChatChunk`) from `internal/adapter/http/chat-handler.go:257-425`, exporting names and replacing the scan buffer size with a local const:

```go
package shared

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// maxAggregateScanToken bounds a single SSE line while aggregating.
const maxAggregateScanToken = 10 * 1024 * 1024

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *ChatCompletionUsage   `json:"usage,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason,omitempty"`
}

type ChatCompletionMessage struct {
	Role      string              `json:"role"`
	Content   string              `json:"content"`
	ToolCalls []ChatToolCallEntry `json:"tool_calls,omitempty"`
}

type ChatToolCallEntry struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function ChatToolCallFn `json:"function"`
}

type ChatToolCallFn struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamingChatChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id,omitempty"`
				Type     string `json:"type,omitempty"`
				Function struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *ChatCompletionUsage `json:"usage,omitempty"`
}

// AggregateChatCompletion reads an OpenAI-compatible SSE stream and assembles
// a single non-streaming chat completion response.
func AggregateChatCompletion(stream io.Reader, requestedModel, requestID string) (ChatCompletionResponse, error) {
	response := ChatCompletionResponse{
		ID:      "chatcmpl-" + requestID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []ChatCompletionChoice{{Index: 0, Message: ChatCompletionMessage{Role: "assistant"}}},
	}

	// Use a map of index → pointer into a separate slice to avoid dangling pointers on append.
	choiceList := []*ChatCompletionChoice{&response.Choices[0]}
	choiceMap := map[int]*ChatCompletionChoice{0: choiceList[0]}

	scanner := bufio.NewScanner(stream)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxAggregateScanToken)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var chunk streamingChatChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return ChatCompletionResponse{}, err
		}
		if chunk.ID != "" {
			response.ID = chunk.ID
		}
		if chunk.Created != 0 {
			response.Created = chunk.Created
		}
		if chunk.Model != "" {
			response.Model = chunk.Model
		}
		if chunk.Usage != nil {
			response.Usage = chunk.Usage
		}

		for _, chunkChoice := range chunk.Choices {
			choice := choiceMap[chunkChoice.Index]
			if choice == nil {
				newChoice := &ChatCompletionChoice{
					Index:   chunkChoice.Index,
					Message: ChatCompletionMessage{Role: "assistant"},
				}
				choiceList = append(choiceList, newChoice)
				choiceMap[chunkChoice.Index] = newChoice
			}
			choice = choiceMap[chunkChoice.Index]

			if chunkChoice.Delta.Role != "" {
				choice.Message.Role = chunkChoice.Delta.Role
			}
			if chunkChoice.Delta.Content != "" {
				choice.Message.Content += chunkChoice.Delta.Content
			}

			for _, tc := range chunkChoice.Delta.ToolCalls {
				idx := tc.Index
				for len(choice.Message.ToolCalls) <= idx {
					choice.Message.ToolCalls = append(choice.Message.ToolCalls, ChatToolCallEntry{Type: "function"})
				}
				dest := &choice.Message.ToolCalls[idx]
				if tc.ID != "" {
					dest.ID = tc.ID
				}
				if tc.Type != "" {
					dest.Type = tc.Type
				}
				if tc.Function.Name != "" {
					dest.Function.Name = tc.Function.Name
				}
				dest.Function.Arguments += tc.Function.Arguments
			}

			if chunkChoice.FinishReason != nil {
				choice.FinishReason = *chunkChoice.FinishReason
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ChatCompletionResponse{}, err
	}

	// Rebuild response.Choices from choiceList (avoids dangling pointer issue)
	response.Choices = make([]ChatCompletionChoice, len(choiceList))
	for i, cp := range choiceList {
		response.Choices[i] = *cp
	}

	return response, nil
}
```

- [ ] **Step 4: Update `chat-handler.go` and its test**

In `internal/adapter/http/chat-handler.go`: delete the aggregation types and `aggregateChatCompletion` (lines 257–425), and change the call site to:

```go
completion, err := shared.AggregateChatCompletion(result.Stream, partial.Model, requestID)
```

`shared` is already imported in this file's package via other files — add `"github.com/dungnt/dntproxy/internal/adapter/shared"` to chat-handler.go imports if missing. `maxChatBodySize` stays in the http package (messages-handler.go still uses it). In `chat-handler_test.go` delete the 4 moved test functions (and now-unused imports if any remain unused).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/adapter/shared/ ./internal/adapter/http/ -v`
Expected: PASS (aggregate tests green in shared, http tests green after removal)

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/shared/aggregate.go internal/adapter/shared/aggregate_test.go internal/adapter/http/chat-handler.go internal/adapter/http/chat-handler_test.go
git commit -m "refactor: move OpenAI SSE aggregation to adapter/shared"
```

---

### Task 2: Settings — MCP defaults in domain + settings API

**Files:**
- Modify: `internal/domain/config.go` (Settings struct + new MCPSettings type + Normalize)
- Create: `internal/domain/settings-mcp_test.go`
- Modify: `internal/adapter/http/settings-handler.go` (request/view structs + update mapping)

**Interfaces:**
- Consumes: existing `domain.Settings`, `settingsUpdateRequest`/`settingsAPIView`.
- Produces: `domain.Settings.MCP domain.MCPSettings` (json `"mcp"`); `domain.MCPSettings{DefaultModel string `json:"defaultModel,omitempty"`, VideoMaxFrames int `json:"videoMaxFrames,omitempty"`}` with `(*MCPSettings).Normalize()` clamping VideoMaxFrames to default 8, max 16. Tasks 6, 7, 9 consume these. Settings API request/view gain field `MCP *domain.MCPSettings` / `MCP domain.MCPSettings` (json `"mcp"`).

- [ ] **Step 1: Write the failing test**

Create `internal/domain/settings-mcp_test.go`:

```go
package domain

import "testing"

func TestMCPSettingsNormalize(t *testing.T) {
	m := MCPSettings{}
	m.Normalize()
	if m.VideoMaxFrames != 8 {
		t.Fatalf("VideoMaxFrames = %d, want default 8", m.VideoMaxFrames)
	}

	m = MCPSettings{VideoMaxFrames: 99, DefaultModel: "glm/glm-4.5v"}
	m.Normalize()
	if m.VideoMaxFrames != 16 {
		t.Fatalf("VideoMaxFrames = %d, want clamp 16", m.VideoMaxFrames)
	}

	m = MCPSettings{VideoMaxFrames: 3}
	m.Normalize()
	if m.VideoMaxFrames != 3 || m.DefaultModel != "" {
		t.Fatalf("valid values must be kept: %+v", m)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/ -run TestMCPSettingsNormalize -v`
Expected: FAIL — `undefined: MCPSettings`

- [ ] **Step 3: Implement domain changes**

In `internal/domain/config.go`, add below `CompressionSettings` (after its `Normalize`, around line 31):

```go
// MCPSettings configures the MCP analytics server (/mcp).
type MCPSettings struct {
	DefaultModel   string `json:"defaultModel,omitempty"`
	VideoMaxFrames int    `json:"videoMaxFrames,omitempty"` // default 8, clamped 1..16
}

// Normalize sets safe defaults for zero values loaded from JSON.
func (m *MCPSettings) Normalize() {
	if m.VideoMaxFrames <= 0 {
		m.VideoMaxFrames = 8
	}
	if m.VideoMaxFrames > 16 {
		m.VideoMaxFrames = 16
	}
}
```

In `domain.Settings` (after the Compression field, ~line 54):

```go
	// MCP analytics server settings (/mcp)
	MCP MCPSettings `json:"mcp,omitempty"`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/ -run TestMCPSettingsNormalize -v`
Expected: PASS

- [ ] **Step 5: Wire the settings API**

In `internal/adapter/http/settings-handler.go`:

1. `apiGetSettings` view literal — add after `Compression: s.Compression,`:

```go
			MCP:                    s.MCP,
```

2. `apiUpdateSettings` inside `store.Update`, after `cfg.Settings.Compression.Normalize()`:

```go
			if req.MCP != nil {
				cfg.Settings.MCP = *req.MCP
				cfg.Settings.MCP.Normalize()
			}
```

and in the response literal after `Compression: updated.Compression,`:

```go
			MCP:                    updated.MCP,
```

3. Structs — add to `settingsAPIView`:

```go
	MCP domain.MCPSettings `json:"mcp"`
```

and to `settingsUpdateRequest` (pointer so a missing field keeps current settings):

```go
	MCP *domain.MCPSettings `json:"mcp,omitempty"`
```

- [ ] **Step 6: Run all tests and commit**

Run: `go test ./internal/domain/ ./internal/adapter/http/ && go build ./...`
Expected: PASS / builds clean

```bash
git add internal/domain/config.go internal/domain/settings-mcp_test.go internal/adapter/http/settings-handler.go
git commit -m "feat: mcp settings block (default vision model, max video frames)"
```

---

### Task 3: ffmpeg binary locator/downloader (`adapter/ffmpeg`)

**Files:**
- Create: `internal/adapter/ffmpeg/ensure.go`
- Create: `internal/adapter/ffmpeg/download.go`
- Create: `internal/adapter/ffmpeg/ensure_test.go`
- Modify: `go.mod` via `go get github.com/ulikunitz/xz`

**Interfaces:**
- Consumes: nothing (stdlib + xz).
- Produces: `ffmpeg.Locate(ctx context.Context) (ffmpegPath, ffprobePath string, err error)`. Task 6 consumes this. Pure helpers exported for tests: `ffmpeg.DownloadURL(goos, goarch string) (ffmpegURL, ffprobeURL string)`.

- [ ] **Step 1: Add dependency**

```bash
go get github.com/ulikunitz/xz@latest
```

- [ ] **Step 2: Write the failing test**

Create `internal/adapter/ffmpeg/ensure_test.go`:

```go
package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDownloadURLPlatforms(t *testing.T) {
	cases := []struct {
		goos, goarch       string
		wantFF, wantProbe  string
	}{
		{"linux", "amd64", "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz", ""},
		{"linux", "arm64", "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linuxarm64-gpl.tar.xz", ""},
		{"windows", "amd64", "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip", ""},
		{"darwin", "amd64", "https://evermeet.cx/ffmpeg/getrelease/ffmpeg/zip", "https://evermeet.cx/ffmpeg/getrelease/ffprobe/zip"},
		{"darwin", "arm64", "", ""},
		{"solaris", "amd64", "", ""},
	}
	for _, tc := range cases {
		ff, probe := DownloadURL(tc.goos, tc.goarch)
		if ff != tc.wantFF || probe != tc.wantProbe {
			t.Errorf("%s/%s = (%q, %q), want (%q, %q)", tc.goos, tc.goarch, ff, probe, tc.wantFF, tc.wantProbe)
		}
	}
}

func TestLocateEnvOverride(t *testing.T) {
	dir := t.TempDir()
	ff := filepath.Join(dir, "ffmpeg")
	probe := filepath.Join(dir, "ffprobe")
	for _, p := range []string{ff, probe} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DNTPROXY_FFMPEG_PATH", ff)
	t.Setenv("DNTPROXY_FFPROBE_PATH", probe)

	gotFF, gotProbe, err := Locate(context.Background())
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if gotFF != ff || gotProbe != probe {
		t.Fatalf("Locate = (%q, %q), want overrides", gotFF, gotProbe)
	}
}

func TestLocateUsesPathBeforeDownload(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("no system ffmpeg; PATH branch not testable")
	}
	ff, probe, err := Locate(context.Background())
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if ff == "" || probe == "" {
		t.Fatalf("expected both paths, got (%q, %q)", ff, probe)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/adapter/ffmpeg/ -v`
Expected: FAIL — package has no Go files / `undefined: DownloadURL`, `undefined: Locate`

- [ ] **Step 4: Implement `ensure.go`**

Create `internal/adapter/ffmpeg/ensure.go`:

```go
// Package ffmpeg locates or auto-downloads ffmpeg/ffprobe binaries, mirroring
// the tunnel adapter's cloudflared lifecycle (system PATH → env override →
// cached ~/.dntproxy/bin/ → platform download).
package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Env overrides for pre-installed binaries.
const (
	EnvFFmpegPath  = "DNTPROXY_FFMPEG_PATH"
	EnvFFprobePath = "DNTPROXY_FFPROBE_PATH"
)

// Locate returns executable paths for ffmpeg and ffprobe. Resolution order
// per binary: env override → system PATH → cached ~/.dntproxy/bin/ → download.
func Locate(ctx context.Context) (string, string, error) {
	ff, err := resolveOne(ctx, EnvFFmpegPath, "ffmpeg", binaryName("ffmpeg"))
	if err != nil {
		return "", "", fmt.Errorf("ffmpeg: %w", err)
	}
	probe, err := resolveOne(ctx, EnvFFprobePath, "ffprobe", binaryName("ffprobe"))
	if err != nil {
		return "", "", fmt.Errorf("ffprobe: %w", err)
	}
	return ff, probe, nil
}

func resolveOne(ctx context.Context, envKey, lookPath, cachedName string) (string, error) {
	if p := os.Getenv(envKey); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("%s=%q not found", envKey, p)
	}
	if p, err := exec.LookPath(lookPath); err == nil {
		return p, nil
	}

	cached := filepath.Join(cacheDir(), cachedName)
	if info, err := os.Stat(cached); err == nil && info.Size() > 1024 {
		return cached, nil
	}

	if err := downloadAll(ctx); err != nil {
		return "", fmt.Errorf("auto-download failed (or install ffmpeg and retry): %w", err)
	}
	if info, err := os.Stat(cached); err == nil && info.Size() > 1024 {
		return cached, nil
	}
	return "", fmt.Errorf("download completed but %s is missing", cached)
}

func binaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func cacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".dntproxy", "bin")
}

// downloadAll fetches the platform bundle once; single-binary bundles
// (darwin) download ffmpeg and ffprobe separately.
func downloadAll(ctx context.Context) error {
	ffURL, probeURL := DownloadURL(runtime.GOOS, runtime.GOARCH)
	if ffURL == "" {
		return fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if err := fetchBundle(ctx, ffURL); err != nil {
		return err
	}
	if probeURL != "" {
		if err := fetchBundle(ctx, probeURL); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 5: Implement `download.go`**

Create `internal/adapter/ffmpeg/download.go`:

```go
package ffmpeg

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

// fetchBundle downloads one archive (zip or tar.xz) into the cache dir.
func fetchBundle(ctx context.Context, url string) error {
	fmt.Printf("[ffmpeg] Downloading %s\n", url)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 300<<20))
	if err != nil {
		return fmt.Errorf("read download: %w", err)
	}

	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return err
	}

	switch {
	case strings.HasSuffix(url, ".zip"):
		return extractZip(body)
	case strings.HasSuffix(url, ".tar.xz"):
		return extractTarXz(body)
	default:
		return fmt.Errorf("unsupported archive format: %s", url)
	}
}

func extractZip(body []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}
	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if base != "ffmpeg.exe" && base != "ffprobe.exe" && base != "ffmpeg" && base != "ffprobe" {
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if err := writeFileFromZip(zr, f, filepath.Join(cacheDir(), base)); err != nil {
			return err
		}
		fmt.Printf("[ffmpeg] Extracted %s\n", base)
	}
	return verifyExtracted()
}

func writeFileFromZip(zr *zip.Reader, f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return writeExecutable(target, rc)
}

func extractTarXz(body []byte) error {
	xr, err := xz.NewReader(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("xz: %w", err)
	}
	tr := tar.NewReader(xr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		base := filepath.Base(hdr.Name)
		if base != "ffmpeg" && base != "ffprobe" {
			continue
		}
		if err := writeExecutable(filepath.Join(cacheDir(), base), tr); err != nil {
			return err
		}
		fmt.Printf("[ffmpeg] Extracted %s\n", base)
	}
	return verifyExtracted()
}

func writeExecutable(target string, r io.Reader) error {
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func verifyExtracted() error {
	for _, name := range []string{binaryName("ffmpeg"), binaryName("ffprobe")} {
		p := filepath.Join(cacheDir(), name)
		if info, err := os.Stat(p); err != nil || info.Size() < 1024 {
			return fmt.Errorf("extraction incomplete: %s missing from archive", name)
		}
	}
	return nil
}

// DownloadURL returns archive URLs for the platform. linux uses a combined
// BtbN tar.xz (ffprobeURL empty); windows uses a gyan.dev zip (both binaries
// inside); darwin uses evermeet.cx single-binary zips. Empty ffmpegURL means
// unsupported platform.
func DownloadURL(goos, goarch string) (ffmpegURL, ffprobeURL string) {
	if goarch != "amd64" && goarch != "arm64" {
		return "", ""
	}
	switch goos {
	case "linux":
		if goarch == "amd64" {
			return "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz", ""
		}
		return "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linuxarm64-gpl.tar.xz", ""
	case "windows":
		if goarch == "amd64" {
			return "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip", ""
		}
		return "", ""
	case "darwin":
		if goarch == "amd64" {
			return "https://evermeet.cx/ffmpeg/getrelease/ffmpeg/zip", "https://evermeet.cx/ffmpeg/getrelease/ffprobe/zip"
		}
		return "", ""
	default:
		return "", ""
	}
}
```

Note: `download.go` needs `"archive/tar"` in imports (used by `extractTarXz`).

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/adapter/ffmpeg/ -v && go build ./...`
Expected: PASS (env-override and URL-matrix tests green; PATH test skipped unless ffmpeg installed)

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/ffmpeg/ go.mod go.sum
git commit -m "feat: ffmpeg/ffprobe binary locator with platform auto-download"
```

---

### Task 4: Frame extraction (`adapter/ffmpeg`)

**Files:**
- Create: `internal/adapter/ffmpeg/extract.go`
- Create: `internal/adapter/ffmpeg/extract_test.go`

**Interfaces:**
- Consumes: `Locate` from Task 3 (only in tests).
- Produces:
  - `ffmpeg.FrameTimestamps(duration time.Duration, maxFrames int) []time.Duration` (pure)
  - `ffmpeg.ExtractFrames(ctx context.Context, ffmpegPath, ffprobePath, videoPath, dir string, maxFrames int) ([]string, error)` — returns JPEG paths in chronological order.
  Task 6 wraps these behind its injectable `extractVideo` func.

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/ffmpeg/extract_test.go`:

```go
package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFrameTimestamps(t *testing.T) {
	got := FrameTimestamps(10*time.Second, 4)
	want := []time.Duration{1250 * time.Millisecond, 3750 * time.Millisecond, 6250 * time.Millisecond, 8750 * time.Millisecond}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FrameTimestamps[%d] = %v, want %v (all: %v)", i, got[i], want[i], got)
		}
	}

	if ts := FrameTimestamps(0, 4); len(ts) != 1 {
		t.Fatalf("zero duration should yield single frame at 0, got %v", ts)
	}
}

// hasFFmpeg reports whether a real ffmpeg is available for integration tests.
func hasFFmpeg(t *testing.T) (string, string) {
	t.Helper()
	ff, probe, err := Locate(context.Background())
	if err != nil {
		t.Skip("ffmpeg/ffprobe not available:", err)
	}
	return ff, probe
}

func TestExtractFramesIntegration(t *testing.T) {
	ff, probe := hasFFmpeg(t)
	ctx := context.Background()

	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.mp4")
	gen := exec.CommandContext(ctx, ff,
		"-f", "lavfi", "-i", "testsrc=duration=2:size=320x240:rate=10",
		"-y", fixture)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot generate fixture: %v\n%s", err, out)
	}

	framesDir := t.TempDir()
	frames, err := ExtractFrames(ctx, ff, probe, fixture, framesDir, 4)
	if err != nil {
		t.Fatalf("ExtractFrames: %v", err)
	}
	if len(frames) != 4 {
		t.Fatalf("got %d frames, want 4: %v", len(frames), frames)
	}
	for i, f := range frames {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read frame %d: %v", i, err)
		}
		if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
			t.Fatalf("frame %d is not JPEG: %s (%d bytes)", i, f, len(data))
		}
	}
}

func TestExtractFramesDownscales(t *testing.T) {
	ff, probe := hasFFmpeg(t)
	ctx := context.Background()
	dir := t.TempDir()
	fixture := filepath.Join(dir, "wide.mp4")
	gen := exec.CommandContext(ctx, ff,
		"-f", "lavfi", "-i", "testsrc=duration=1:size=1920x1080:rate=10",
		"-y", fixture)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot generate fixture: %v\n%s", err, out)
	}
	frames, err := ExtractFrames(ctx, ff, probe, fixture, t.TempDir(), 1)
	if err != nil {
		t.Fatalf("ExtractFrames: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/ffmpeg/ -run "TestFrame" -v`
Expected: FAIL — `undefined: FrameTimestamps`, `undefined: ExtractFrames`

- [ ] **Step 3: Implement `extract.go`**

Create `internal/adapter/ffmpeg/extract.go`:

```go
package ffmpeg

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FrameTimestamps returns evenly spaced sample points across duration,
// offset by half a step so frames avoid the exact boundaries. Durations <= 0
// yield a single frame at 0.
func FrameTimestamps(duration time.Duration, maxFrames int) []time.Duration {
	if maxFrames < 1 {
		maxFrames = 1
	}
	if duration <= 0 {
		return []time.Duration{0}
	}
	step := duration / time.Duration(maxFrames)
	ts := make([]time.Duration, 0, maxFrames)
	for i := 0; i < maxFrames; i++ {
		ts = append(ts, step*time.Duration(i)+step/2)
	}
	return ts
}

// ExtractFrames extracts up to maxFrames evenly spaced JPEG frames from
// videoPath into dir (which must exist), returning frame paths in
// chronological order. Frames whose extraction fails are skipped; an error
// is returned only when no frame could be extracted or probing fails.
func ExtractFrames(ctx context.Context, ffmpegPath, ffprobePath, videoPath, dir string, maxFrames int) ([]string, error) {
	duration, err := probeDuration(ctx, ffprobePath, videoPath)
	if err != nil {
		return nil, err
	}

	timestamps := FrameTimestamps(duration, maxFrames)
	frames := make([]string, 0, len(timestamps))
	for i, ts := range timestamps {
		out := filepath.Join(dir, fmt.Sprintf("frame-%03d.jpg", i))
		args := []string{
			"-hide_banner", "-loglevel", "error",
			"-ss", formatSeconds(ts),
			"-i", videoPath,
			"-frames:v", "1",
			"-vf", "scale='min(1280,iw)':-2",
			"-q:v", "4",
			"-y", out,
		}
		cmd := exec.CommandContext(ctx, ffmpegPath, args...)
		if outBytes, err := cmd.CombinedOutput(); err != nil {
			// Likely past EOF for short videos — skip and keep going.
			fmt.Printf("[ffmpeg] frame %d at %v failed: %v\n%s\n", i, ts, err, outBytes)
			continue
		}
		if info, err := os.Stat(out); err == nil && info.Size() > 0 {
			frames = append(frames, out)
		}
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames extracted from %s", videoPath)
	}
	return frames, nil
}

func probeDuration(ctx context.Context, ffprobePath, videoPath string) (time.Duration, error) {
	out, err := exec.CommandContext(ctx, ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration: %w", err)
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || math.IsNaN(seconds) || seconds <= 0 {
		return 0, fmt.Errorf("ffprobe returned unusable duration %q", strings.TrimSpace(string(out)))
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func formatSeconds(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/ffmpeg/ -v`
Expected: PASS (integration tests run when ffmpeg is on PATH or cached, otherwise skip)

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/ffmpeg/extract.go internal/adapter/ffmpeg/extract_test.go
git commit -m "feat: even-spaced JPEG frame extraction via ffmpeg"
```

---

### Task 5: Media download (`adapter/mcp`)

**Files:**
- Create: `internal/adapter/mcp/media.go`
- Create: `internal/adapter/mcp/media_test.go`

**Interfaces:**
- Consumes: nothing (stdlib).
- Produces (package `mcp`):
  - `const MaxImageBytes = 20 << 20`, `const MaxVideoBytes = 150 << 20`
  - `FetchImageAsDataURL(ctx context.Context, client *http.Client, rawURL string) (string, error)`
  - `DownloadVideo(ctx context.Context, client *http.Client, rawURL string) (path string, cleanup func(), err error)`
  Task 6 consumes all.

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/mcp/media_test.go`:

```go
package mcp

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var pngBytes = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

func TestFetchImageAsDataURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer srv.Close()

	got, err := FetchImageAsDataURL(context.Background(), srv.Client(), srv.URL+"/img.png")
	if err != nil {
		t.Fatalf("FetchImageAsDataURL: %v", err)
	}
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
	if got != want {
		t.Fatalf("got %q... want %q...", got[:40], want[:40])
	}
}

func TestFetchImageDataURLPassthrough(t *testing.T) {
	raw := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
	got, err := FetchImageAsDataURL(context.Background(), http.DefaultClient, raw)
	if err != nil || got != raw {
		t.Fatalf("passthrough broken: %q, %v", got, err)
	}
}

func TestFetchImageRejectsBadScheme(t *testing.T) {
	if _, err := FetchImageAsDataURL(context.Background(), http.DefaultClient, "file:///etc/passwd"); err == nil {
		t.Fatal("file:// scheme must be rejected")
	}
}

func TestFetchImageRejectsOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, MaxImageBytes+1))
	}))
	defer srv.Close()

	if _, err := FetchImageAsDataURL(context.Background(), srv.Client(), srv.URL+"/big.png"); err == nil {
		t.Fatal("oversized image must be rejected")
	}
}

func TestDownloadVideo(t *testing.T) {
	payload := strings.Repeat("MPEGDATA", 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	path, cleanup, err := DownloadVideo(context.Background(), srv.Client(), srv.URL+"/v.mp4")
	if err != nil {
		t.Fatalf("DownloadVideo: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}
	if filepath.Ext(path) != ".mp4" {
		t.Fatalf("ext = %q, want .mp4", filepath.Ext(path))
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cleanup must remove the downloaded file")
	}
}

func TestDownloadVideoRejectsOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, MaxVideoBytes+1))
	}))
	defer srv.Close()

	_, cleanup, err := DownloadVideo(context.Background(), srv.Client(), srv.URL+"/huge.mp4")
	if err == nil {
		cleanup()
		t.Fatal("oversized video must be rejected")
	}
}

func TestDownloadVideoTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, cleanup, err := DownloadVideo(ctx, srv.Client(), srv.URL+"/slow.mp4"); err == nil {
		cleanup()
		t.Fatal("context deadline must abort the download")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/mcp/ -v`
Expected: FAIL — `undefined: FetchImageAsDataURL` etc.

- [ ] **Step 3: Implement `media.go`**

Create `internal/adapter/mcp/media.go`:

```go
// Package mcp exposes dntproxy's vision-capable providers as an MCP
// (Model Context Protocol) server with image/video analysis tools.
package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Media size and time limits (see the design spec).
const (
	MaxImageBytes        = 20 << 20 // 20 MB
	MaxVideoBytes        = 150 << 20 // 150 MB
	imageDownloadTimeout = 30 * time.Second
	videoDownloadTimeout = 2 * time.Minute
)

// FetchImageAsDataURL downloads an image over http/https and returns it as a
// base64 data URL. data: URIs pass through unchanged.
func FetchImageAsDataURL(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	if strings.HasPrefix(rawURL, "data:") {
		return rawURL, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("image_url must be http(s) or a data: URI")
	}

	ctx, cancel := context.WithTimeout(ctx, imageDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download image: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	if len(data) > MaxImageBytes {
		return "", fmt.Errorf("image exceeds %d MB limit", MaxImageBytes/(1<<20))
	}

	mime := resp.Header.Get("Content-Type")
	if mime == "" || !strings.HasPrefix(mime, "image/") {
		mime = http.DetectContentType(data)
	}
	mime, _, _ = strings.Cut(mime, ";")
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// DownloadVideo downloads a video over http/https into a temp file, returning
// its path and a cleanup func that removes the file. The caller MUST call
// cleanup.
func DownloadVideo(ctx context.Context, client *http.Client, rawURL string) (string, func(), error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", nil, fmt.Errorf("video_url must be http(s)")
	}

	ctx, cancel := context.WithTimeout(ctx, videoDownloadTimeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		cancel()
		return "", nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return "", nil, fmt.Errorf("download video: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cancel()
		return "", nil, fmt.Errorf("download video: HTTP %d", resp.StatusCode)
	}

	ext := strings.ToLower(filepath.Ext(u.Path))
	if ext == "" || len(ext) > 6 {
		ext = ".mp4"
	}
	tmp, err := os.CreateTemp("", "dntproxy-video-*"+ext)
	if err != nil {
		cancel()
		return "", nil, err
	}

	cleanup := func() {
		cancel()
		tmp.Close()
		os.Remove(tmp.Name())
	}

	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, MaxVideoBytes+1)); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("write video: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cancel()
		return "", nil, err
	}
	if info, err := os.Stat(tmp.Name()); err != nil || info.Size() == 0 {
		cleanup()
		return "", nil, fmt.Errorf("downloaded video is empty")
	}
	// Size check after download; over-limit downloads are rejected outright.
	if info, _ := os.Stat(tmp.Name()); info.Size() > MaxVideoBytes {
		cleanup()
		return "", nil, fmt.Errorf("video exceeds %d MB limit", MaxVideoBytes/(1<<20))
	}
	return tmp.Name(), cleanup, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/mcp/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/media.go internal/adapter/mcp/media_test.go
git commit -m "feat: mcp media downloader with size/scheme limits"
```

---

### Task 6: Analyzer executor — media → chat → text

**Files:**
- Create: `internal/adapter/mcp/executor.go`
- Create: `internal/adapter/mcp/executor_test.go`
- Modify: `go.mod` via `go get github.com/modelcontextprotocol/go-sdk@latest` (SDK types used in Task 7, but the uuid dep already exists)

**Interfaces:**
- Consumes: `port.ChatService.HandleChat(body []byte, modelStr, requestID string, policy *port.APIKeyPolicy, meta ...port.RequestMetadata) *port.ChatResult`; `shared.AggregateChatCompletion`; `store port.CredentialStore` with `GetSettings() (*domain.Settings, error)` — verify the exact method name on `port.CredentialStore` (`grep -n "GetSettings" internal/port/credential-store.go`) and use it verbatim; `FetchImageAsDataURL`, `DownloadVideo`, `MaxImageBytes` (Task 5); `Locate`, `ExtractFrames` (Tasks 3–4).
- Produces (package `mcp`):
  - `type Identity struct { Policy *port.APIKeyPolicy; TenantID, APIKeyID string }`
  - `func WithIdentity(ctx context.Context, id *Identity) context.Context`; `func IdentityFrom(ctx context.Context) *Identity`
  - `type Analyzer struct { ... }` with `func NewAnalyzer(chat port.ChatService, store port.CredentialStore) *Analyzer` and `func (a *Analyzer) Analyze(ctx context.Context, kind, mediaURL, prompt, model string, maxFrames int) (string, error)` (kind: `"image"` or `"video"`; maxFrames ≤ 0 → settings default)
  - Test seam: `a.ExtractVideo func(ctx context.Context, videoPath string, maxFrames int) (frames []string, cleanup func(), err error)` — swap in tests.
  - Test helper shared with Task 7: `type fakeChat struct` + `func (f *fakeChat) HandleChat(...)`, defined in `executor_test.go` (same package).
  Task 7 consumes Analyzer/Identity; Task 8 consumes WithIdentity.

- [ ] **Step 1: Add the SDK dependency**

```bash
go get github.com/modelcontextprotocol/go-sdk@latest
```

- [ ] **Step 2: Write the failing test**

Create `internal/adapter/mcp/executor_test.go`:

```go
package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// fakeChat records the last HandleChat call and returns a canned result.
type fakeChat struct {
	body      []byte
	model     string
	requestID string
	policy    *port.APIKeyPolicy
	resp      *port.ChatResult
}

func (f *fakeChat) HandleChat(body []byte, model, requestID string, policy *port.APIKeyPolicy, meta ...port.RequestMetadata) *port.ChatResult {
	f.body = body
	f.model = model
	f.requestID = requestID
	f.policy = policy
	return f.resp
}

func sseResp(content string) *port.ChatResult {
	payload := map[string]any{
		"id":     "chatcmpl-x",
		"object": "chat.completion.chunk",
		"choices": []any{
			map[string]any{"index": 0, "delta": map[string]any{"content": content}, "finish_reason": nil},
		},
	}
	b, _ := json.Marshal(payload)
	stream := "data: " + string(b) + "\n\ndata: [DONE]\n\n"
	return &port.ChatResult{Stream: io.NopCloser(strings.NewReader(stream)), StatusCode: 200}
}

func testStore(t *testing.T, s domain.Settings) *storage.JsonDB {
	t.Helper()
	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(cfg *domain.AppConfig) { cfg.Settings = s }); err != nil {
		t.Fatal(err)
	}
	return db
}

var dataPNG = "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

func TestAnalyzeImageResolvesDefaultModel(t *testing.T) {
	chat := &fakeChat{resp: sseResp("a cat sits")}
	a := NewAnalyzer(chat, testStore(t, domain.Settings{
		MCP: domain.MCPSettings{DefaultModel: "glm/glm-4.5v"},
	}))

	text, err := a.Analyze(context.Background(), "image", dataPNG, "describe", "", 0)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if text != "a cat sits" {
		t.Fatalf("text = %q", text)
	}
	if chat.model != "glm/glm-4.5v" {
		t.Fatalf("model = %q, want settings default", chat.model)
	}
	if !strings.HasPrefix(chat.requestID, "mcp-") {
		t.Fatalf("requestID = %q, want mcp- prefix", chat.requestID)
	}
}

func TestAnalyzeParamOverridesDefaultModel(t *testing.T) {
	chat := &fakeChat{resp: sseResp("ok")}
	a := NewAnalyzer(chat, testStore(t, domain.Settings{
		MCP: domain.MCPSettings{DefaultModel: "glm/glm-4.5v"},
	}))

	if _, err := a.Analyze(context.Background(), "image", dataPNG, "d", "kr/claude-sonnet-4.5", 0); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if chat.model != "kr/claude-sonnet-4.5" {
		t.Fatalf("model = %q, want param override", chat.model)
	}
}

func TestAnalyzeWithoutModelErrors(t *testing.T) {
	a := NewAnalyzer(&fakeChat{resp: sseResp("x")}, testStore(t, domain.Settings{}))

	_, err := a.Analyze(context.Background(), "image", dataPNG, "d", "", 0)
	if err == nil || !strings.Contains(err.Error(), "No default vision model") {
		t.Fatalf("err = %v, want No default vision model", err)
	}
}

func TestAnalyzeBuildsImageParts(t *testing.T) {
	chat := &fakeChat{resp: sseResp("ok")}
	a := NewAnalyzer(chat, testStore(t, domain.Settings{MCP: domain.MCPSettings{DefaultModel: "m1"}}))

	if _, err := a.Analyze(context.Background(), "image", dataPNG, "describe this", "", 0); err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	var req struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text,omitempty"`
				ImageURL *struct {
					URL string `json:"url"`
				} `json:"image_url,omitempty"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(chat.body, &req); err != nil {
		t.Fatalf("unmarshal body: %v\n%s", err, chat.body)
	}
	if !req.Stream {
		t.Fatal("stream must be true")
	}
	if req.Model != "m1" {
		t.Fatalf("body model = %q", req.Model)
	}
	parts := req.Messages[0].Content
	if len(parts) != 2 || parts[0].Type != "text" || parts[0].Text != "describe this" {
		t.Fatalf("content parts = %+v", parts)
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != dataPNG {
		t.Fatalf("image part = %+v", parts[1])
	}
}

func TestAnalyzePropagatesChatError(t *testing.T) {
	chat := &fakeChat{resp: &port.ChatResult{StatusCode: 503, Error: "upstream down"}}
	a := NewAnalyzer(chat, testStore(t, domain.Settings{MCP: domain.MCPSettings{DefaultModel: "m1"}}))

	if _, err := a.Analyze(context.Background(), "image", dataPNG, "d", "", 0); err == nil || !strings.Contains(err.Error(), "upstream down") {
		t.Fatalf("err = %v, want upstream down", err)
	}
}

func TestAnalyzeIdentityPassesPolicy(t *testing.T) {
	chat := &fakeChat{resp: sseResp("ok")}
	a := NewAnalyzer(chat, testStore(t, domain.Settings{MCP: domain.MCPSettings{DefaultModel: "m1"}}))
	ctx := WithIdentity(context.Background(), &Identity{Policy: &port.APIKeyPolicy{AllowedModels: []string{"m1"}}, TenantID: "t1", APIKeyID: "k1"})

	if _, err := a.Analyze(ctx, "image", dataPNG, "d", "", 0); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if chat.policy == nil || len(chat.policy.AllowedModels) != 1 {
		t.Fatalf("policy not passed through: %+v", chat.policy)
	}
}

func TestAnalyzeVideoUsesExtractor(t *testing.T) {
	chat := &fakeChat{resp: sseResp("video ok")}
	a := NewAnalyzer(chat, testStore(t, domain.Settings{
		MCP: domain.MCPSettings{DefaultModel: "m1", VideoMaxFrames: 8},
	}))

	var capturedMax int
	a.ExtractVideo = func(ctx context.Context, videoPath string, maxFrames int) ([]string, func(), error) {
		capturedMax = maxFrames
		if filepath.Ext(videoPath) != ".mp4" {
			return nil, nil, errors.New("bad video path: " + videoPath)
		}
		frames := []string{dataPNG, dataPNG, dataPNG}
		return frames, func() {}, nil
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fakevid"))
	}))
	defer srv.Close()

	text, err := a.Analyze(context.Background(), "video", srv.URL+"/v.mp4", "what happens", "", 3)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if text != "video ok" {
		t.Fatalf("text = %q", text)
	}
	if capturedMax != 3 {
		t.Fatalf("maxFrames = %d, want param 3", capturedMax)
	}

	// maxFrames <= 0 must fall back to settings default
	_ = capturedMax
	a.ExtractVideo = func(ctx context.Context, videoPath string, maxFrames int) ([]string, func(), error) {
		if maxFrames != 8 {
			return nil, nil, errors.New("want settings default 8")
		}
		return []string{dataPNG}, func() {}, nil
	}
	if _, err := a.Analyze(context.Background(), "video", srv.URL+"/v.mp4", "what", "", 0); err != nil {
		t.Fatalf("Analyze default frames: %v", err)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/adapter/mcp/ -run TestAnalyze -v`
Expected: FAIL — `undefined: NewAnalyzer` etc.

- [ ] **Step 4: Implement `executor.go`**

Create `internal/adapter/mcp/executor.go`:

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/google/uuid"

	"github.com/dungnt/dntproxy/internal/adapter/ffmpeg"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/port"
)

// Identity carries the authenticated MCP caller into the chat pipeline.
type Identity struct {
	Policy   *port.APIKeyPolicy
	TenantID string
	APIKeyID string
}

type identityKey struct{}

// WithIdentity returns a context carrying the caller identity.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// IdentityFrom returns the identity in ctx, or nil for unauthenticated use.
func IdentityFrom(ctx context.Context) *Identity {
	id, _ := ctx.Value(identityKey{}).(*Identity)
	return id
}

// Analyzer runs one vision analysis: load media → build an OpenAI chat body →
// ChatService → aggregate SSE → text.
type Analyzer struct {
	chat  port.ChatService
	store port.CredentialStore
	// ExtractVideo extracts frames from a downloaded video file. Swap in
	// tests; production default uses ffmpeg (defaultExtractVideo).
	ExtractVideo func(ctx context.Context, videoPath string, maxFrames int) (frames []string, cleanup func(), err error)
}

// NewAnalyzer creates an Analyzer with the production ffmpeg extractor.
func NewAnalyzer(chat port.ChatService, store port.CredentialStore) *Analyzer {
	return &Analyzer{
		chat:         chat,
		store:        store,
		ExtractVideo: defaultExtractVideo,
	}
}

// noModelConfigured is the actionable error when neither the tool call nor
// settings name a vision model.
var errNoModelConfigured = fmt.Errorf("No default vision model configured — set it in Settings → MCP Analytics, or pass the model parameter")

// Analyze runs vision analysis over one media item.
// kind is "image" or "video"; maxFrames <= 0 uses the settings default.
func (a *Analyzer) Analyze(ctx context.Context, kind, mediaURL, prompt, model string, maxFrames int) (string, error) {
	settings, err := a.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("load settings: %w", err)
	}
	settings.MCP.Normalize()
	if model == "" {
		model = settings.MCP.DefaultModel
	}
	if model == "" {
		return "", errNoModelConfigured
	}
	if maxFrames <= 0 {
		maxFrames = settings.MCP.VideoMaxFrames
	}
	if prompt == "" {
		prompt = defaultPrompt(kind)
	}

	var imageParts []string
	switch kind {
	case "image":
		dataURL, err := FetchImageAsDataURL(ctx, http.DefaultClient, mediaURL)
		if err != nil {
			return "", err
		}
		imageParts = []string{dataURL}
	case "video":
		videoPath, cleanup, err := DownloadVideo(ctx, http.DefaultClient, mediaURL)
		if err != nil {
			return "", err
		}
		defer cleanup()

		frames, framesCleanup, err := a.ExtractVideo(ctx, videoPath, maxFrames)
		if framesCleanup != nil {
			defer framesCleanup()
		}
		if err != nil {
			return "", err
		}
		if len(frames) > maxFrames {
			frames = frames[:maxFrames]
		}
		imageParts = frames
	default:
		return "", fmt.Errorf("unknown media kind %q", kind)
	}

	return a.complete(ctx, model, prompt, imageParts)
}

func (a *Analyzer) complete(ctx context.Context, model, prompt string, imageParts []string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:    model,
		Stream:   true,
		Messages: []chatMessage{userMessage(prompt, imageParts)},
	})
	if err != nil {
		return "", err
	}

	requestID := "mcp-" + uuid.NewString()
	var policy *port.APIKeyPolicy
	meta := port.RequestMetadata{Context: ctx}
	if id := IdentityFrom(ctx); id != nil {
		policy = id.Policy
		meta.TenantID = id.TenantID
		meta.APIKeyID = id.APIKeyID
	}

	result := a.chat.HandleChat(body, model, requestID, policy, meta)
	if result.Error != "" {
		return "", fmt.Errorf("model %s: %s", model, result.Error)
	}
	if result.Stream == nil {
		return "", fmt.Errorf("model %s: empty response (HTTP %d)", model, result.StatusCode)
	}
	defer result.Stream.Close()

	completion, err := shared.AggregateChatCompletion(result.Stream, model, requestID)
	if err != nil {
		return "", fmt.Errorf("aggregate response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("model %s returned no choices", model)
	}
	return completion.Choices[0].Message.Content, nil
}

// defaultExtractVideo locates ffmpeg and extracts frames into a temp dir.
func defaultExtractVideo(ctx context.Context, videoPath string, maxFrames int) ([]string, func(), error) {
	ffmpegPath, ffprobePath, err := ffmpeg.Locate(ctx)
	if err != nil {
		return nil, nil, err
	}
	dir, err := os.MkdirTemp("", "dntproxy-frames-*")
	if err != nil {
		return nil, nil, err
	}
	frames, err := ffmpeg.ExtractFrames(ctx, ffmpegPath, ffprobePath, videoPath, dir, maxFrames)
	if err != nil {
		os.RemoveAll(dir)
		return nil, nil, err
	}
	return frames, func() { os.RemoveAll(dir) }, nil
}
```

Create `internal/adapter/mcp/request.go` (chat body types + helpers):

```go
package mcp

type chatRequest struct {
	Model    string        `json:"model"`
	Stream   bool          `json:"stream"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

func userMessage(prompt string, imageDataURLs []string) chatMessage {
	parts := make([]contentPart, 0, len(imageDataURLs)+1)
	parts = append(parts, contentPart{Type: "text", Text: prompt})
	for _, u := range imageDataURLs {
		parts = append(parts, contentPart{Type: "image_url", ImageURL: &imageURL{URL: u}})
	}
	return chatMessage{Role: "user", Content: parts}
}

func defaultPrompt(kind string) string {
	if kind == "video" {
		return "Describe this video in detail."
	}
	return "Describe this image in detail."
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/adapter/mcp/ -v`
Expected: PASS (media tests from Task 5 still green)

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/mcp/executor.go internal/adapter/mcp/request.go internal/adapter/mcp/executor_test.go go.mod go.sum
git commit -m "feat: mcp analyzer executor routing media analysis through ChatService"
```

---

### Task 7: MCP server — tools + streamable handler

**Files:**
- Create: `internal/adapter/mcp/server.go`
- Create: `internal/adapter/mcp/server_test.go`

**Interfaces:**
- Consumes: `Analyzer` (Task 6), `Identity` (Task 6), `domain.MCPSettings` (Task 2), SDK generic `mcp.AddTool[In,Out]`, `mcp.Server`, `mcp.CallToolResult`, `mcp.TextContent`, `mcp.NewStreamableHTTPHandler`. (Verified against the go-sdk source: this SDK has NO `NewTool`/`WithString`/`WithNumber` builders — tool schemas are inferred from typed `In` structs via `jsonschema` struct tags, and handler errors are auto-wrapped into `IsError` tool results. Required fields are non-pointer / no `omitempty`.)
- Produces (package `mcp`):
  - `func NewServer(store port.CredentialStore, chat port.ChatService) *mcp.Server`
  - `func NewStreamableHandler(store port.CredentialStore, chat port.ChatService) http.Handler`
  Task 8 consumes `NewStreamableHandler`.

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/mcp/server_test.go` (uses `mcp.NewInMemoryTransports` + `srv.Connect` — see the SDK example; `Session.CallTool` returns `(*CallToolResult, error)`):

```go
package mcp

import (
	"context"
	"strings"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestMCPServerToolsRoundTrip(t *testing.T) {
	chat := &fakeChat{resp: sseResp("analysis result")}
	store := testStore(t, domain.Settings{MCP: domain.MCPSettings{DefaultModel: "glm/glm-4.5v"}})
	srv := NewServer(store, chat)

	ctx := context.Background()
	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := gomcp.NewClient(&gomcp.Implementation{Name: "it", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	if !names["analyze_image"] || !names["analyze_video"] {
		t.Fatalf("tools = %v, want analyze_image + analyze_video", names)
	}

	res, err := session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      "analyze_image",
		Arguments: map[string]any{"image_url": dataPNG, "prompt": "what is this"},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	text := resultText(t, res)
	if !strings.Contains(text, "analysis result") {
		t.Fatalf("text = %q", text)
	}
	if chat.model != "glm/glm-4.5v" {
		t.Fatalf("model = %q, want settings default", chat.model)
	}
}

func TestMCPServerMissingModelIsToolError(t *testing.T) {
	srv := NewServer(testStore(t, domain.Settings{}), &fakeChat{resp: sseResp("x")})

	ctx := context.Background()
	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := gomcp.NewClient(&gomcp.Implementation{Name: "it", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      "analyze_image",
		Arguments: map[string]any{"image_url": dataPNG},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !res.IsError {
		t.Fatal("missing model must be a tool error result")
	}
	if !strings.Contains(resultText(t, res), "No default vision model") {
		t.Fatalf("text = %q", resultText(t, res))
	}
}

func TestMCPServerMissingImageURLIsToolError(t *testing.T) {
	srv := NewServer(testStore(t, domain.Settings{MCP: domain.MCPSettings{DefaultModel: "m"}}), &fakeChat{resp: sseResp("x")})

	ctx := context.Background()
	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := gomcp.NewClient(&gomcp.Implementation{Name: "it", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &gomcp.CallToolParams{Name: "analyze_image", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	// Missing required image_url is rejected by the SDK's schema validation and
	// returned as an IsError tool result.
	if !res.IsError {
		t.Fatalf("res = %+v, want missing-argument tool error", res)
	}
}

func resultText(t *testing.T, res *gomcp.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*gomcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/mcp/ -run TestMCPServer -v`
Expected: FAIL — `undefined: NewServer`

- [ ] **Step 3: Implement `server.go`**

Create `internal/adapter/mcp/server.go` — the generic `mcp.AddTool[In, Out]` infers each tool's input JSON schema from the `In` struct; `jsonschema` struct tags become property descriptions, and required-ness follows Go types (non-pointer = required). Returned errors are automatically converted to `CallToolResult{IsError: true}` by the SDK, so handlers just return `err`:

```go
package mcp

import (
	"context"
	"net/http"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/version"
)

const serverName = "dntproxy"

type analyzeImageArgs struct {
	// ImageURL is an https URL or a data:image/...;base64,... URI.
	ImageURL string `json:"image_url" jsonschema:"Image URL (https) or data:image/...;base64,... URI"`
	// Prompt is what to analyze; defaults to describing the image in detail.
	Prompt string `json:"prompt,omitempty" jsonschema:"What to analyze. Default: describe the image in detail."`
	// Model is the vision model, alias, or combo to use; defaults to the
	// server setting mcp.defaultModel.
	Model string `json:"model,omitempty" jsonschema:"Vision model to use (model id, alias or combo). Default: server setting mcp.defaultModel."`
}

type analyzeVideoArgs struct {
	// VideoURL is an https URL to a video file, max 150 MB.
	VideoURL string `json:"video_url" jsonschema:"Video URL (https), max 150 MB"`
	// Prompt is what to analyze; defaults to describing the video in detail.
	Prompt string `json:"prompt,omitempty" jsonschema:"What to analyze. Default: describe the video in detail."`
	// Model is the vision model, alias, or combo to use.
	Model string `json:"model,omitempty" jsonschema:"Vision model to use. Default: server setting mcp.defaultModel."`
	// MaxFrames is how many evenly spaced frames to extract (1-16).
	MaxFrames int `json:"max_frames,omitempty" jsonschema:"Frames to extract (1-16). Default: server setting, 8."`
}

// NewServer builds the MCP server with the analytics tools registered.
func NewServer(store port.CredentialStore, chat port.ChatService) *gomcp.Server {
	analyzer := NewAnalyzer(chat, store)
	srv := gomcp.NewServer(&gomcp.Implementation{Name: serverName, Version: version.Version}, nil)

	gomcp.AddTool[analyzeImageArgs, any](srv, &gomcp.Tool{
		Name:        "analyze_image",
		Description: "Analyze an image with a vision model and return a text description. Accepts an https URL or a data: URI.",
	}, func(ctx context.Context, _ *gomcp.CallToolRequest, args analyzeImageArgs) (*gomcp.CallToolResult, any, error) {
		if args.ImageURL == "" {
			return nil, nil, errImageURLRequired
		}
		text, err := analyzer.Analyze(ctx, "image", args.ImageURL, args.Prompt, args.Model, 0)
		if err != nil {
			return nil, nil, err
		}
		return toolText(text), nil, nil
	})

	gomcp.AddTool[analyzeVideoArgs, any](srv, &gomcp.Tool{
		Name:        "analyze_video",
		Description: "Analyze a video by extracting evenly spaced frames and sending them to a vision model. Returns a text description.",
	}, func(ctx context.Context, _ *gomcp.CallToolRequest, args analyzeVideoArgs) (*gomcp.CallToolResult, any, error) {
		if args.VideoURL == "" {
			return nil, nil, errVideoURLRequired
		}
		text, err := analyzer.Analyze(ctx, "video", args.VideoURL, args.Prompt, args.Model, args.MaxFrames)
		if err != nil {
			return nil, nil, err
		}
		return toolText(text), nil, nil
	})

	return srv
}

// NewStreamableHandler returns the HTTP handler mounted at /mcp. One server
// instance serves all requests; per-request auth identity arrives via context.
func NewStreamableHandler(store port.CredentialStore, chat port.ChatService) http.Handler {
	srv := NewServer(store, chat)
	return gomcp.NewStreamableHTTPHandler(func(*http.Request) *gomcp.Server { return srv }, nil)
}

// toolText wraps analysis text as MCP text content.
func toolText(text string) *gomcp.CallToolResult {
	return &gomcp.CallToolResult{Content: []gomcp.Content{&gomcp.TextContent{Text: text}}}
}
```

Create `internal/adapter/mcp/errors.go`:

```go
package mcp

import "errors"

var (
	errImageURLRequired = errors.New("image_url is required (https URL or data: URI)")
	errVideoURLRequired = errors.New("video_url is required (https URL)")
)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/mcp/ -v && go build ./...`
Expected: PASS; whole package builds

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/server.go internal/adapter/mcp/server_test.go
git commit -m "feat: MCP server with analyze_image and analyze_video tools"
```

---

### Task 8: HTTP wiring — `/mcp` route with API key auth

**Files:**
- Create: `internal/adapter/http/mcp-handler.go`
- Create: `internal/adapter/http/mcp-handler_test.go`
- Modify: `internal/adapter/http/router.go` (call `mountMCP` from `NewRouter`)
- Modify: `cmd/dntproxy/main.go` (startup log line)

**Interfaces:**
- Consumes: `mcp.NewStreamableHandler(store, chat)` (Task 7), `mcp.Identity`/`mcp.WithIdentity` (Task 6), `apiKeyMiddleware(store)`, `extractAPIKeyPolicy(c)`, `GetTenantID(c)`, `GetAPIKey(c)` (router.go).
- Produces: `POST|GET|DELETE /mcp` endpoint, API-key protected. No new exported symbols.

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/http/mcp-handler_test.go`:

```go
package http

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/provider"
	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gin-gonic/gin"
)

func newMCPTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	cfg := domain.DefaultConfig()
	cfg.Settings.AdminKeyBootstrapped = true
	cfg.Settings.DashboardAccessMigrated = true
	cfg.APIKeys = []domain.APIKey{{
		ID: "k1", Name: "proxy", Key: "sk-dnt-mcp-test", IsActive: true,
	}}
	if err := db.Update(func(c *domain.AppConfig) {
		c.APIKeys = cfg.APIKeys
		c.Settings = cfg.Settings
	}); err != nil {
		t.Fatal(err)
	}

	return NewRouter(db, provider.NewRegistry(), provider.NewImageRegistry(), nil)
}

func TestMCPRequiresAPIKey(t *testing.T) {
	router := newMCPTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 without key", w.Code)
	}
}

func TestMCPRejectsInvalidKey(t *testing.T) {
	router := newMCPTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Authorization", "Bearer sk-wrong")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 with invalid key", w.Code)
	}
}

func TestMCPValidKeyPassesAuth(t *testing.T) {
	router := newMCPTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Authorization", "Bearer sk-dnt-mcp-test")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Fatalf("status = %d, valid key must pass auth (body: %s)", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/http/ -run TestMCP -v`
Expected: FAIL — `undefined: mountMCP` won't appear yet; instead 404/401 mismatch: without wiring, the first test may pass (NoRoute returns 404... actually NoRoute returns JSON 404) — `status = 404, want 401`. Tests fail because `/mcp` isn't registered.

- [ ] **Step 3: Implement `mcp-handler.go` and wire the router**

Create `internal/adapter/http/mcp-handler.go`:

```go
package http

import (
	"github.com/dungnt/dntproxy/internal/adapter/mcp"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// mountMCP registers the MCP analytics endpoint at /mcp behind the API key
// middleware, injecting the authenticated caller's identity so account
// selection and key policies apply to MCP-triggered analysis.
func mountMCP(r *gin.Engine, store port.CredentialStore, chatService port.ChatService) {
	identity := func(c *gin.Context) {
		id := &mcp.Identity{
			Policy:   extractAPIKeyPolicy(c),
			TenantID: GetTenantID(c),
		}
		if key := GetAPIKey(c); key != nil {
			id.APIKeyID = key.ID
		}
		c.Request = c.Request.WithContext(mcp.WithIdentity(c.Request.Context(), id))
		c.Next()
	}
	r.Any("/mcp", apiKeyMiddleware(store), identity, gin.WrapH(mcp.NewStreamableHandler(store, chatService)))
}
```

In `internal/adapter/http/router.go`, inside `NewRouter` after `RegisterAPIRoutes(...)` (line ~80):

```go
	// MCP analytics server (streamable HTTP, API key required)
	mountMCP(r, store, chatService)
```

In `cmd/dntproxy/main.go`, after the Anthropic Messages API log line (~line 158):

```go
	log.Printf("[dntproxy] MCP analytics endpoint: http://localhost%s/mcp", addr)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/http/ -run TestMCP -v && go build ./...`
Expected: PASS; binary builds

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/http/mcp-handler.go internal/adapter/http/mcp-handler_test.go internal/adapter/http/router.go cmd/dntproxy/main.go
git commit -m "feat: mount MCP analytics endpoint at /mcp behind API key auth"
```

---

### Task 9: Settings UI — MCP Analytics card

**Files:**
- Modify: `ui/src/lib/go-api.ts` (mapSettings + updateSettings payload)
- Modify: `ui/src/components/screens/settings-screen.tsx` (SettingsData, DEFAULT_SETTINGS, handleSave, new card component)

**Interfaces:**
- Consumes: settings API fields `mcp.defaultModel`, `mcp.videoMaxFrames` (Task 2); `goApi.getModels()` (existing; returns models with `capabilities` array); `goApi.getSettings()`/`updateSettings`.
- Produces: UI-only. No consumers.

- [ ] **Step 1: Update `go-api.ts` mapping**

In `mapSettings` (line ~275), add to the returned object:

```ts
    mcpDefaultModel: String(go?.mcp?.defaultModel || ""),
    mcpVideoMaxFrames: Number(go?.mcp?.videoMaxFrames ?? 8),
```

In `updateSettings` payload object (line ~467), add:

```ts
      mcp: {
        defaultModel: data.mcpDefaultModel || "",
        videoMaxFrames: Number(data.mcpVideoMaxFrames ?? 8),
      },
```

- [ ] **Step 2: Update `settings-screen.tsx` state and save**

Add to `SettingsData` interface (after `disableImageGeneration: boolean;`):

```ts
  mcpDefaultModel: string;
  mcpVideoMaxFrames: number;
```

Add to `DEFAULT_SETTINGS`:

```ts
  mcpDefaultModel: "",
  mcpVideoMaxFrames: 8,
```

In `handleSave`, add to the `updateSettings({...})` payload:

```ts
        mcpDefaultModel: settings.mcpDefaultModel,
        mcpVideoMaxFrames: settings.mcpVideoMaxFrames,
```

Add `Puzzle` to the lucide-react import list.

- [ ] **Step 3: Add the MCP Analytics card**

In `settings-screen.tsx`, change the render of the last line of the main return from `<DefaultModelsCard settings={settings} updateField={updateField} />` to:

```tsx
      <DefaultModelsCard settings={settings} updateField={updateField} />
      <MCPCard settings={settings} updateField={updateField} />
```

Add the component at the bottom of the file:

```tsx
function MCPCard({
  settings,
  updateField,
}: {
  settings: SettingsData;
  updateField: <K extends keyof SettingsData>(
    key: K,
    value: SettingsData[K],
  ) => void;
}) {
  const [visionModels, setVisionModels] = useState<string[]>([]);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    goApi
      .getModels()
      .then((models: Array<{ id: string; capabilities?: string[] }>) => {
        setVisionModels(
          (models || [])
            .filter((m) => Array.isArray(m.capabilities) && m.capabilities.includes("vision"))
            .map((m) => m.id),
        );
      })
      .catch(() => setVisionModels([]));
  }, []);

  const endpoint = `${window.location.origin}/mcp`;
  const snippet = JSON.stringify(
    {
      mcpServers: {
        dntproxy: {
          url: endpoint,
          headers: { Authorization: "Bearer <your-api-key>" },
        },
      },
    },
    null,
    2,
  );

  const copySnippet = async () => {
    try {
      await navigator.clipboard.writeText(snippet);
      setCopied(true);
      toast.success("Client config copied");
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Copy failed");
    }
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Puzzle className="size-5 text-emerald-600" />
          <CardTitle className="text-base">MCP Analytics</CardTitle>
        </div>
        <CardDescription>
          MCP server endpoint with analyze_image / analyze_video tools. Any API
          key can call it with <code>Authorization: Bearer</code>.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-5 p-6 pt-0">
        <div className="space-y-2">
          <Label htmlFor="mcpModel">Default vision model</Label>
          <Input
            id="mcpModel"
            list="mcp-vision-models"
            placeholder="e.g. glm/glm-4.5v"
            value={settings.mcpDefaultModel}
            onChange={(e) => updateField("mcpDefaultModel", e.target.value)}
            className="max-w-xs"
          />
          <datalist id="mcp-vision-models">
            {visionModels.map((id) => (
              <option key={id} value={id} />
            ))}
          </datalist>
          <p className="text-xs text-muted-foreground">
            Used when a tool call does not pass its own model. Tools also
            accept aliases and combos.
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="mcpFrames">Video frames per analysis</Label>
          <Input
            id="mcpFrames"
            type="number"
            min={1}
            max={16}
            value={settings.mcpVideoMaxFrames}
            onChange={(e) =>
              updateField(
                "mcpVideoMaxFrames",
                Math.min(16, Math.max(1, parseInt(e.target.value, 10) || 8)),
              )
            }
            className="max-w-xs"
          />
          <p className="text-xs text-muted-foreground">
            Videos are sampled into evenly spaced frames (max 16) and sent as
            images to the vision model.
          </p>
        </div>

        <div className="space-y-2 border-t pt-4">
          <Label>MCP endpoint</Label>
          <code className="block rounded-md bg-muted px-3 py-2 text-xs">
            {endpoint}
          </code>
          <div className="flex items-center gap-2">
            <Button type="button" variant="outline" size="sm" onClick={copySnippet}>
              {copied ? "Copied" : "Copy client config"}
            </Button>
            <span className="text-xs text-muted-foreground">
              Paste into your MCP client config, then replace the placeholder key.
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 4: Build the UI to verify**

Run: `cd ui && npm run build`
Expected: tsc + vite build succeed with no type errors

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/go-api.ts ui/src/components/screens/settings-screen.tsx
git commit -m "feat(ui): MCP Analytics settings card with client config snippet"
```

---

### Task 10: Docs + full verification

**Files:**
- Modify: `AGENTS.md` (API endpoints list + new Key Behaviors subsection + implementation status)
- Modify: `docs/superpowers/specs/2026-09-03-mcp-image-video-analytics-design.md` — no changes; verify only.

**Interfaces:** none — documentation and final gates.

- [ ] **Step 1: Update AGENTS.md**

In the Management API endpoints section (after the auto-login lines), add:

```markdown
- `POST|GET|DELETE /mcp` — MCP analytics server (streamable HTTP, `analyze_image` / `analyze_video`)
```

Add a new subsection under Key Behaviors (after the Cloudflare Tunnel section):

```markdown
### MCP Analytics Server (`internal/adapter/mcp`)
- Streamable HTTP MCP endpoint at `/mcp`, API key required (`Authorization: Bearer`), any valid key works
- Tools: `analyze_image(image_url, prompt?, model?)` and `analyze_video(video_url, prompt?, model?, max_frames?)`,
  both return `{ content: [{ type: "text", text }] }`; failures come back as `isError: true` tool results
- Media is always downloaded and inlined as base64 data URLs (image ≤ 20 MB; video ≤ 150 MB / 2 min timeout)
- Video: ffprobe duration → evenly spaced JPEG frames (1–16, settings default 8, scaled ≤ 1280 px) → sent as
  multiple `image_url` parts, so every vision model can analyze video
- Model resolution: tool `model` param → settings `mcp.defaultModel` → tool error; ffmpeg/ffprobe auto-download
  to `~/.dntproxy/bin/` on first video call (env overrides `DNTPROXY_FFMPEG_PATH` / `DNTPROXY_FFPROBE_PATH`)
- Requests run through `ChatService` (requestID `mcp-<uuid>`), so account fallback, cooldown and logging apply
```

In Implementation Status, add:

```markdown
### Phase 9: MCP Analytics Server — DONE
- `/mcp` streamable HTTP endpoint (official go-sdk), analyze_image/analyze_video tools
- ffmpeg frame-extraction pipeline (auto-download like cloudflared)
- Settings `mcp` block + dashboard "MCP Analytics" card with client config snippet
```

- [ ] **Step 2: Run the full verification suite**

```bash
go vet ./...
go test ./...
go build -o dntproxy.exe ./cmd/dntproxy/
cd ui && npm run build
```

Expected: vet clean, all tests PASS, both builds succeed.

- [ ] **Step 3: Manual smoke test (server running)**

```bash
./dntproxy.exe &
# initialize handshake with a real key from ~/.dntproxy/db.json
curl -s -X POST http://localhost:20199/mcp \
  -H "Authorization: Bearer <key>" -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'
```

Expected: JSON/SSE response with `serverInfo.name == "dntproxy"`. Without the key: 401.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md
git commit -m "docs: MCP analytics server in AGENTS.md"
```

---

## Task Dependency Order

Tasks 1 → 2 → (3 → 4) → 5 → 6 → 7 → 8 → 9 → 10. Tasks 3–5 are independent of each other; 6 needs 1+2+4+5; 7 needs 6; 8 needs 7; 9 needs 2 (API fields); 10 last.
