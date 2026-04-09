package port

import (
	"io"

	"github.com/dungnt/dntproxy/internal/domain"
)

// ProviderExecutor handles making requests to a specific AI provider.
type ProviderExecutor interface {
	// Execute sends a translated request to the provider and returns a streaming reader.
	// The returned io.ReadCloser streams SSE-formatted data (OpenAI compatible).
	Execute(model string, body []byte, credentials *domain.Credentials) (io.ReadCloser, int, error)
}
