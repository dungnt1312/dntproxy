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

// APIKeyPolicy holds restrictions from the API key for a single request.
type APIKeyPolicy struct {
	AllowedConnectionIDs []string // nil/empty = unrestricted
	AllowedModels        []string // nil/empty = unrestricted
}

// ChatService defines the chat orchestration contract.
type ChatService interface {
	// HandleChat processes a chat completion request.
	// policy may be nil if no API key restrictions apply.
	HandleChat(body []byte, modelStr string, requestID string, policy *APIKeyPolicy) *ChatResult
}
