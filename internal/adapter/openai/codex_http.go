package openai

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCodexResponseHeaderTimeout = 120 * time.Second
	defaultCodexRetryAttempts          = 1
)

type codexDoFunc func(*http.Request) (*http.Response, error)

var codexHTTPClient = newCodexHTTPClient()

func newCodexHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: codexResponseHeaderTimeout(),
			ForceAttemptHTTP2:     true,
		},
		// No overall timeout: Codex responses stream can run for minutes.
	}
}

func codexResponseHeaderTimeout() time.Duration {
	return durationFromEnvMillis("DNTPROXY_CODEX_RESPONSE_HEADER_TIMEOUT_MS", defaultCodexResponseHeaderTimeout)
}

func codexRetryAttempts() int {
	value := os.Getenv("DNTPROXY_CODEX_RETRY_ATTEMPTS")
	if value == "" {
		return defaultCodexRetryAttempts
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return defaultCodexRetryAttempts
	}
	return n
}

func durationFromEnvMillis(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	ms, err := strconv.Atoi(value)
	if err != nil || ms <= 0 {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}

func shouldRetryCodexRequest(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout awaiting response headers")
}

func doCodexRequestWithRetry(req *http.Request, retries int, delay time.Duration, do codexDoFunc) (*http.Response, error) {
	if retries < 0 {
		retries = 0
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body.Close()

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		attemptReq := req.Clone(req.Context())
		attemptReq.Body = io.NopCloser(bytes.NewReader(body))
		attemptReq.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
		attemptReq.ContentLength = int64(len(body))

		resp, err := do(attemptReq)
		if err == nil || !shouldRetryCodexRequest(err) || attempt == retries {
			return resp, err
		}

		lastErr = err
		log.Printf("[CODEX] retrying request after response-header timeout | attempt=%d/%d err=%s", attempt+1, retries+1, err)
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	return nil, lastErr
}
