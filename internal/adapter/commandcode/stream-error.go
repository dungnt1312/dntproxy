package commandcode

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func parseErrorEvent(line string) (ccStreamEvent, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return ccStreamEvent{}, false
	}
	var event ccStreamEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return ccStreamEvent{}, false
	}
	if event.Type != "error" {
		return ccStreamEvent{}, false
	}
	return event, true
}

func commandCodeStreamFailure(event ccStreamEvent) (int, error) {
	msg := "upstream error"
	var code *int
	if event.Error != nil {
		if event.Error.Message != "" {
			msg = event.Error.Message
		}
		code = event.Error.StatusCode
	}
	return mapCommandCodeStreamErrorStatus(msg, code), fmt.Errorf("%s", msg)
}

func mapCommandCodeStreamErrorStatus(msg string, eventStatus *int) int {
	if eventStatus != nil && *eventStatus >= 400 && *eventStatus <= 599 {
		return *eventStatus
	}
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "gateway request failed"),
		strings.Contains(lower, "invalid error response format"):
		return http.StatusBadGateway
	case strings.Contains(lower, "insufficient credit"),
		strings.Contains(lower, "quota"):
		return http.StatusPaymentRequired
	case strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "invalid api key"):
		return http.StatusUnauthorized
	default:
		return http.StatusBadGateway
	}
}
