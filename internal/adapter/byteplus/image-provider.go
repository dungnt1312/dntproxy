package byteplus

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// JSON routes are capped at 10 MiB. A 7 MiB local image expands to roughly
// 9.4 MiB as a base64 data URI, leaving room for JSON/prompt overhead.
const maxReferenceImageBytes int64 = 7 * 1024 * 1024

// ImageProvider implements ModelArk's unified generation/edit endpoint.
type ImageProvider struct {
	client *ImageClient
}

func NewImageProvider() *ImageProvider {
	return &ImageProvider{client: NewImageClient()}
}

func NewImageProviderWithClient(client *ImageClient) *ImageProvider {
	if client == nil {
		client = NewImageClient()
	}
	return &ImageProvider{client: client}
}

func (p *ImageProvider) Capabilities(model string) domain.ImageCapabilities {
	model = strings.ToLower(canonicalModel(model))
	if !strings.Contains(model, "seedream") && !strings.Contains(model, "seededit") {
		return domain.ImageCapabilities{}
	}
	maxReferences := MaxReferenceImages
	edit := true
	switch {
	case strings.Contains(model, "seedream-3-0-t2i"):
		edit = false
		maxReferences = 0
	case strings.Contains(model, "seedream-5-0-pro"):
		maxReferences = 10
	}
	return domain.ImageCapabilities{
		Generate:           true,
		Edit:               edit,
		MultiReference:     edit && maxReferences > 1,
		MaxReferences:      maxReferences,
		MaxInputBytes:      maxReferenceImageBytes,
		MaxTotalInputBytes: maxReferenceImageBytes,
		InputFormats:       []string{"jpeg", "png", "webp", "bmp", "tiff", "gif", "heic", "heif"},
		ResponseFormats:    []string{"url", "b64_json"},
	}
}

func (p *ImageProvider) Generate(ctx context.Context, req port.ImageRequest) ([]domain.ImageResult, int, error) {
	if req.Form != nil {
		return nil, http.StatusBadRequest, errors.New("BytePlus image generation accepts JSON only")
	}
	body, err := BuildImageRequest(req.Body, req.Model)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	return p.imageClient().Execute(ctx, body, req.Credentials, req.Logger)
}

func (p *ImageProvider) Edit(ctx context.Context, req port.ImageRequest) ([]domain.ImageResult, int, error) {
	if !p.Capabilities(req.Model).Edit {
		return nil, http.StatusBadRequest, errors.New("selected BytePlus model does not support image editing")
	}
	if req.Form != nil {
		return nil, http.StatusBadRequest, errors.New("BytePlus image editing accepts JSON image URLs or data URLs only")
	}
	body, err := BuildImageEditRequest(req.Body, req.Model)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	return p.imageClient().Execute(ctx, body, req.Credentials, req.Logger)
}

func (p *ImageProvider) imageClient() *ImageClient {
	if p != nil && p.client != nil {
		return p.client
	}
	return NewImageClient()
}

var _ port.ImageProvider = (*ImageProvider)(nil)
