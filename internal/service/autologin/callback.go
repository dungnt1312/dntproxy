package autologin

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// callbackData is one OAuth redirect landing on the shared localhost callback.
type callbackData struct {
	Code  string
	State string
	Error string
}

// callbackDispatcher is a single HTTP server on OpenAI's registered callback
// port that routes incoming redirects to the right worker by OAuth state —
// this is what lets N accounts log in in parallel on one port.
type callbackDispatcher struct {
	srv     *http.Server
	mu      sync.Mutex
	pending map[string]chan callbackData
}

func newCallbackDispatcher(port int) *callbackDispatcher {
	d := &callbackDispatcher{pending: make(map[string]chan callbackData)}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", d.handle)
	d.srv = &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}
	return d
}

// start listens in the background. A bind failure is not fatal: workers also
// read the code from the final page URL, only the in-browser success page
// degrades to a connection error.
func (d *callbackDispatcher) start() error {
	errCh := make(chan error, 1)
	go func() {
		if err := d.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (d *callbackDispatcher) close() {
	_ = d.srv.Close()
}

// register returns the channel that will receive this state's callback.
func (d *callbackDispatcher) register(state string) chan callbackData {
	ch := make(chan callbackData, 1)
	d.mu.Lock()
	d.pending[state] = ch
	d.mu.Unlock()
	return ch
}

func (d *callbackDispatcher) unregister(state string) {
	d.mu.Lock()
	delete(d.pending, state)
	d.mu.Unlock()
}

func (d *callbackDispatcher) handle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	state := q.Get("state")
	data := callbackData{
		Code:  q.Get("code"),
		State: state,
		Error: q.Get("error"),
	}

	var delivered bool
	if state != "" {
		d.mu.Lock()
		if ch, ok := d.pending[state]; ok {
			select {
			case ch <- data:
				delivered = true
			default:
			}
		}
		d.mu.Unlock()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if data.Error != "" || !delivered {
		fmt.Fprintf(w, `<!DOCTYPE html><html><body style="font-family:system-ui;background:#1a1a2e;color:#e0e0e0;display:flex;justify-content:center;align-items:center;height:100vh;margin:0"><div style="text-align:center"><h2>❌ Authorization failed</h2><p>%s</p></div></body></html>`, strings.ReplaceAll(data.Error, "<", "&lt;"))
		return
	}
	w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:system-ui;background:#1a1a2e;color:#e0e0e0;display:flex;justify-content:center;align-items:center;height:100vh;margin:0"><div style="text-align:center"><h2>✅ Connected to dntproxy!</h2><p>You can close this tab.</p></div></body></html>`))
}
