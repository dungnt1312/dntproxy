package byteplus

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
)

var bytePlusSensitiveURLPattern = regexp.MustCompile(`(?i)(?:https?://|data:image/)[^\s"'<>]+`)

type imageResponse struct {
	Data  []imageData `json:"data"`
	Error *errorBody  `json:"error"`
}

type imageData struct {
	URL     string     `json:"url"`
	B64JSON string     `json:"b64_json"`
	Error   *errorBody `json:"error"`
}

type errorBody struct {
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
}

// APIError is an error returned in ModelArk's JSON envelope.
type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return "BytePlus API error: " + e.Message
	}
	if e.Message == "" {
		return "BytePlus API error: " + e.Code
	}
	return fmt.Sprintf("BytePlus API error %s: %s", e.Code, e.Message)
}

// ParseImageResponse converts a non-streaming ModelArk response to the common
// OpenAI-compatible image result type.
func ParseImageResponse(body []byte) ([]domain.ImageResult, error) {
	var response imageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("parse BytePlus image response: %w", err)
	}
	if response.Error != nil {
		return nil, newAPIError(response.Error)
	}

	results := make([]domain.ImageResult, 0, len(response.Data))
	for index, item := range response.Data {
		if item.Error != nil {
			return nil, fmt.Errorf("BytePlus image %d failed: %w", index, newAPIError(item.Error))
		}
		urlValue := strings.TrimSpace(item.URL)
		base64Value := strings.TrimSpace(item.B64JSON)
		if urlValue != "" && base64Value != "" {
			return nil, fmt.Errorf("BytePlus image %d returned both url and b64_json", index)
		}
		switch {
		case urlValue != "":
			results = append(results, domain.ImageResult{URL: urlValue})
		case base64Value != "":
			results = append(results, domain.ImageResult{B64JSON: base64Value})
		default:
			return nil, fmt.Errorf("BytePlus image %d did not contain output", index)
		}
	}
	if len(results) == 0 {
		return nil, errors.New("BytePlus response did not contain image output")
	}
	return results, nil
}

func newAPIError(body *errorBody) *APIError {
	return &APIError{Code: parseErrorCode(body.Code), Message: sanitizeBytePlusMessage(body.Message)}
}

func sanitizeBytePlusMessage(message string) string {
	message = strings.TrimSpace(message)
	message = bytePlusSensitiveURLPattern.ReplaceAllString(message, "[redacted-url]")
	if len(message) > 1000 {
		message = message[:1000]
	}
	return message
}

func sanitizeBytePlusBody(body []byte) []byte {
	return bytePlusSensitiveURLPattern.ReplaceAll(body, []byte("[redacted-url]"))
}

func parseErrorCode(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return strings.TrimSpace(string(raw))
}

// HTTPStatus maps common ModelArk error codes to an OpenAI-compatible status.
func HTTPStatus(err error) int {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return 0
	}
	code := strings.ToLower(apiErr.Code)
	switch {
	case strings.Contains(code, "authentication"), strings.Contains(code, "unauthorized"),
		strings.Contains(code, "invalidapi"), code == strconv.Itoa(http.StatusUnauthorized):
		return http.StatusUnauthorized
	case strings.Contains(code, "ratelimit"), strings.Contains(code, "too_many"),
		code == strconv.Itoa(http.StatusTooManyRequests):
		return http.StatusTooManyRequests
	case strings.Contains(code, "invalid"), strings.Contains(code, "badrequest"),
		code == strconv.Itoa(http.StatusBadRequest):
		return http.StatusBadRequest
	default:
		return 0
	}
}
