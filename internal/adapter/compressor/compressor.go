package compressor

import (
	"sync/atomic"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/compressor/filters"
)

// Compressor intercepts request bodies and compresses tool result content.
type Compressor struct {
	opts     atomic.Pointer[Options]
	loader   func() Options
	lastLoad atomic.Int64 // UnixNano of last refresh
}

// New creates a Compressor with static options (for tests and direct construction).
func New(opts Options) *Compressor {
	if opts.MinContentLength <= 0 {
		opts.MinContentLength = 500
	}
	c := &Compressor{}
	c.opts.Store(&opts)
	return c
}

// NewWithLoader creates a Compressor whose options refresh at most once per second.
// load must return the current Options; it is called on each Compress invocation
// at most once per second to respect live settings changes.
func NewWithLoader(load func() Options) *Compressor {
	opts := load()
	if opts.MinContentLength <= 0 {
		opts.MinContentLength = 500
	}
	c := &Compressor{loader: load}
	c.opts.Store(&opts)
	c.lastLoad.Store(time.Now().UnixNano())
	return c
}

// Compress rewrites tool result content in body.
// Returns the original body unchanged on any parse error or when disabled.
func (c *Compressor) Compress(body []byte) ([]byte, Stats) {
	c.maybeRefresh()
	opts := c.opts.Load()
	if !opts.Enabled {
		return body, Stats{}
	}
	return walkAndCompress(body, c)
}

// currentOpts returns a copy of the current options (lock-free).
func (c *Compressor) currentOpts() Options {
	return *c.opts.Load()
}

// maybeRefresh re-reads settings from the loader at most once per second.
func (c *Compressor) maybeRefresh() {
	if c.loader == nil {
		return
	}
	last := time.Unix(0, c.lastLoad.Load())
	if time.Since(last) < time.Second {
		return
	}
	opts := c.loader()
	if opts.MinContentLength <= 0 {
		opts.MinContentLength = 500
	}
	c.opts.Store(&opts)
	c.lastLoad.Store(time.Now().UnixNano())
}

// applyFilter dispatches content to the appropriate filter for ct.
// A deferred recover() guards against regex panics or other filter bugs.
func (c *Compressor) applyFilter(content string, ct ContentType) (out string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			out, ok = content, false
		}
	}()
	switch ct {
	case ContentGitDiff, ContentGitStatus, ContentGitLog:
		return filters.GitFilter(content)
	case ContentGoTest, ContentCargoTest, ContentPytest:
		return filters.TestFilter(content)
	case ContentLS:
		return filters.LsFilter(content)
	case ContentLog:
		return filters.LogFilter(content)
	case ContentJSON:
		return filters.JSONFilter(content)
	default:
		return content, false
	}
}
