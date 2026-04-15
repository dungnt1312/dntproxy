package logger

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/google/uuid"
)

// RequestLog tracks the full lifecycle of a single proxy request.
// Each incoming chat completion request creates one RequestLog that
// records resolution, upstream call, usage, and final status.
type RequestLog struct {
	mu sync.Mutex

	// Identity
	ID        string
	StartTime time.Time

	// Client (inbound)
	ClientMethod string
	ClientPath   string
	RequestModel string // raw model from client, e.g. "sonnet-4"
	BodySize     int

	// Resolution
	ResolvedProvider string // e.g. "kiro"
	ResolvedModel    string // e.g. "Codex-sonnet-4-5-20250514"
	IsCombo          bool
	ComboName        string

	// Account
	ConnectionID   string
	ConnectionName string
	AttemptCount   int // number of account attempts

	// Upstream
	UpstreamURL      string
	UpstreamMethod   string
	UpstreamStatus   int
	UpstreamDuration time.Duration

	// Result
	StatusCode int
	Error      string
	Streaming  bool

	// Usage (filled by stream sniffer callback)
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	UsageSource  string

	// Cost (filled by async writer from price cache)
	CostInput  float64
	CostOutput float64
	CostTotal  float64
	Currency   string

	// Bodies (optional, dev mode only)
	RequestBody  string
	ResponseBody string
}

// NewRequestLog creates a new request log with a unique ID.
func NewRequestLog(requestID string) *RequestLog {
	if requestID == "" {
		requestID = uuid.New().String()
	}
	return &RequestLog{
		ID:        requestID,
		StartTime: time.Now(),
		Currency:  "USD",
	}
}

// Begin records the incoming client request.
func (r *RequestLog) Begin(method, path, model string, bodySize int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ClientMethod = method
	r.ClientPath = path
	r.RequestModel = model
	r.BodySize = bodySize
}

// Route records model resolution results.
func (r *RequestLog) Route(provider, resolvedModel string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ResolvedProvider = provider
	r.ResolvedModel = resolvedModel
}

// RouteCombo records combo resolution.
func (r *RequestLog) RouteCombo(comboName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.IsCombo = true
	r.ComboName = comboName
}

// SelectAccount records which account was selected.
func (r *RequestLog) SelectAccount(connID, connName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ConnectionID = connID
	r.ConnectionName = connName
	r.AttemptCount++
}

// Upstream records upstream request/response info.
func (r *RequestLog) Upstream(url string, method string, status int, duration time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.UpstreamURL = url
	r.UpstreamMethod = method
	r.UpstreamStatus = status
	r.UpstreamDuration = duration
	if err != nil && r.Error == "" {
		r.Error = err.Error()
	}
}

// SetUsage records token usage from stream sniffer.
func (r *RequestLog) SetUsage(input, output int, source string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.InputTokens = input
	r.OutputTokens = output
	r.TotalTokens = input + output
	r.UsageSource = source
}

// SetBodies records request and response bodies for dev debugging.
func (r *RequestLog) SetBodies(reqBody, respBody string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if reqBody != "" {
		r.RequestBody = reqBody
	}
	if respBody != "" {
		r.ResponseBody = respBody
	}
}

// End finalises the request log and emits terminal + structured log entries.
func (r *RequestLog) End(statusCode int, errMsg string) {
	r.mu.Lock()
	r.StatusCode = statusCode
	if errMsg != "" && r.Error == "" {
		r.Error = errMsg
	}
	r.mu.Unlock()

	// Emit terminal output
	terminalPrint(r)

	// Emit structured log entries to the logger (for SQLite + SSE)
	r.emitStructuredLogs()
}

// WrapStream wraps an io.ReadCloser so that End() is called when
// the stream is fully consumed and closed.
func (r *RequestLog) WrapStream(stream io.ReadCloser) io.ReadCloser {
	r.Streaming = true
	return &requestLogStreamWrapper{
		ReadCloser: stream,
		reqlog:     r,
	}
}

// emitStructuredLogs creates LogEntry records for the structured logger.
func (r *RequestLog) emitStructuredLogs() {
	appLogger := Get()

	now := time.Now()
	totalDuration := now.Sub(r.StartTime)

	level := "INFO"
	if r.StatusCode >= 400 || r.Error != "" {
		level = "ERROR"
	}

	// Single consolidated log entry per request
	entry := domain.LogEntry{
		ID:             r.ID,
		Level:          level,
		Provider:       strings.ToUpper(r.ResolvedProvider),
		Direction:      "request",
		Method:         r.ClientMethod,
		Path:           r.ClientPath,
		StatusCode:     r.StatusCode,
		DurationMs:     totalDuration.Milliseconds(),
		ConnectionID:   r.ConnectionID,
		ConnectionName: r.ConnectionName,
		Model:          r.ResolvedModel,
		RequestID:      r.ID,
		Message:        r.buildMessage(),
		Error:          r.Error,
		BodySize:       r.BodySize,
		InputTokens:    r.InputTokens,
		OutputTokens:   r.OutputTokens,
		TotalTokens:    r.TotalTokens,
		UsageSource:    r.UsageSource,
		Currency:       r.Currency,
		RequestBody:    r.RequestBody,
		ResponseBody:   r.ResponseBody,
	}

	if r.ResolvedProvider == "" {
		entry.Provider = "PROXY"
	}

	appLogger.AddEntry(entry)
}

func (r *RequestLog) buildMessage() string {
	parts := []string{}

	// Model routing
	if r.RequestModel != "" {
		if r.ResolvedProvider != "" && r.ResolvedModel != "" {
			resolved := r.ResolvedProvider + "/" + r.ResolvedModel
			if r.RequestModel != resolved {
				parts = append(parts, fmt.Sprintf("%s → %s", r.RequestModel, resolved))
			} else {
				parts = append(parts, r.RequestModel)
			}
		} else {
			parts = append(parts, r.RequestModel)
		}
	}

	// Account
	if r.ConnectionName != "" {
		parts = append(parts, fmt.Sprintf("via %q", r.ConnectionName))
	}

	// Status
	if r.StatusCode > 0 {
		parts = append(parts, fmt.Sprintf("status=%d", r.StatusCode))
	}

	// Duration
	if r.UpstreamDuration > 0 {
		parts = append(parts, fmt.Sprintf("ttfb=%s", r.UpstreamDuration.Round(time.Millisecond)))
	}

	// Usage
	if r.TotalTokens > 0 {
		parts = append(parts, fmt.Sprintf("tokens=%d", r.TotalTokens))
	}

	if len(parts) == 0 {
		return "Request completed"
	}
	return strings.Join(parts, " | ")
}

// requestLogStreamWrapper wraps an io.ReadCloser and calls reqlog.End()
// on close to finalize the request log.
type requestLogStreamWrapper struct {
	io.ReadCloser
	reqlog *RequestLog
	once   sync.Once
}

func (w *requestLogStreamWrapper) Close() error {
	err := w.ReadCloser.Close()
	w.once.Do(func() {
		status := 200
		errMsg := ""
		if err != nil {
			status = 502
			errMsg = err.Error()
		}
		w.reqlog.End(status, errMsg)
	})
	return err
}
