package auth

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// callbackServer is the one-shot loopback listener behind Kiro's AuthSSOServer
// (IdC login): it binds 127.0.0.1 on an OS-assigned port, accepts a single
// /oauth/callback request, validates it and hands back the authorization code.
type callbackServer struct {
	state    string
	listener net.Listener
	srv      *http.Server
	port     int

	result    chan callbackOutcome // buffered(1); first outcome wins
	once      sync.Once
	closeOnce sync.Once
	closed    chan struct{}
}

type callbackOutcome struct {
	code string
	err  error
}

// startCallbackServer listens on 127.0.0.1:<port>. port 0 picks a free port,
// which is what Kiro does for IdC.
func startCallbackServer(state string, port int, timeout time.Duration) (*callbackServer, error) {
	ln, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, wrapKiroErr(KindNetworkIssue,
			"AuthSSOServer: failed to listen to server", err)
	}
	s := &callbackServer{
		state:    state,
		listener: ln,
		port:     ln.Addr().(*net.TCPAddr).Port,
		result:   make(chan callbackOutcome, 1),
		closed:   make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(idcCallbackPath, s.handleCallback)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	s.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() { _ = s.srv.Serve(ln) }()
	// authenticationFlowTimeoutInMs: abandon after 10 minutes.
	go func() {
		select {
		case <-time.After(timeout):
			s.deliver(callbackOutcome{err: newKiroErrUser(KindAbandoned,
				"Timed-out waiting for browser login flow to complete",
				"Authentication session timed out. Please try signing in again.")})
		case <-s.closed:
		}
	}()
	return s, nil
}

// RedirectURI is the exact redirect_uri to send to the authorize endpoint.
func (s *callbackServer) RedirectURI() string {
	return "http://127.0.0.1:" + strconv.Itoa(s.port) + idcCallbackPath
}

func (s *callbackServer) deliver(o callbackOutcome) {
	s.once.Do(func() {
		s.result <- o
		go s.Close()
	})
}

// Close stops the listener. Safe to call repeatedly and from several
// goroutines (deliver, Cancel and the timeout all race to close it).
func (s *callbackServer) Close() {
	s.closeOnce.Do(func() {
		close(s.closed)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			_ = s.srv.Close()
		}
	})
}

// Cancel aborts the pending flow (cancelCurrentFlow).
func (s *callbackServer) Cancel() {
	s.deliver(callbackOutcome{err: newKiroErr(KindCanceled, "user cancellation")})
}

func (s *callbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code, err := validateIdCCallback(r.URL.Query(), s.state)
	if err != nil {
		http.Redirect(w, r, AuthErrorPageURL(UserMessage(err)), http.StatusFound)
		s.deliver(callbackOutcome{err: err})
		return
	}
	http.Redirect(w, r, AuthSuccessPageURL(), http.StatusFound)
	s.deliver(callbackOutcome{code: code})
}

// validateIdCCallback mirrors AuthSSOServer.handleAuthentication.
func validateIdCCallback(q url.Values, expectedState string) (string, error) {
	if e, d := q.Get("error"), q.Get("error_description"); e != "" && d != "" {
		if e == "access_denied" {
			return "", newKiroErrUser(KindInvalidAuth, "Access denied through identity provider",
				"Authentication failed: The identity provider denied access to Kiro. Please ensure you grant all required permissions.")
		}
		return "", newKiroErrUser(KindInvalidAuth, "Failure from identity provider: "+e+": "+d,
			"Authentication failed: The identity provider reported an error: "+e+": "+d)
	}
	code := q.Get("code")
	if code == "" {
		return "", newKiroErrUser(KindInvalidAuth, "AuthSSOServer: missing code",
			"Authentication failed: Please try signing in again.")
	}
	if expectedState != "" {
		st := q.Get("state")
		if st == "" {
			return "", newKiroErrUser(KindInvalidAuth, "AuthSSOServer: missing state",
				"Authentication failed: Please try signing in again.")
		}
		if st != expectedState {
			return "", newKiroErrUser(KindInvalidAuth, "AuthSSOServer: invalid state",
				"Authentication failed: Please try signing in again.")
		}
	}
	return code, nil
}
