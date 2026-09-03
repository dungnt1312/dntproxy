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
