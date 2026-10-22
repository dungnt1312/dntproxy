package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"time"
)

// retryBaseDelay scales the exponential retry delay. Tests set it to 0.
var retryBaseDelay = 100 * time.Millisecond

type httpResult struct {
	Status int
	Body   []byte
	Header http.Header
}

// reqOpts describes retry behaviour of one client:
//   - axios-retry (auth service): network errors only, never timeouts, 3 retries
//   - AWS SDK standard mode (SSO OIDC): network errors, timeouts, 429 and 5xx, 4 attempts
type reqOpts struct {
	client        *http.Client
	maxAttempts   int
	retryTimeouts bool
	retryStatus   func(int) bool
}

func expBackoff(retry int) time.Duration {
	d := retryBaseDelay << (retry - 1)
	if d <= 0 {
		return 0
	}
	return d + rand.N(d/5+1)
}

func isTimeoutErr(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// doRequest sends one logical request with the client's retry policy. HTTP
// error statuses are returned to the caller for classification; only transport
// failures come back as errors.
func doRequest(ctx context.Context, method, url string, body []byte, contentType string, headers map[string]string, o reqOpts) (*httpResult, error) {
	var lastErr error
	for attempt := 1; attempt <= o.maxAttempts; attempt++ {
		var rd io.Reader
		if body != nil {
			rd = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, rd)
		if err != nil {
			return nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := o.client.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil || attempt == o.maxAttempts || (isTimeoutErr(err) && !o.retryTimeouts) {
				return nil, err
			}
			if sleepErr := sleepCtx(ctx, expBackoff(attempt)); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt == o.maxAttempts {
				return nil, readErr
			}
			continue
		}
		res := &httpResult{Status: resp.StatusCode, Body: data, Header: resp.Header}
		if o.retryStatus != nil && o.retryStatus(resp.StatusCode) && attempt < o.maxAttempts {
			if sleepErr := sleepCtx(ctx, expBackoff(attempt)); sleepErr != nil {
				return res, nil
			}
			continue
		}
		return res, nil
	}
	return nil, lastErr
}

// doJSON marshals body as JSON and sends it through doRequest.
func doJSON(ctx context.Context, method, url string, body interface{}, headers map[string]string, o reqOpts) (*httpResult, error) {
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}
	return doRequest(ctx, method, url, raw, "application/json", headers, o)
}

// transportError classifies a transport failure (FailedToConnect / timeout).
func transportError(prefix string, err error) *KiroError {
	if isTimeoutErr(err) {
		return wrapKiroErr(KindNetworkIssue, prefix+": network related issue (timeout)", err)
	}
	return wrapKiroErr(KindNetworkIssue, prefix+": failed to connect", err)
}
