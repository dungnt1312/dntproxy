package service

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dungnt/dntproxy/internal/port"
)

func shouldFallbackToNextAccount(status int, errorText string) bool {
	switch status {
	case 400, 405, 411, 413, 414, 415, 422, 431:
		return false
	}

	lower := strings.ToLower(errorText)
	clientErrorHints := []string{
		"invalid request",
		"improperly formed request",
		"malformed",
		"invalid json",
		"missing required",
		"unsupported parameter",
		"tool schema",
	}
	for _, hint := range clientErrorHints {
		if strings.Contains(lower, hint) {
			return false
		}
	}

	return true
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

func mapSelectionErrorToChatResult(err error) *port.ChatResult {
	var selErr *AccountSelectionError
	if !errors.As(err, &selErr) {
		return nil
	}

	switch selErr.Kind {
	case SelectionErrNoActiveCredentials:
		return &port.ChatResult{StatusCode: http.StatusNotFound, Error: selErr.Error()}
	case SelectionErrUnsupportedModel:
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: selErr.Error()}
	case SelectionErrRateLimited, SelectionErrModelLocked:
		return &port.ChatResult{StatusCode: http.StatusTooManyRequests, Error: selErr.Error()}
	default:
		return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: selErr.Error()}
	}
}
