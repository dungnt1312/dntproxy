package port

import (
	"context"
	"io"

	"github.com/dungnt/dntproxy/internal/domain"
)

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

// RequestMetadata carries optional per-request observability data.
type RequestMetadata struct {
	Compression *domain.CompressionLogMetadata
	TenantID    string // for multi-tenancy (SaaS). Empty = legacy single-tenant.
	Context     context.Context
	SessionKey  string // raw client session header value (optional)
	APIKeyID    string // authenticated API key ID, if any
}

// ChatService defines the chat orchestration contract.
type ChatService interface {
	// HandleChat processes a chat completion request.
	// policy may be nil if no API key restrictions apply.
	HandleChat(body []byte, modelStr string, requestID string, policy *APIKeyPolicy, metadata ...RequestMetadata) *ChatResult
}
