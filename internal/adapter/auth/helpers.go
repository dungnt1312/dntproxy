package auth

import (
	"io"
	"net/url"
	"strings"
)

// formReader wraps url.Values as an io.Reader for form-encoded POST bodies.
type formReader struct {
	data string
	pos  int
}

func newFormReader(values url.Values) *formReader {
	return &formReader{data: values.Encode()}
}

func (r *formReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// Close implements io.Closer (no-op).
func (r *formReader) Close() error {
	return nil
}

// ParseResetTime parses various reset time formats to RFC3339 string.
func ParseResetTime(resetValue interface{}) string {
	if resetValue == nil {
		return ""
	}

	switch v := resetValue.(type) {
	case string:
		return v
	case float64:
		// Unix timestamp in seconds
		return ""
	case int64:
		// Unix timestamp in milliseconds
		return ""
	default:
		return ""
	}
}

// MaskToken masks a token for logging (shows first 4 and last 4 chars).
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

// ExtractProvider extracts provider name from auth method.
func ExtractProvider(authMethod string) string {
	switch strings.ToLower(authMethod) {
	case "builder-id", "idc", "google", "github", "imported":
		return "kiro"
	case "openai", "openai-oauth":
		return "openai"
	default:
		return authMethod
	}
}
