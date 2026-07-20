package minimax

import (
	"net/http"
	"time"
)

// ImageResponseHeaderTimeout allows MiniMax enough time to finish generation
// before returning its response. Image generation commonly takes longer than
// the first-byte timeout used by streaming chat requests.
const ImageResponseHeaderTimeout = 180 * time.Second

// ImageHTTPClient is isolated from the shared streaming client so image latency
// does not change timeout behavior for chat and other providers.
var ImageHTTPClient = NewImageHTTPClient()

func NewImageHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:          20,
			MaxIdleConnsPerHost:   5,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: ImageResponseHeaderTimeout,
			ForceAttemptHTTP2:     true,
		},
	}
}
