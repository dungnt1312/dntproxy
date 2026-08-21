package service

import (
	"errors"
	"net/http"

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
	message := errMsg
	if message == "" {
		message = http.StatusText(status)
	}
	if message == "" {
		message = "request failed"
	}
	return status, message
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
		return &port.ChatResult{StatusCode: http.StatusTooManyRequests, Error: selErr.Error()}
	default:
		return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: selErr.Error()}
	}
}
