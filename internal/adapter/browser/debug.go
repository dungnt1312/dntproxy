package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
)

// DebugScreenshot saves a best-effort PNG of the page state after a failed
// login attempt so stall points (captcha, unknown screens) are diagnosable.
// Returns the file path, or "" when capture failed.
func DebugScreenshot(page *rod.Page, label string) string {
	if page == nil {
		return ""
	}
	dir := filepath.Join(os.TempDir(), "dntproxy-autologin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%d.png", sanitizeLabel(label), time.Now().Unix()))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := page.Context(ctx).Screenshot(false, nil)
	if err != nil {
		return ""
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return ""
	}
	return path
}

// PageURL returns the page's current URL ("" when unavailable).
func PageURL(page *rod.Page) string {
	if page == nil {
		return ""
	}
	if info, err := page.Info(); err == nil && info != nil {
		return info.URL
	}
	return ""
}

func sanitizeLabel(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '@' || r == '.' || r == '-' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return string(out)
}
