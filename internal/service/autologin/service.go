// Package autologin bulk-logs-in OpenAI/Codex accounts with an automated
// browser and imports the resulting OAuth tokens as dntproxy connections.
// Accounts are pasted as "email|password|2fa_secret" lines; a worker pool
// drives one isolated browser per account through the OAuth PKCE flow.
package autologin

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/browser"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

const (
	callbackPort     = 1455
	redirectURI      = "http://localhost:1455/auth/callback"
	maxWorkers       = 10
	attemptTimeout   = 150 * time.Second
	maxLoginAttempts = 3
	// An existing connection only counts as "healthy, skip it" when its token
	// outlives this horizon — anything closer to expiry is better off being
	// re-logged-in for a fresh token.
	skipHealthHorizon = 24 * time.Hour
)

// Owner identifies the dashboard principal that started a job; only it may
// poll status or stop the run.
type Owner struct {
	TenantID string
	KeyID    string
}

// AccountResult is the per-account outcome reported to the dashboard.
type AccountResult struct {
	Email        string `json:"email"`
	Status       string `json:"status"` // "success" | "error" | "skipped" | "stopped"
	Plan         string `json:"plan,omitempty"`
	ConnectionID string `json:"connectionId,omitempty"`
	Replaced     bool   `json:"replaced"` // true = refreshed an existing connection
	Error        string `json:"error,omitempty"`
}

// Status is a snapshot of job progress.
type Status struct {
	Running   bool            `json:"running"`
	Stopped   bool            `json:"stopped"`
	Total     int             `json:"total"`
	Done      int             `json:"done"`
	Failed    int             `json:"failed"`
	Cancelled int             `json:"cancelled"`
	Skipped   int             `json:"skipped"`
	Workers   int             `json:"workers"`
	Headless  bool            `json:"headless"`
	Active    []string        `json:"active"`
	Results   []AccountResult `json:"results"`
}

type job struct {
	owner      Owner
	store      port.CredentialStore
	accounts   []account
	totalAll   int // pasted account count, including pre-skipped ones
	workers    int
	headless   bool
	ctx        context.Context
	cancel     context.CancelFunc
	dispatcher *callbackDispatcher
	sessions   map[*browser.Session]struct{}

	mu           sync.Mutex
	done         int
	failed       int
	stopped      bool
	stoppedCount int
	skippedCount int
	finished     bool
	active       []string
	results      []AccountResult
}

// Service runs one bulk auto-login job at a time.
type Service struct {
	store port.CredentialStore

	mu  sync.Mutex
	job *job
}

// NewService creates the singleton bulk-login service.
func NewService(store port.CredentialStore) *Service {
	return &Service{store: store}
}

// Start validates and launches a new bulk run. Returns an error when a run is
// already active or no valid account line was provided. With skipExisting,
// pasted accounts that already have a healthy openai OAuth connection (same
// tenant) are recorded as "skipped" without touching a browser.
func (s *Service) Start(lines []string, workers int, headless bool, skipExisting bool, owner Owner) (Status, error) {
	accounts, problems := ParseAccountLines(lines)
	if len(accounts) == 0 {
		detail := "no valid account lines"
		for _, p := range problems {
			detail += "; " + p
		}
		return Status{}, fmt.Errorf("%s", detail)
	}
	if workers < 1 {
		workers = 1
	}
	if workers > maxWorkers {
		workers = maxWorkers
	}

	toProcess := accounts
	var skipped []AccountResult
	if skipExisting {
		if cfg, err := s.store.Load(); err != nil {
			log.Printf("[auto-login] skip-existing check unavailable (%v) — processing every account", err)
		} else {
			var relevant []domain.ProviderConnection
			for _, c := range cfg.ProviderConnections {
				if c.Provider == "openai" && c.AuthType == "oauth" && c.TenantID == owner.TenantID {
					relevant = append(relevant, c)
				}
			}
			toProcess, skipped = classifyAccounts(accounts, relevant, time.Now())
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job != nil && s.job.isRunning() {
		return s.job.snapshot(), fmt.Errorf("a bulk auto-login job is already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	j := &job{
		owner:        owner,
		store:        s.store,
		accounts:     toProcess,
		totalAll:     len(accounts),
		workers:      workers,
		headless:     headless,
		ctx:          ctx,
		cancel:       cancel,
		dispatcher:   newCallbackDispatcher(callbackPort),
		sessions:     make(map[*browser.Session]struct{}),
		skippedCount: len(skipped),
		results:      skipped,
	}
	if err := j.dispatcher.start(); err != nil {
		// Code capture falls back to parsing the browser's final URL.
		log.Printf("[auto-login] callback port %d unavailable (%v) — falling back to URL parsing", callbackPort, err)
	}
	s.job = j
	go j.run()
	if len(skipped) > 0 {
		log.Printf("[auto-login] skipped %d already-healthy account(s), processing %d", len(skipped), len(toProcess))
	}
	return j.snapshot(), nil
}

// Stop cancels the running job and kills its browsers. Idempotent.
func (s *Service) Stop(owner Owner) (Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job == nil {
		return Status{}, false
	}
	if !sameOwner(s.job.owner, owner) {
		return s.job.snapshot(), false
	}
	s.job.stop()
	return s.job.snapshot(), true
}

// Status returns the current/last job snapshot scoped to the caller.
func (s *Service) Status(owner Owner) (Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job == nil {
		return Status{Active: []string{}, Results: []AccountResult{}}, false
	}
	if !sameOwner(s.job.owner, owner) {
		return Status{}, false
	}
	return s.job.snapshot(), true
}

func sameOwner(a, b Owner) bool {
	return a.TenantID == b.TenantID && a.KeyID == b.KeyID
}

func (j *job) isRunning() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return !j.finished
}

func (j *job) stop() {
	j.mu.Lock()
	if j.stopped || j.finished {
		j.mu.Unlock()
		return
	}
	j.stopped = true
	sessions := make([]*browser.Session, 0, len(j.sessions))
	for s := range j.sessions {
		sessions = append(sessions, s)
	}
	j.mu.Unlock()

	log.Printf("[auto-login] stop requested — cancelling %d browser(s)", len(sessions))
	j.cancel()
	for _, s := range sessions {
		go s.Close()
	}
}

func (j *job) snapshot() Status {
	j.mu.Lock()
	defer j.mu.Unlock()
	st := Status{
		Running:   !j.finished,
		Stopped:   j.stopped,
		Total:     j.totalAll,
		Done:      j.done,
		Failed:    j.failed,
		Cancelled: j.stoppedCount,
		Skipped:   j.skippedCount,
		Workers:   j.workers,
		Headless:  j.headless,
		Active:    append([]string{}, j.active...),
		Results:   append([]AccountResult{}, j.results...),
	}
	if st.Results == nil {
		st.Results = []AccountResult{}
	}
	if st.Active == nil {
		st.Active = []string{}
	}
	return st
}
