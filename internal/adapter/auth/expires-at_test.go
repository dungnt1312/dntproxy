package auth

import (
	"testing"
	"time"
)

func TestParseExpiresAtRFC3339(t *testing.T) {
	raw := "2026-07-08T12:00:00Z"
	ts, ok := ParseExpiresAt(raw)
	if !ok {
		t.Fatal("expected ok")
	}
	if ts.UTC().Format(time.RFC3339) != raw {
		t.Fatalf("got %s", ts.Format(time.RFC3339))
	}
}

func TestParseExpiresAtCLIProxyLayout(t *testing.T) {
	_, ok := ParseExpiresAt("2026-07-08 12:00:00")
	if !ok {
		t.Fatal("expected ok for space-separated layout")
	}
}

func TestNormalizeExpiresAtRFC3339(t *testing.T) {
	out, ok := NormalizeExpiresAtRFC3339("2026-07-08 12:00:00")
	if !ok || out == "" {
		t.Fatalf("normalize failed: %q ok=%v", out, ok)
	}
}