package browser

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// LoginError carries the failing stage so callers can decide retries and
// show actionable messages.
type LoginError struct {
	Stage     string
	Msg       string
	Retryable bool
}

func (e *LoginError) Error() string { return e.Stage + ": " + e.Msg }

// CallbackHostPort is OpenAI's registered Codex CLI redirect target.
const CallbackHostPort = "localhost:1455"

var (
	emailSelectors = []string{
		`input[name="email"]`,
		`input[type="email"]`,
		`input[name="username"]`,
		`input[id*="email" i]`,
		`input[autocomplete="username"]`,
		`input[placeholder*="email" i]`,
	}
	passwordSelectors = []string{
		`input[name="password"]`,
		`input[type="password"]`,
		`input[id*="password" i]`,
		`input[autocomplete="current-password"]`,
		`input[placeholder*="password" i]`,
		`input[placeholder*="mật khẩu" i]`,
	}
	totpSelectors = []string{
		`input[name="code"]`,
		`input[inputmode="numeric"]`,
		`input[autocomplete="one-time-code"]`,
		`input[id*="code" i]`,
		`input[placeholder*="code" i]`,
		`input[aria-label*="code" i]`,
	}
	// Buttons that advance the current screen (email → password → consent).
	submitTexts = regexp.MustCompile(`continue|next|log ?in|sign ?in|submit|verify|authorize|allow|accept|tiếp tục|đăng nhập`)
	// Page texts that mean the flow can never succeed.
	hardFailTexts = regexp.MustCompile(`(?i)wrong password|wrong email|incorrect email|incorrect password|couldn'?t authenticate|too many attempts|account (has been )?(locked|disabled|suspended)`)
	// Page texts that indicate an interactive challenge automation cannot solve.
	captchaTexts = regexp.MustCompile(`(?i)verify (that )?you'?re (a )?human|prove you are human|unusual activity|captcha`)
	// Phone-verification gate: OpenAI demands an SMS/WhatsApp one-time code.
	// No automation can pass it, so the account fails immediately, no retries.
	phoneTexts = regexp.MustCompile(`(?i)phone number required|add your phone number|verify your phone|phone number is required`)
	// Buttons that advance the post-login screens: workspace picker,
	// authorization/consent, "allow access". Clicked while waiting for the
	// callback — by then no login-form Continue exists anymore.
	consentTexts = regexp.MustCompile(`continue|authorize|allow|accept|confirm|tiếp tục`)
)

// LoginOpenAI walks auth.openai.com: fill email → password → optional TOTP →
// consent, and returns the callback URL (containing the authorization code)
// once the browser is redirected to the registered localhost callback.
func LoginOpenAI(ctx context.Context, page *rod.Page, authURL, email, password, totpSecret string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", &LoginError{Stage: "start", Msg: err.Error(), Retryable: true}
	}

	if err := page.Navigate(authURL); err != nil {
		return "", &LoginError{Stage: "open", Msg: err.Error(), Retryable: true}
	}
	stableCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_ = page.Context(stableCtx).WaitStable(400 * time.Millisecond)
	page.Context(ctx)

	if err := fillFirstVisible(page, emailSelectors, email, 15*time.Second); err != nil {
		return "", &LoginError{Stage: "email", Msg: err.Error(), Retryable: true}
	}
	submitOrEnter(page)
	if msg := checkHardFail(page); msg != "" {
		return "", &LoginError{Stage: "email", Msg: msg, Retryable: false}
	}

	if err := fillFirstVisible(page, passwordSelectors, password, 20*time.Second); err != nil {
		return "", &LoginError{Stage: "password", Msg: err.Error(), Retryable: false}
	}
	submitOrEnter(page)
	time.Sleep(300 * time.Millisecond)
	if msg := checkHardFail(page); msg != "" {
		return "", &LoginError{Stage: "password", Msg: msg, Retryable: false}
	}

	// Accounts without 2FA simply never show the prompt — keep going either way.
	if totpSecret != "" {
		fillTOTPIfPrompted(page, totpSecret, 12*time.Second)
	}

	return waitCallbackRedirect(ctx, page, 45*time.Second)
}

