// Package browser drives a real Chromium via CDP (go-rod) to automate
// interactive OAuth logins that cannot be done with plain HTTP calls
// (email/password/2FA entry, consent screens, bot checks).
package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Session is one isolated automation browser with a fresh profile — safe to
// run several in parallel without cookie/session mixing.
type Session struct {
	browser *rod.Browser
}

// launchMu serializes browser launches so the first-run Chromium download
// (rod fetches it automatically, like cloudflared) never races between
// parallel workers.
var launchMu sync.Mutex

// NewSession launches a fresh browser. Chromium is downloaded on first use;
// later launches reuse the cached binary or a system Chrome when present.
func NewSession(headless bool) (*Session, error) {
	launchMu.Lock()
	defer launchMu.Unlock()

	// Launching can include a one-time browser download, so allow 10 minutes
	// regardless of the caller's (much shorter) per-account context.
	launchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	l := launcher.New().
		Context(launchCtx).
		Headless(headless).
		// Rod enables these by default; both make bot detection trivial.
		Delete("enable-automation").
		Set("disable-blink-features", "AutomationControlled").
		Set("no-default-browser-check").
		Set("no-first-run").
		// Leakless drops an unsigned helper exe into %TEMP% that EDR products
		// (SentinelOne) block as a security risk. Without it, a hard-killed
		// proxy can orphan browsers — SweepOrphans cleans those up instead.
		Leakless(false)

	wsURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch browser: %w", err)
	}

	b := rod.New().ControlURL(wsURL)
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("connect browser: %w", err)
	}
	return &Session{browser: b}, nil
}

// NewPage opens a blank page bound to ctx (all later ops abort when ctx ends).
func (s *Session) NewPage(ctx context.Context) (*rod.Page, error) {
	page, err := s.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, fmt.Errorf("open page: %w", err)
	}
	return page.Context(ctx), nil
}

// Close shuts the browser process down.
func (s *Session) Close() {
	if s.browser != nil {
		_ = s.browser.Close()
	}
}
