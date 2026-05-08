package filters

import (
	"fmt"
	"strings"
	"testing"
)

func TestLogFilter_DedupAdjacentLines(t *testing.T) {
	var lines []string
	for i := 0; i < 15; i++ {
		lines = append(lines, "2026-05-08T10:00:01Z INFO repeated message")
	}
	in := strings.Join(lines, "\n")
	out, ok := LogFilter(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// Should collapse repeated lines into "msg ×15"
	if strings.Count(out, "repeated message") > 1 {
		t.Error("adjacent identical lines should be deduped")
	}
	if !strings.Contains(out, "×15") {
		t.Errorf("dedup count marker missing, got: %s", out)
	}
}

func TestLogFilter_StripTimestamp(t *testing.T) {
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = "2026-05-08T10:00:01Z INFO message body"
	}
	in := strings.Join(lines, "\n")
	out, _ := LogFilter(in)
	// Timestamps should be stripped
	if strings.Contains(out, "2026-05-08T10:00:01Z") {
		t.Error("timestamp prefix should be stripped from repeated lines")
	}
}

func TestLogFilter_TruncateLongLog(t *testing.T) {
	var lines []string
	for i := 0; i < 300; i++ {
		lines = append(lines, "2026-05-08T10:00:"+fmt.Sprintf("%02d", i%60)+":00Z INFO unique message number "+fmt.Sprintf("%d", i))
	}
	in := strings.Join(lines, "\n")
	out, ok := LogFilter(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	outLines := strings.Split(out, "\n")
	if len(outLines) >= 300 {
		t.Errorf("expected truncation, got %d lines", len(outLines))
	}
	if !strings.Contains(out, "lines elided") {
		t.Error("truncation marker missing")
	}
}

func TestLogFilter_ShortLog_PassThrough(t *testing.T) {
	in := "line one\nline two\nline three"
	out, _ := LogFilter(in)
	// Short logs should pass through without modification (< 10 unique lines)
	_ = out // just must not panic
}
