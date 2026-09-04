package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/ffmpeg"
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
	meta      []port.RequestMetadata
	resp      *port.ChatResult
}

func (f *fakeChat) HandleChat(body []byte, model, requestID string, policy *port.APIKeyPolicy, meta ...port.RequestMetadata) *port.ChatResult {
	f.body = body
	f.model = model
	f.requestID = requestID
	f.policy = policy
	f.meta = meta
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
	if len(chat.meta) == 0 {
		t.Fatal("no request metadata passed through")
	}
	if chat.meta[0].TenantID != "t1" {
		t.Fatalf("meta TenantID = %q, want t1", chat.meta[0].TenantID)
	}
	if chat.meta[0].APIKeyID != "k1" {
		t.Fatalf("meta APIKeyID = %q, want k1", chat.meta[0].APIKeyID)
	}
}

func TestFramesToTimeline(t *testing.T) {
	dir := t.TempDir()

	// Real JPEG magic plus arbitrary bytes; the helper must not validate
	// content beyond a successful non-empty read.
	write := func(name string, b []byte) ffmpeg.Frame {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return ffmpeg.Frame{Path: p}
	}
	f1 := write("frame-000.jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10})
	f1.Start, f1.End = 0, 2*time.Minute
	f2 := write("frame-001.jpg", []byte{0xFF, 0xD8, 'j', 'p', 'e', 'g'})
	f2.Start, f2.End = 2*time.Minute, 4*time.Minute

	timeline, err := framesToTimeline([]ffmpeg.Frame{f1, f2})
	if err != nil {
		t.Fatalf("framesToTimeline: %v", err)
	}
	if len(timeline) != 2 {
		t.Fatalf("got %d frames, want 2", len(timeline))
	}
	want := func(b []byte) string {
		return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(b)
	}
	if timeline[0].DataURL != want([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}) {
		t.Fatalf("url[0] = %q", timeline[0].DataURL)
	}
	if timeline[0].Start != 0 || timeline[0].End != 2*time.Minute {
		t.Fatalf("frame[0] range = [%v,%v]", timeline[0].Start, timeline[0].End)
	}
	if timeline[1].Start != 2*time.Minute || timeline[1].End != 4*time.Minute {
		t.Fatalf("frame[1] range = [%v,%v]", timeline[1].Start, timeline[1].End)
	}
}

func TestFramesToTimelineErrorsOnMissingFile(t *testing.T) {
	if _, err := framesToTimeline([]ffmpeg.Frame{{Path: filepath.Join(t.TempDir(), "nope.jpg")}}); err == nil {
		t.Fatal("want error for missing frame file")
	}
}

func TestAnalyzeVideoUsesExtractor(t *testing.T) {
	chat := &fakeChat{resp: sseResp("video ok")}
	a := NewAnalyzer(chat, testStore(t, domain.Settings{
		MCP: domain.MCPSettings{DefaultModel: "m1", VideoMaxFrames: 8},
	}))

	var capturedMax int
	a.ExtractVideo = func(ctx context.Context, videoPath string, maxFrames int) ([]TimelineFrame, func(), error) {
		capturedMax = maxFrames
		if filepath.Ext(videoPath) != ".mp4" {
			return nil, nil, errors.New("bad video path: " + videoPath)
		}
		frames := []TimelineFrame{
			{DataURL: dataPNG, Start: 0, End: time.Minute},
			{DataURL: dataPNG, Start: time.Minute, End: 2 * time.Minute},
			{DataURL: dataPNG, Start: 2 * time.Minute, End: 3 * time.Minute},
		}
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

	// The chat body must interleave timeline labels with the frames so the
	// model can attribute content to specific video spans.
	var req struct {
		Messages []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text,omitempty"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(chat.body, &req); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	parts := req.Messages[0].Content
	// [prompt, label, img, label, img, label, img]
	if len(parts) != 7 {
		t.Fatalf("content parts = %d, want 7", len(parts))
	}
	if !strings.Contains(parts[1].Text, "00:00 – 01:00") {
		t.Fatalf("label[0] = %q, want 00:00 – 01:00", parts[1].Text)
	}
	if !strings.Contains(parts[3].Text, "01:00 – 02:00") {
		t.Fatalf("label[1] = %q, want 01:00 – 02:00", parts[3].Text)
	}

	// maxFrames <= 0 must fall back to settings default
	a.ExtractVideo = func(ctx context.Context, videoPath string, maxFrames int) ([]TimelineFrame, func(), error) {
		if maxFrames != 8 {
			return nil, nil, errors.New("want settings default 8")
		}
		return []TimelineFrame{{DataURL: dataPNG}}, func() {}, nil
	}
	if _, err := a.Analyze(context.Background(), "video", srv.URL+"/v.mp4", "what", "", 0); err != nil {
		t.Fatalf("Analyze default frames: %v", err)
	}
}

func TestAnalyzeVideoClampsMaxFrames(t *testing.T) {
	chat := &fakeChat{resp: sseResp("clamped ok")}
	a := NewAnalyzer(chat, testStore(t, domain.Settings{
		MCP: domain.MCPSettings{DefaultModel: "m1"},
	}))

	var capturedMax int
	a.ExtractVideo = func(ctx context.Context, videoPath string, maxFrames int) ([]TimelineFrame, func(), error) {
		capturedMax = maxFrames
		return []TimelineFrame{{DataURL: dataPNG}}, func() {}, nil
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fakevid"))
	}))
	defer srv.Close()

	// Tool-supplied max_frames must be clamped to the hard ceiling of 16
	// (matches MCPSettings.Normalize), never passed to ffmpeg unbounded.
	if _, err := a.Analyze(context.Background(), "video", srv.URL+"/v.mp4", "what", "m1", 1000); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if capturedMax != 16 {
		t.Fatalf("maxFrames = %d, want clamped 16", capturedMax)
	}
}
