package compressor

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCompress_DisabledShortCircuits(t *testing.T) {
	c := New(Options{Enabled: false})
	body := []byte(`{"model":"test","messages":[{"role":"tool","tool_call_id":"x","content":"` +
		buildLongGitDiff() + `"}]}`)
	out, stats := c.Compress(body)
	if !bytes.Equal(body, out) {
		t.Fatal("body mutated when disabled")
	}
	if stats.OriginalBytes != 0 {
		t.Fatal("stats should be zero when disabled")
	}
}

func TestCompress_FailOpenOnInvalidJSON(t *testing.T) {
	c := New(Options{Enabled: true, MinContentLength: 1})
	in := []byte("{not-json")
	out, stats := c.Compress(in)
	if !bytes.Equal(in, out) {
		t.Fatal("body mutated on invalid JSON")
	}
	if stats.OriginalBytes != 0 {
		t.Fatal("stats should be empty on parse failure")
	}
}

func TestCompress_NoMessages(t *testing.T) {
	c := New(Options{Enabled: true, MinContentLength: 1})
	body := []byte(`{"model":"gpt-4","stream":true}`)
	out, _ := c.Compress(body)
	if !bytes.Equal(body, out) {
		t.Fatal("body mutated when no messages key")
	}
}

func TestCompress_ShapeA_ToolRole(t *testing.T) {
	c := New(Options{Enabled: true, MinContentLength: 100})
	diff := buildLongGitDiff()
	payload := map[string]interface{}{
		"model": "test",
		"messages": []interface{}{
			map[string]interface{}{
				"role":         "tool",
				"tool_call_id": "call_1",
				"content":      diff,
			},
		},
	}
	body, _ := json.Marshal(payload)
	out, stats := c.Compress(body)
	if len(out) >= len(body) {
		t.Fatalf("expected compression, got orig=%d out=%d", len(body), len(out))
	}
	if stats.Detections["git-diff"] == 0 {
		t.Fatal("expected git-diff detection")
	}
}

func TestCompress_BelowMinLength_Skipped(t *testing.T) {
	c := New(Options{Enabled: true, MinContentLength: 10000})
	diff := buildLongGitDiff()
	payload := map[string]interface{}{
		"model": "test",
		"messages": []interface{}{
			map[string]interface{}{
				"role":         "tool",
				"tool_call_id": "call_1",
				"content":      diff,
			},
		},
	}
	body, _ := json.Marshal(payload)
	out, stats := c.Compress(body)
	// Body might change slightly due to JSON re-serialization, but content should be uncompressed
	_ = out
	if stats.Detections["git-diff"] > 0 {
		t.Fatal("should not have compressed content below MinContentLength")
	}
	if stats.Skipped == 0 {
		t.Fatal("expected skipped count > 0")
	}
}

func TestCompress_IsError_Skipped(t *testing.T) {
	c := New(Options{Enabled: true, MinContentLength: 1})
	diff := buildLongGitDiff()
	payload := map[string]interface{}{
		"model": "test",
		"messages": []interface{}{
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "call_1",
						"is_error":    true,
						"content":     diff,
					},
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	_, stats := c.Compress(body)
	if stats.Detections["git-diff"] > 0 {
		t.Fatal("should not compress is_error=true blocks")
	}
}

func TestCompress_CodeFile_NotCompressed(t *testing.T) {
	c := New(Options{Enabled: true, MinContentLength: 1})
	src := "package main\nimport \"fmt\"\nfunc main() {\nfmt.Println(\"hello\")\n}\n"
	payload := map[string]interface{}{
		"model": "test",
		"messages": []interface{}{
			map[string]interface{}{
				"role":         "tool",
				"tool_call_id": "call_1",
				"content":      src,
			},
		},
	}
	body, _ := json.Marshal(payload)
	_, stats := c.Compress(body)
	if stats.Detections["code-file"] > 0 {
		t.Fatal("code files should never be compressed")
	}
}

// buildLongGitDiff returns a realistic git diff long enough to trigger compression.
func buildLongGitDiff() string {
	var sb bytes.Buffer
	for i := 0; i < 5; i++ {
		sb.WriteString("diff --git a/file.go b/file.go\n")
		sb.WriteString("index abc1234..def5678 100644\n")
		sb.WriteString("--- a/file.go\n")
		sb.WriteString("+++ b/file.go\n")
		sb.WriteString("@@ -10,7 +10,7 @@ func Foo() {\n")
		sb.WriteString(" context line one\n")
		sb.WriteString(" context line two\n")
		sb.WriteString("-old implementation line here that is quite verbose\n")
		sb.WriteString("+new implementation line here that is improved\n")
		sb.WriteString(" context line three\n")
		sb.WriteString(" context line four\n")
		sb.WriteString(" context line five\n")
	}
	return sb.String()
}
