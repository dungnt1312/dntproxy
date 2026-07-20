package shared

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

// StreamingHTTPClient is a shared HTTP client configured for streaming responses.
// It reuses TCP connections across requests and has no client-level timeout
// (which would kill long-running streams). Timeouts are:
//   - ResponseHeaderTimeout: 30s (time to first byte)
//   - IdleConnTimeout: 90s (keep-alive connection reuse)
//   - No overall Timeout (streaming can take minutes)
var StreamingHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second, // Time to first byte
		ForceAttemptHTTP2:     true,
	},
	// No Timeout — streaming responses can take minutes.
	// ResponseHeaderTimeout guards against unresponsive servers.
}

// MaskedToken masks a token string for logging, showing first 4 and last 4 chars.
func MaskedToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

var logBodiesEnabled atomic.Bool // controlled by Settings.LogBodies (UI toggle)

// SetLogBodiesEnabled updates the runtime flag from settings.
// Called at startup and whenever settings are updated via API.
func SetLogBodiesEnabled(enabled bool) {
	logBodiesEnabled.Store(enabled)
}

// ShouldLogBodies returns true when persistent body logging is enabled in settings.
func ShouldLogBodies() bool {
	return logBodiesEnabled.Load()
}

// ShouldLogRawBodies is retained for terminal/debug display only. It does not
// enable persistent request/response body logging.
func ShouldLogRawBodies() bool {
	v := os.Getenv("DNTPROXY_LOG_RAW_BODIES")
	return v == "1" || v == "true"
}

// LoggedBodyMaxBytes returns the max number of bytes to store for request/response bodies.
// Defaults:
//   - body logging enabled: 8KB (sanitized + truncated)
//
// Override with DNTPROXY_LOG_BODY_MAX_BYTES (0 or negative means "no truncation", but still capped to 1MB).
func LoggedBodyMaxBytes() int {
	// Base defaults
	defaultLimit := 8192
	hardCap := 1 * 1024 * 1024

	v := os.Getenv("DNTPROXY_LOG_BODY_MAX_BYTES")
	if v == "" {
		return defaultLimit
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultLimit
	}
	if n <= 0 {
		return hardCap
	}
	if n > hardCap {
		return hardCap
	}
	return n
}

// PrepareLoggedBody sanitizes and truncates a body for persistent logs.
// Returns empty string if body logging is disabled in settings.
func PrepareLoggedBody(b []byte) string {
	if !ShouldLogBodies() {
		return ""
	}
	return TruncateBody(SanitizeBody(b), LoggedBodyMaxBytes())
}

// TruncateBody returns at most maxBytes of the body as a string,
// with a suffix indicating truncation when needed.
func TruncateBody(b []byte, maxBytes int) string {
	if len(b) <= maxBytes {
		return string(b)
	}
	return string(b[:maxBytes]) + fmt.Sprintf("... [truncated %d bytes]", len(b)-maxBytes)
}

// SanitizeBody masks sensitive fields in a JSON request/response body
// to prevent leaking secrets in persisted logs.
func SanitizeBody(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return b // not valid JSON, return as-is
	}

	// Mask sensitive fields in root
	maskFields(raw, []string{
		"api_key", "apiKey", "api-key",
		"access_token", "accessToken", "access-token",
		"refresh_token", "refreshToken", "refresh-token",
		"secret_key", "secretKey", "secret-key",
		"session_token", "sessionToken", "session-token",
		"sessionToken", "SessionToken",
		"authorization", "Authorization",
		"image", "images", "mask", "subject_reference", "image_file",
	})

	// Mask sensitive fields in headers (if present)
	if headers, ok := raw["headers"].(map[string]interface{}); ok {
		maskFields(headers, []string{
			"Authorization", "authorization", "X-Api-Key", "x-api-key",
			"Cookie", "cookie", "X-Amz-Security-Token", "x-amz-security-token",
		})
	}

	// Mask in messages array (common in chat completions)
	if messages, ok := raw["messages"].([]interface{}); ok {
		for _, msg := range messages {
			if m, ok := msg.(map[string]interface{}); ok {
				// Don't mask content, only mask if there are tool_calls with sensitive data
				_ = m
			}
		}
	}

	sanitized, err := json.Marshal(raw)
	if err != nil {
		return b // marshaling failed, return original
	}
	return sanitized
}

// maskFields sets each key in the map to "***REDACTED***" if it exists.
func maskFields(m map[string]interface{}, keys []string) {
	for _, key := range keys {
		if _, exists := m[key]; exists {
			m[key] = "***REDACTED***"
		}
	}
}

// ResponseLevel returns "ERROR" for status >= 400, "INFO" otherwise.
func ResponseLevel(status int) string {
	if status >= 400 {
		return "ERROR"
	}
	return "INFO"
}

// ConnectionToCredentials converts a ProviderConnection to runtime Credentials.
func ConnectionToCredentials(conn *domain.ProviderConnection) *domain.Credentials {
	creds := &domain.Credentials{
		ConnectionID:         conn.ID,
		ConnectionName:       conn.Name,
		Provider:             conn.Provider,
		AccessToken:          conn.AccessToken,
		RefreshToken:         conn.RefreshToken,
		APIKey:               conn.APIKey,
		BaseURL:              conn.BaseURL,
		ModelPrefix:          conn.ModelPrefix,
		ProviderSpecificData: conn.ProviderSpecificData,
	}

	// Extract profileArn from providerSpecificData
	if conn.ProviderSpecificData != nil {
		if v, ok := conn.ProviderSpecificData["profileArn"]; ok {
			if s, ok := v.(string); ok {
				creds.ProfileArn = s
			}
		}
	}

	return creds
}
