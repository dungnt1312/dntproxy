package autologin

import (
	"context"
	"errors"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/browser"
	"golang.org/x/sync/errgroup"
)

// run processes every account with a bounded worker pool, then marks the
// job finished (the snapshot stays queryable until the next Start).
func (j *job) run() {
	// Kill browsers orphaned by an earlier hard-killed run before launching
	// new ones (leakless is disabled, so nothing else reaps them).
	if len(j.accounts) > 0 {
		browser.SweepOrphans()
	}

	defer func() {
		j.dispatcher.close()
		j.mu.Lock()
		j.finished = true
		j.mu.Unlock()
		log.Printf("[auto-login] job finished: %d ok, %d failed, %d total",
			j.snapshot().Done, j.snapshot().Failed, len(j.accounts))
	}()

	var group errgroup.Group
	group.SetLimit(j.workers)
	for i := range j.accounts {
		acct := j.accounts[i]
		group.Go(func() error {
			if j.ctx.Err() != nil {
				j.record(AccountResult{Email: acct.Email, Status: "stopped", Error: "stopped by user"})
				return nil
			}
			j.processAccount(acct)
			return nil
		})
	}
	group.Wait()
}

func (j *job) processAccount(acct account) {
	j.mu.Lock()
	j.active = append(j.active, acct.Email)
	j.mu.Unlock()
	defer func() {
		j.mu.Lock()
		for i, e := range j.active {
			if e == acct.Email {
				j.active = append(j.active[:i], j.active[i+1:]...)
				break
			}
		}
		j.mu.Unlock()
	}()

	log.Printf("[auto-login] %s: starting", acct.Email)
	var lastErr error
	for attempt := 1; attempt <= maxLoginAttempts; attempt++ {
		if j.ctx.Err() != nil {
			j.record(AccountResult{Email: acct.Email, Status: "stopped", Error: "stopped by user"})
			return
		}
		if attempt > 1 {
			log.Printf("[auto-login] %s: retry %d/%d (%v)", acct.Email, attempt, maxLoginAttempts, lastErr)
		}

		result, err := j.loginOnce(acct)
		if err == nil {
			log.Printf("[auto-login] %s: success (plan=%s replaced=%v)", acct.Email, result.Plan, result.Replaced)
			j.record(*result)
			return
		}
		lastErr = err

		if !isRetryable(err) {
			log.Printf("[auto-login] %s: failed: %v", acct.Email, err)
			j.record(AccountResult{Email: acct.Email, Status: "error", Error: err.Error()})
			return
		}
	}
	j.record(AccountResult{Email: acct.Email, Status: "error", Error: lastErr.Error()})
}

// loginOnce performs a single full login attempt in a fresh browser.
func (j *job) loginOnce(acct account) (*AccountResult, error) {
	attemptCtx, cancel := context.WithTimeout(j.ctx, attemptTimeout)
	defer cancel()

	verifier, challenge, state, err := auth.GeneratePKCE()
	if err != nil {
		return nil, &browser.LoginError{Stage: "pkce", Msg: err.Error(), Retryable: true}
	}
	callbackCh := j.dispatcher.register(state)
	defer j.dispatcher.unregister(state)

	session, err := browser.NewSession(j.headless)
	if err != nil {
		return nil, &browser.LoginError{Stage: "browser", Msg: err.Error(), Retryable: true}
	}
	j.trackSession(session)
	defer func() {
		j.untrackSession(session)
		session.Close()
	}()

	page, err := session.NewPage(attemptCtx)
	if err != nil {
		return nil, &browser.LoginError{Stage: "browser", Msg: err.Error(), Retryable: true}
	}

	authURL := auth.BuildCodexAuthURL(redirectURI, state, challenge)
	callbackURL, err := browser.LoginOpenAI(attemptCtx, page, authURL, acct.Email, acct.Password, acct.TOTPSecret)
	if err != nil {
		if path := browser.DebugScreenshot(page, acct.Email); path != "" {
			log.Printf("[auto-login] %s: debug screenshot: %s", acct.Email, path)
		}
		if u := browser.PageURL(page); u != "" {
			log.Printf("[auto-login] %s: stalled at %s", acct.Email, u)
		}
		return nil, err
	}

	code := extractCode(callbackURL, state)
	if code == "" {
		// The browser landed on the callback but the dispatcher may not have
		// been reachable — give it a moment in case it delivers the code.
		select {
		case data := <-callbackCh:
			code = data.Code
		case <-time.After(5 * time.Second):
		case <-attemptCtx.Done():
		}
	}
	if code == "" {
		return nil, &browser.LoginError{Stage: "callback", Msg: "no authorization code received", Retryable: true}
	}

	tokens, err := auth.ExchangeOpenAICode(code, redirectURI, verifier)
	if err != nil {
		return nil, &browser.LoginError{Stage: "exchange", Msg: err.Error(), Retryable: false}
	}

	profile := decodeOpenAIProfile(tokens.AccessToken)
	email := profile.Email
	if email == "" {
		email = auth.ExtractEmailFromJWT(tokens.IDToken)
	}
	if email == "" {
		email = acct.Email
	}

	connID, replaced, err := j.upsertConnection(tokens, email, profile, j.owner.TenantID)
	if err != nil {
		return nil, &browser.LoginError{Stage: "save", Msg: err.Error(), Retryable: false}
	}

	return &AccountResult{
		Email:        email,
		Status:       "success",
		Plan:         profile.PlanType,
		ConnectionID: connID,
		Replaced:     replaced,
	}, nil
}

func (j *job) trackSession(s *browser.Session) {
	j.mu.Lock()
	j.sessions[s] = struct{}{}
	j.mu.Unlock()
}

func (j *job) untrackSession(s *browser.Session) {
	j.mu.Lock()
	delete(j.sessions, s)
	j.mu.Unlock()
}

func (j *job) record(result AccountResult) {
	j.mu.Lock()
	defer j.mu.Unlock()
	switch result.Status {
	case "success":
		j.done++
	case "stopped":
		j.stoppedCount++
	default:
		j.failed++
	}
	j.results = append(j.results, result)
}

// extractCode pulls the authorization code from the callback URL, verifying
// the state belongs to this worker.
func extractCode(callbackURL, state string) string {
	parsed, err := url.Parse(callbackURL)
	if err != nil {
		return ""
	}
	q := parsed.Query()
	if s := q.Get("state"); s != "" && s != state {
		return ""
	}
	return q.Get("code")
}

func isRetryable(err error) bool {
	var le *browser.LoginError
	if errors.As(err, &le) {
		return le.Retryable
	}
	return strings.Contains(err.Error(), "context canceled")
}
