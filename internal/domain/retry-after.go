package domain

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// retryAfterMarker is an internal, machine-readable hint appended to an upstream
// error string so the cooldown logic can honor a server-provided Retry-After
// without changing the executor→service error contract (a plain string). It is
// stripped before any error text is surfaced to a client.
const retryAfterMarker = "[retry-after-ms="

// retryAfterMaxMs caps a server-provided Retry-After so a hostile or buggy
// upstream cannot pin an account out for an unreasonable time. Operators can
// still shorten it further via MaxCooldownSeconds.
const retryAfterMaxMs = 3600000 // 1 hour

// ParseRetryAfterHeader converts a Retry-After header value into milliseconds.
// It accepts both forms from RFC 7231: a delay in seconds ("120") or an HTTP
// date ("Wed, 21 Oct 2026 07:28:00 GMT"). Returns (0, false) when the value is
// empty, malformed, or in the past.
func ParseRetryAfterHeader(value string) (int, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0, false
		}
		return capRetryAfterMs(secs * 1000), true
	}
	if t, err := http.ParseTime(v); err == nil {
		ms := int(time.Until(t).Milliseconds())
		if ms <= 0 {
			return 0, false
		}
		return capRetryAfterMs(ms), true
	}
	return 0, false
}

func capRetryAfterMs(ms int) int {
	if ms > retryAfterMaxMs {
		return retryAfterMaxMs
	}
	return ms
}

// AppendRetryAfterHint appends the machine-readable Retry-After marker to an
// error string. A non-positive value is a no-op so callers can pass the result
// of ParseRetryAfterHeader unconditionally.
func AppendRetryAfterHint(errorText string, ms int) string {
	if ms <= 0 {
		return errorText
	}
	return fmt.Sprintf("%s %s%d]", errorText, retryAfterMarker, ms)
}

// ExtractRetryAfterHint returns the Retry-After hint (in ms) embedded by
// AppendRetryAfterHint, or (0, false) when none is present.
func ExtractRetryAfterHint(errorText string) (int, bool) {
	idx := strings.Index(errorText, retryAfterMarker)
	if idx < 0 {
		return 0, false
	}
	rest := errorText[idx+len(retryAfterMarker):]
	end := strings.IndexByte(rest, ']')
	if end < 0 {
		return 0, false
	}
	ms, err := strconv.Atoi(rest[:end])
	if err != nil || ms <= 0 {
		return 0, false
	}
	return ms, true
}

// StripRetryAfterHint removes the machine-readable marker so the error text is
// safe to surface to a client. Safe to call on strings without a marker.
func StripRetryAfterHint(errorText string) string {
	idx := strings.Index(errorText, retryAfterMarker)
	if idx < 0 {
		return errorText
	}
	rest := errorText[idx+len(retryAfterMarker):]
	end := strings.IndexByte(rest, ']')
	if end < 0 {
		return errorText
	}
	// Drop the marker plus any single space immediately before it.
	prefix := errorText[:idx]
	prefix = strings.TrimRight(prefix, " ")
	suffix := rest[end+1:]
	return strings.TrimSpace(prefix + suffix)
}

// RetryAfterTimestamp converts a Retry-After hint (ms from now) into the same
// RFC3339 cooldown timestamp format used by RateLimitedUntil, for surfacing to
// clients as an absolute time.
func RetryAfterTimestamp(ms int) string {
	if ms <= 0 {
		return ""
	}
	return CooldownUntil(ms)
}
