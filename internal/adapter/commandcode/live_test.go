package commandcode

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestLiveCommandCodeAuth(t *testing.T) {
	if os.Getenv("DNTPROXY_LIVE_COMMANDCODE") == "" {
		t.Skip("set DNTPROXY_LIVE_COMMANDCODE=1 to run against the real Command Code API")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".commandcode", "auth.json"))
	if err != nil {
		t.Fatalf("read auth.json: %v", err)
	}
	var auth struct {
		APIKey string `json:"apiKey"`
	}
	if err := json.Unmarshal(raw, &auth); err != nil {
		t.Fatalf("parse auth.json: %v", err)
	}
	if auth.APIKey == "" {
		t.Fatal("auth.json has empty apiKey")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	reader, status, err := NewExecutor().Execute(ctx, "deepseek/deepseek-v4-pro", []byte(`{
		"model":"deepseek/deepseek-v4-pro",
		"messages":[{"role":"user","content":"Reply with exactly: pong"}],
		"max_tokens":32,
		"stream":true
	}`), &domain.Credentials{
		Provider: "commandcode",
		APIKey:   auth.APIKey,
	}, &testRequestLogger{})
	if err != nil {
		t.Fatalf("Execute() status=%d err=%v", status, err)
	}
	defer reader.Close()
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "data: ") {
		t.Fatalf("expected OpenAI SSE, got %q", truncateForTest(text, 400))
	}
	if !strings.Contains(strings.ToLower(text), "pong") && !strings.Contains(text, `"content"`) {
		t.Fatalf("expected content chunks, got %q", truncateForTest(text, 400))
	}
}

func truncateForTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
