package http

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// sseKeepAliveInterval is how long a streaming response may stay silent before
// dntproxy writes an SSE comment line. Upstreams such as Kiro can think for a
// minute or more before the first visible token (reasoning is not forwarded),
// and clients/gateways cut connections that look idle. Comment lines (": ...")
// are ignored by SSE parsers, so they keep the pipe alive without changing the
// stream. DNTPROXY_SSE_KEEPALIVE_MS overrides it; 0 disables.
var sseKeepAliveInterval = func() time.Duration {
	if v, ok := os.LookupEnv("DNTPROXY_SSE_KEEPALIVE_MS"); ok {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 15 * time.Second
}()

const sseKeepAliveLine = ": keep-alive\n\n"

// sseHeartbeat serialises writes to a streaming response and fills silent gaps
// with keep-alive comments. Every real write must happen between Lock and
// Unlock; Unlock records the activity so a heartbeat only fires after a full
// interval of silence.
type sseHeartbeat struct {
	w        gin.ResponseWriter
	interval time.Duration

	mu       sync.Mutex
	last     time.Time
	lastSize int // w.Size() at the last observed write

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

// startSSEHeartbeat begins sending keep-alives on w. With interval <= 0 it only
// provides the write lock.
func startSSEHeartbeat(w gin.ResponseWriter, interval time.Duration) *sseHeartbeat {
	h := &sseHeartbeat{w: w, interval: interval, last: time.Now(), lastSize: w.Size(), stop: make(chan struct{}), done: make(chan struct{})}
	if interval <= 0 {
		close(h.done)
		return h
	}
	go h.run()
	return h
}

func (h *sseHeartbeat) run() {
	defer close(h.done)
	// Check a few times per interval so the silence never runs much past it.
	tick := h.interval / 3
	if tick <= 0 {
		tick = h.interval
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-t.C:
			h.mu.Lock()
			if time.Since(h.last) >= h.interval {
				if _, err := h.w.Write([]byte(sseKeepAliveLine)); err != nil {
					h.mu.Unlock()
					return // client is gone; the handler will notice on its own
				}
				h.w.Flush()
				h.last = time.Now()
				h.lastSize = h.w.Size()
			}
			h.mu.Unlock()
		}
	}
}

// Lock takes the write lock before writing real stream data.
func (h *sseHeartbeat) Lock() { h.mu.Lock() }

// Unlock releases the write lock. It counts as activity only if bytes were
// actually written, so upstream chunks that produce no client output (usage-
// only chunks, dropped events) do not suppress the keep-alive.
func (h *sseHeartbeat) Unlock() {
	if s := h.w.Size(); s != h.lastSize {
		h.lastSize = s
		h.last = time.Now()
	}
	h.mu.Unlock()
}

// Stop ends the heartbeat and waits for it, so the caller owns the writer again.
func (h *sseHeartbeat) Stop() {
	h.stopOnce.Do(func() { close(h.stop) })
	<-h.done
}
