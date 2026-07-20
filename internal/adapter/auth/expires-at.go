package auth

import (
	"strings"
	"time"
)

// ParseExpiresAt parses credential expiry timestamps from RFC3339 and other common layouts
// (including values written by CLIProxyAPI auth files).
func ParseExpiresAt(raw string) (time.Time, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts.UTC(), true
		}
	}
	return time.Time{}, false
}

// NormalizeExpiresAtRFC3339 returns a canonical RFC3339 UTC string when parsing succeeds.
func NormalizeExpiresAtRFC3339(raw string) (string, bool) {
	ts, ok := ParseExpiresAt(raw)
	if !ok {
		return "", false
	}
	return ts.Format(time.RFC3339), true
}

// IsAccessTokenExpired reports whether the stored expiry is in the past.
func IsAccessTokenExpired(expiresAt string) bool {
	ts, ok := ParseExpiresAt(expiresAt)
	if !ok {
		return false
	}
	return !ts.After(time.Now().UTC())
}