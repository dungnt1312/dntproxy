package openai

import (
	"math/rand"
	"time"
)

func currentTimeUnix() int64 {
	return time.Now().Unix()
}

const alphaNumChars = "abcdefghijklmnopqrstuvwxyz0123456789"

// randomAlphaNum generates a random alphanumeric string of the given length.
func randomAlphaNum(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphaNumChars[rand.Intn(len(alphaNumChars))]
	}
	return string(b)
}
