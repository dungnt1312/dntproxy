package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/google/uuid"
)

// kiroMaxAttempts is 1 initial attempt + 1 retry on response-header timeout.
const kiroMaxAttempts = 2

// Executor handles making requests to Kiro (AWS CodeWhisperer) API.
type Executor struct{}

// NewExecutor creates a new Kiro executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// buildKiroRequest creates the upstream request with AWS-style SDK headers.
// attempt (1-based) is echoed in Amz-Sdk-Request for server-side diagnostics.
func buildKiroRequest(ctx context.Context, url string, payloadBytes []byte, credentials *domain.Credentials, attempt int) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.amazon.eventstream")
	req.Header.Set("User-Agent", "AWS-SDK-JS/3.0.0 kiro-ide/1.0.0")
	req.Header.Set("X-Amz-User-Agent", "aws-sdk-js/3.0.0 kiro-ide/1.0.0")
	req.Header.Set("Amz-Sdk-Request", fmt.Sprintf("attempt=%d; max=%d", attempt, kiroMaxAttempts))
	req.Header.Set("Amz-Sdk-Invocation-Id", uuid.New().String())

	// Only the legacy CodeWhisperer surface expects the RPC target header; the
	// Q and runtime hosts reject requests that carry it.
	if strings.Contains(url, "://codewhisperer.") {
		req.Header.Set("X-Amz-Target", "AmazonCodeWhispererStreamingService.GenerateAssistantResponse")
	}

	if IsAPIKeyAuth(credentials) {
		if key := APIKeyValue(credentials); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
			req.Header.Set("TokenType", "API_KEY")
		}
		return req, nil
	}

	if credentials.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
		if credentials.GetAuthMethod() == AuthMethodExternalIDP {
			req.Header.Set("TokenType", "EXTERNAL_IDP")
		}
	}

	return req, nil
}

// Execute sends a translated request to Kiro and returns a streaming reader
// that emits OpenAI-compatible SSE data.
func (e *Executor) Execute(ctx context.Context, model string, body []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Parse the OpenAI request
	var openaiReq OpenAIRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, 400, fmt.Errorf("parse request: %w", err)
	}

	// Translate to Kiro format
	kiroPayload, err := BuildKiroPayload(&openaiReq, model, credentials)
	if err != nil {
		return nil, 400, fmt.Errorf("build kiro payload: %w", err)
	}

	payloadBytes, err := json.Marshal(kiroPayload)
	if err != nil {
		return nil, 500, fmt.Errorf("marshal kiro payload: %w", err)
	}

	reqlog.SetBodies(shared.PrepareLoggedBody(payloadBytes), "")

	resp, status, err := e.send(ctx, payloadBytes, credentials, reqlog)
	if err != nil {
		return nil, status, err
	}

	// Look up model definition for context window (used in token estimation)
	ctxWindow := domain.GetModelContextWindow(model, "kiro")

	// A client that explicitly asked for stream=false is aggregated downstream, so
	// no bytes reach it until the whole turn is buffered. That lets us verify
	// stream integrity before returning: if the upstream truncated mid-answer we
	// fail the attempt with a retryable error so the chat service rotates to
	// another account, instead of handing back a cut-off turn. Streaming requests
	// cannot do this (bytes are already flushed), so they keep the pipe path and
	// the Flush()-side "length" finish reason.
	if openaiReq.Stream != nil && !*openaiReq.Stream {
		return e.executeBuffered(model, ctxWindow, resp, reqlog)
	}

	// Create a pipe to transform EventStream → SSE
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		defer resp.Body.Close()

		transformer := e.newTransformer(model, ctxWindow, reqlog)

		if err := e.transformStream(resp.Body, transformer, func(chunk []byte) error {
			_, writeErr := pw.Write(chunk)
			return writeErr
		}); err != nil {
			pw.CloseWithError(err)
			return
		}

		// Flush remaining
		flushChunks := transformer.Flush()
		for _, chunk := range flushChunks {
			pw.Write(chunk)
		}
	}()

	return pr, 200, nil
}

// executeBuffered fully consumes and transforms an upstream response in memory,
// then returns it as a single reader. It runs only for non-streaming requests,
// where nothing has been sent to the client yet, so a detected truncation can be
// surfaced as a retryable error for account failover rather than a cut-off turn.
func (e *Executor) executeBuffered(model string, ctxWindow int, resp *http.Response, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
	defer resp.Body.Close()

	transformer := e.newTransformer(model, ctxWindow, reqlog)

	var out bytes.Buffer
	if err := e.transformStream(resp.Body, transformer, func(chunk []byte) error {
		out.Write(chunk)
		return nil
	}); err != nil {
		return nil, 502, fmt.Errorf("kiro stream read failed: %w", err)
	}

	for _, chunk := range transformer.Flush() {
		out.Write(chunk)
	}

	// Truncation is a soft upstream failure: fail over without penalizing the
	// account (CheckFallbackError treats the sentinel as a no-penalty retry).
	if transformer.WasTruncated() {
		log.Printf("[KIRO] buffered truncation: model=%s — failing over to next account", model)
		return nil, 502, fmt.Errorf("returned 502: %s", domain.TruncatedStreamError(model, transformer.TotalContentBytes()))
	}

	return io.NopCloser(&out), 200, nil
}

