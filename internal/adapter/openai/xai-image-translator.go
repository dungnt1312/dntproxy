package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
)

const (
	xAIImagesModel        = "grok-imagine-image"
	xAIImagesQualityModel = "grok-imagine-image-quality"
)

// IsXAIImagesModel checks if the model is an xAI/Grok image generation model.
func IsXAIImagesModel(model string) bool {
	base := strings.ToLower(strings.TrimSpace(model))
	// Check for "xai/" or "grok/" prefix
	if idx := strings.LastIndex(base, "/"); idx >= 0 && idx < len(base)-1 {
		base = strings.TrimSpace(base[idx+1:])
	}
	return base == xAIImagesModel || base == xAIImagesQualityModel
}

// xAIImageRequest is the xAI-native image generation request format.
type xAIImageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
}

// BuildXAIImageRequest converts an OpenAI image generation request to xAI format.
func BuildXAIImageRequest(rawJSON []byte, model string) ([]byte, string, error) {
	var req struct {
		Model          string `json:"model"`
		Prompt         string `json:"prompt"`
		N              int    `json:"n"`
		Size           string `json:"size"`
		Quality        string `json:"quality"`
		ResponseFormat string `json:"response_format"`
	}
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return nil, "", fmt.Errorf("invalid request JSON: %w", err)
	}

	responseFormat := normalizeImageResponseFormat(req.ResponseFormat)

	// Map size to aspect_ratio
	aspectRatio := "1:1"
	switch strings.ToLower(strings.TrimSpace(req.Size)) {
	case "1792x1024":
		aspectRatio = "16:9"
	case "1024x1792":
		aspectRatio = "9:16"
	}

	// Map quality to resolution
	resolution := "1k"
	if strings.Contains(strings.ToLower(strings.TrimSpace(req.Size)), "2048") ||
		strings.ToLower(strings.TrimSpace(req.Quality)) == "hd" {
		resolution = "2k"
	}

	n := req.N
	if n <= 0 {
		n = 1
	}

	xaiReq := xAIImageRequest{
		Model:          canonicalXAIModel(model),
		Prompt:         strings.TrimSpace(req.Prompt),
		N:              n,
		ResponseFormat: responseFormat,
		AspectRatio:    aspectRatio,
		Resolution:     resolution,
	}

	body, err := json.Marshal(xaiReq)
	return body, responseFormat, err
}

func canonicalXAIModel(model string) string {
	base := strings.ToLower(strings.TrimSpace(model))
	if idx := strings.LastIndex(base, "/"); idx >= 0 && idx < len(base)-1 {
		base = strings.TrimSpace(base[idx+1:])
	}
	if base == xAIImagesQualityModel {
		return xAIImagesQualityModel
	}
	return xAIImagesModel
}

// CanonicalXAIModelExport returns the canonical xAI image model name.
func CanonicalXAIModelExport(model string) string {
	return canonicalXAIModel(model)
}

// ResolveXAIImageBaseURL returns the base URL for xAI image requests.
func ResolveXAIImageBaseURL(credentials *domain.Credentials) string {
	if credentials.BaseURL != "" {
		return domain.StripVersionSuffix(credentials.BaseURL)
	}
	return "https://api.x.ai"
}
