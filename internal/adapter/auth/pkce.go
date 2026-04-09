package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateCodeVerifier creates a PKCE code verifier (43 chars, base64url).
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge creates a PKCE S256 code challenge from a verifier.
func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GenerateState creates a random state string for CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GeneratePKCE generates a complete PKCE pair (verifier, challenge, state).
func GeneratePKCE() (codeVerifier, codeChallenge, state string, err error) {
	codeVerifier, err = GenerateCodeVerifier()
	if err != nil {
		return
	}
	codeChallenge = GenerateCodeChallenge(codeVerifier)
	state, err = GenerateState()
	return
}