// newTransformer builds a ResponseTransformer wired to the request logger's usage
// and payload hooks.
func (e *Executor) newTransformer(model string, ctxWindow int, reqlog port.RequestLogger) *ResponseTransformer {
	transformer := NewResponseTransformer(model, ctxWindow)
	transformer.SetUsageCallback(func(usage UsageReport) {
		reqlog.SetUsage(usage.InputTokens, usage.OutputTokens, usage.Source)
	})
	transformer.SetPayloadCallback(func(payload PayloadReport) {
		reqlog.SetBodies("", payload.ResponsePreview)
	})
	return transformer
}

// transformStream reads AWS EventStream frames from r, transforms each into SSE
// chunks, and hands them to emit. It does not call Flush — the caller decides
// what to do with the terminal chunks. Returns a non-nil error only on a
// transport or parse failure (a clean EOF is success).
func (e *Executor) transformStream(r io.Reader, transformer *ResponseTransformer, emit func([]byte) error) error {
	buf := make([]byte, 0, 64*1024)
	readBuf := make([]byte, 32*1024)

	for {
		n, err := r.Read(readBuf)
		if n > 0 {
			buf = append(buf, readBuf[:n]...)
			if len(buf) > maxEventStreamFrame {
				return fmt.Errorf("eventstream buffer exceeded")
			}

			frames, remaining, parseErr := ParseEventFrames(buf)
			if parseErr != nil {
				return parseErr
			}
			buf = remaining

			for _, frame := range frames {
				for _, chunk := range transformer.TransformFrame(&frame) {
					if emitErr := emit(chunk); emitErr != nil {
						return emitErr
					}
				}
			}
		}

		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}

// send posts the payload to the ordered Kiro endpoints for this credential and
// returns the first response that is not an auth/routing rejection.
//
// API-key credentials are only accepted by some of the three surfaces, and the
// working one differs from the OAuth default, so 401/403/404 walk to the next
// host instead of failing the account. Any other status is terminal — notably
// 400, which the legacy CodeWhisperer host returns for a payload that the Q
// host accepts.
func (e *Executor) send(ctx context.Context, payloadBytes []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (*http.Response, int, error) {
	endpoints := OrderedEndpoints(credentials)

	var lastStatus int
	var lastErr error

	for i, url := range endpoints {
		resp, status, err := e.sendTo(ctx, url, payloadBytes, credentials, reqlog)
		if err == nil {
			return resp, status, nil
		}

		lastStatus, lastErr = status, err

		hasNext := i < len(endpoints)-1
		if !hasNext || !shouldTryNextEndpoint(status) {
			break
		}
		log.Printf("[KIRO] %s returned %d, trying next endpoint", url, status)
	}

	return nil, lastStatus, lastErr
}

// sendTo performs a single endpoint attempt, retrying once on a response-header
// timeout (transient upstream stall, not an account failure).
func (e *Executor) sendTo(ctx context.Context, url string, payloadBytes []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (*http.Response, int, error) {
	req, err := buildKiroRequest(ctx, url, payloadBytes, credentials, 1)
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	start := time.Now()
	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil && shared.IsResponseHeaderTimeout(err) {
		reqlog.Upstream(url, "POST", 502, duration, err)
		log.Printf("[KIRO] response-header timeout, retrying once: %v", err)
		if retryReq, retryErr := buildKiroRequest(ctx, url, payloadBytes, credentials, 2); retryErr == nil {
			start = time.Now()
			resp, err = shared.StreamingHTTPClient.Do(retryReq)
			duration = time.Since(start)
		}
	}

	if err != nil {
		reqlog.Upstream(url, "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("kiro request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, errRead := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()

		respBodyStr := "Unknown error"
		if errRead == nil {
			respBodyStr = string(bodyBytes)
		}

		reqlog.SetBodies("", shared.PrepareLoggedBody(bodyBytes))
		errUpstream := fmt.Errorf("%s", respBodyStr)
		reqlog.Upstream(url, "POST", resp.StatusCode, duration, errUpstream)

		// Honor a server-provided Retry-After (RFC 7231 seconds or HTTP date) by
		// embedding a machine-readable hint the cooldown logic picks up. Kept out
		// of the human-readable message and stripped before reaching the client.
		errText := fmt.Sprintf("returned %d: %s", resp.StatusCode, respBodyStr)
		if hintMs, ok := domain.ParseRetryAfterHeader(resp.Header.Get("Retry-After")); ok {
			errText = domain.AppendRetryAfterHint(errText, hintMs)
		}

		return nil, resp.StatusCode, fmt.Errorf("%s", errText)
	}

	reqlog.Upstream(url, "POST", resp.StatusCode, duration, nil)
	return resp, resp.StatusCode, nil
}
