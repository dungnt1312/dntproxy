package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// TOTP (RFC 6238) support for automated 2FA during OAuth enrollment.
// Secrets are the base32 keys shown by authenticator apps / stored by sellers.

const totpStep = 30

// NormalizeTOTPSecret cleans a base32 TOTP secret (strips spaces/dashes,
// uppercases) and restores missing padding so it always decodes.
func NormalizeTOTPSecret(secret string) string {
	s := strings.ToUpper(strings.TrimRight(strings.TrimSpace(secret), "="))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	if s == "" {
		return ""
	}
	if pad := len(s) % 8; pad != 0 {
		s += strings.Repeat("=", 8-pad)
	}
	return s
}

// GenerateTOTP produces the current 6-digit TOTP code for the base32 secret
// at time t (30-second step, HMAC-SHA1).
func GenerateTOTP(secret string, t time.Time) (string, error) {
	normalized := NormalizeTOTPSecret(secret)
	if normalized == "" {
		return "", fmt.Errorf("empty TOTP secret")
	}
	key, err := base32.StdEncoding.DecodeString(normalized)
	if err != nil {
		return "", fmt.Errorf("invalid base32 TOTP secret: %w", err)
	}
	return hotpSHA1(key, uint64(t.Unix())/totpStep), nil
}

// hotpSHA1 is RFC 4226 HMAC-SHA1 HOTP truncated to 6 digits.
func hotpSHA1(key []byte, counter uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", code%1000000)
}