// fillFirstVisible polls all selectors at once and fills the first visible
// input, mirroring how auth.openai.com swaps screens under shifting markup.
func fillFirstVisible(page *rod.Page, selectors []string, value string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		for _, sel := range selectors {
			els, err := page.Elements(sel)
			if err != nil {
				continue
			}
			for _, el := range els {
				if visible, err := el.Visible(); err != nil || !visible {
					continue
				}
				if err := el.Input(value); err != nil {
					continue
				}
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no visible input after %s", timeout)
		}
		time.Sleep(150 * time.Millisecond)
	}
}

// submitOrEnter clicks the primary submit-style button, falling back to Enter.
func submitOrEnter(page *rod.Page) {
	if clickMatchingButton(page, submitTexts, 2500*time.Millisecond) {
		return
	}
	_ = page.Keyboard.Press(input.Enter)
}

// fillTOTPIfPrompted enters the current TOTP code when (and if) the 2FA
// screen appears. A missing prompt is not an error (2FA may be remembered).
func fillTOTPIfPrompted(page *rod.Page, secret string, timeout time.Duration) {
	code, err := auth.GenerateTOTP(secret, time.Now())
	if err != nil {
		return
	}
	if err := fillFirstVisible(page, totpSelectors, code, timeout); err != nil {
		return
	}
	time.Sleep(150 * time.Millisecond)
	submitOrEnter(page)
	time.Sleep(1200 * time.Millisecond)
}

// waitCallbackRedirect polls the page until OpenAI redirects to the registered
// localhost callback, clicking consent/workspace "Continue"-style buttons as
// those screens appear (the simplified flow still asks "Select a workspace").
// Gates that can never pass (phone verification, captcha) fail fast here.
func waitCallbackRedirect(ctx context.Context, page *rod.Page, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	lastClick := time.Time{}
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return "", &LoginError{Stage: "callback", Msg: err.Error(), Retryable: true}
		}
		if info, err := page.Info(); err == nil && info != nil {
			u := info.URL
			if strings.Contains(u, CallbackHostPort) && strings.Contains(u, "/auth/callback") {
				return u, nil
			}
		}

		if body, err := page.Elements("body"); err == nil && len(body) > 0 {
			if text, err := body[0].Text(); err == nil {
				lower := strings.ToLower(text)
				switch {
				case phoneTexts.MatchString(lower):
					return "", &LoginError{
						Stage:     "verify",
						Msg:       "account requires phone verification — cannot be automated, marked failed",
						Retryable: false,
					}
				case captchaTexts.MatchString(lower):
					return "", &LoginError{
						Stage:     "callback",
						Msg:       "OpenAI is showing a human-verification challenge — try fewer workers or a headed browser",
						Retryable: true,
					}
				case hardFailTexts.MatchString(lower):
					return "", &LoginError{Stage: "callback", Msg: firstMatch(hardFailTexts, text), Retryable: false}
				}
			}
		}

		// Advance consent/workspace screens; 1s spacing avoids double submits.
		if time.Since(lastClick) > time.Second {
			if clickMatchingButton(page, consentTexts, 800*time.Millisecond) {
				lastClick = time.Now()
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", &LoginError{Stage: "callback", Msg: "timeout waiting for OAuth callback", Retryable: true}
}

// clickMatchingButton clicks the first visible button whose text matches re.
func clickMatchingButton(page *rod.Page, re *regexp.Regexp, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		buttons, err := page.Elements("button")
		if err == nil {
			for _, b := range buttons {
				visible, err := b.Visible()
				if err != nil || !visible {
					continue
				}
				text, err := b.Text()
				if err != nil || !re.MatchString(strings.ToLower(text)) {
					continue
				}
				if err := b.Click(proto.InputMouseButtonLeft, 1); err == nil {
					return true
				}
			}
		}
		time.Sleep(120 * time.Millisecond)
	}
	return false
}

// checkHardFail inspects visible page text for terminal login errors so the
// account fails fast instead of waiting out the full timeout.
func checkHardFail(page *rod.Page) string {
	body, err := page.Elements("body")
	if err != nil || len(body) == 0 {
		return ""
	}
	text, err := body[0].Text()
	if err != nil {
		return ""
	}
	if m := firstMatch(hardFailTexts, text); m != "" {
		return m
	}
	return ""
}

func firstMatch(re *regexp.Regexp, text string) string {
	lower := strings.ToLower(text)
	if m := re.FindString(lower); m != "" {
		return m
	}
	return ""
}
