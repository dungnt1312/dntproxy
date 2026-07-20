package port

import (
	"context"
	"mime/multipart"

	"github.com/dungnt/dntproxy/internal/domain"
)

// ImageRequest is the provider-neutral execution envelope. Authentication,
// tenancy, policy, pinning, and account selection are resolved before this
// value reaches an adapter.
type ImageRequest struct {
	Model       string
	Body        []byte
	Form        *multipart.Form
	Credentials *domain.Credentials
	Logger      RequestLogger
}

// ImageProvider executes OpenAI-compatible image operations for one provider.
type ImageProvider interface {
	Capabilities(model string) domain.ImageCapabilities
	Generate(ctx context.Context, req ImageRequest) ([]domain.ImageResult, int, error)
	Edit(ctx context.Context, req ImageRequest) ([]domain.ImageResult, int, error)
}

// StreamingImageProvider is implemented by providers that can expose partial
// image output. The returned status is the upstream HTTP status.
type StreamingImageProvider interface {
	StreamGenerate(ctx context.Context, req ImageRequest) (<-chan domain.ImageStreamEvent, int, error)
}

// ImageProviderRegistry stores image providers independently of chat
// executors, allowing image-only providers such as BytePlus.
type ImageProviderRegistry interface {
	GetImageProvider(provider string) ImageProvider
	RegisterImageProvider(provider string, imageProvider ImageProvider)
	SupportedImageProviders() []string
}
