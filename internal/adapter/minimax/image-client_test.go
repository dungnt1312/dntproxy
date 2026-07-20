package minimax

import (
	"net/http"
	"testing"
	"time"
)

func TestNewImageHTTPClientUsesImageSpecificHeaderTimeout(t *testing.T) {
	client := NewImageHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.ResponseHeaderTimeout != ImageResponseHeaderTimeout {
		t.Fatalf("ResponseHeaderTimeout = %v, want %v", transport.ResponseHeaderTimeout, ImageResponseHeaderTimeout)
	}
	if transport.ResponseHeaderTimeout < 2*time.Minute {
		t.Fatalf("ResponseHeaderTimeout = %v, want at least 2m", transport.ResponseHeaderTimeout)
	}
	if client.Timeout != 0 {
		t.Fatalf("client Timeout = %v, want no overall timeout", client.Timeout)
	}
}
