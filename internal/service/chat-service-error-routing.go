package service

import (
	"errors"
	"net/http"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

func shouldFallbackToNextAccount(status int, errorText string) bool {
	return domain.IsRetryableUpstream(status, errorText)
}

// shouldStopCredentialRetry reports whether the per-model credential retry budget
// is exhausted. max==0 means unlimited; max==N stops after N distinct connections.
func shouldStopCredentialRetry(attempted int, max int) bool {
	return max > 0 && attempted >= max
}

func normalizeExecutorFailure(status int, errMsg string) (int, string) {
	if status <= 0 {
		status = http.StatusBadGateway
	}
	// Drop the internal Retry-After marker so it never reaches the client body.
	message := domain.StripRetryAfterHint(errMsg)
	if message == "" {
		message = http.StatusText(status)
	}
	if message == "" {
		message = "request failed"
	}
	return status, message
}

// retryAfterFromError converts an embedded Retry-After hint into an absolute
// RFC3339 timestamp for surfacing to the client, or "" when none is present.
func retryAfterFromError(errMsg string) string {
	if ms, ok := domain.ExtractRetryAfterHint(errMsg); ok {
		return domain.RetryAfterTimestamp(ms)
	}
	return ""
}

// selectionRetryAfter extracts the earliest-recovery hint from a selection
// error, or "" when the error is not about a temporary cooldown.
func selectionRetryAfter(err error) string {
	var selErr *AccountSelectionError
	if errors.As(err, &selErr) {
		return selErr.RetryAfter
	}
	return ""
}

// earlierRetryAfter returns the sooner of two RFC3339 hints; unparseable values
// fall through to whichever parses, and "" when neither does.
func earlierRetryAfter(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	ta, errA := time.Parse(time.RFC3339, a)
	tb, errB := time.Parse(time.RFC3339, b)
	switch {
	case errA != nil:
		return b
	case errB != nil:
		return a
	case tb.Before(ta):
		return b
	default:
		return a
	}
}

// mapSelectionErrorToChatResult maps AccountSelectionError to a ChatResult for
// legacy callers that return port.ChatResult directly.
func mapSelectionErrorToChatResult(err error) *port.ChatResult {
	var selErr *AccountSelectionError
	if !errors.As(err, &selErr) {
		return nil
	}

	switch selErr.Kind {
	case SelectionErrNoActiveCredentials:
		return &port.ChatResult{StatusCode: http.StatusNotFound, Error: selErr.Error()}
	case SelectionErrNoAllowedConnection:
		return &port.ChatResult{StatusCode: http.StatusForbidden, Error: selErr.Error()}
	case SelectionErrUnsupportedModel:
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: selErr.Error()}
	case SelectionErrRateLimited, SelectionErrModelLocked:
		return &port.ChatResult{StatusCode: http.StatusTooManyRequests, Error: selErr.Error(), RetryAfter: selErr.RetryAfter}
	default:
		return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: selErr.Error()}
	}
}
