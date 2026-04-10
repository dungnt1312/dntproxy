package port

import "io"

// ChatResult is the result of a chat completion request.
type ChatResult struct {
	// Stream is the SSE stream reader (nil on error).
	Stream io.ReadCloser
	// StatusCode is the HTTP status code.
	StatusCode int
	// Error message (empty on success).
	Error string
}

// ChatService defines the chat orchestration contract.
type ChatService interface {
	// HandleChat processes a chat completion request.
	HandleChat(body []byte, modelStr string, requestID string) *ChatResult
}
