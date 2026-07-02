package openai

import (
	"crypto/rand"
	"time"
)

func currentTimeUnix() int64 {
	return time.Now().Unix()
}

const alphaNumChars = "abcdefghijklmnopqrstuvwxyz0123456789"

// randomAlphaNum generates a random alphanumeric string of the given length.
func randomAlphaNum(n int) string {
	b := make([]byte, n)
	randomBytes := make([]byte, n)
	if _, err := rand.Read(randomBytes); err != nil {
		for i := range b {
			b[i] = alphaNumChars[i%len(alphaNumChars)]
		}
		return string(b)
	}
	for i := range randomBytes {
		b[i] = alphaNumChars[int(randomBytes[i])%len(alphaNumChars)]
	}
	return string(b)
}

// RandomAlphaNumExport generates a random alphanumeric string (exported for image handler).
func RandomAlphaNumExport(n int) string {
	return randomAlphaNum(n)
}
