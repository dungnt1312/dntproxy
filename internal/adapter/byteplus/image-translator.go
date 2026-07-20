package byteplus

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	MaxReferenceImages = 14
)

// ImageRequest is the native, non-streaming ModelArk image request.
// The same endpoint handles text-to-image and image-to-image requests.
type ImageRequest struct {
	Model                            string                            `json:"model"`
	Prompt                           string                            `json:"prompt"`
	Image                            any                               `json:"image,omitempty"`
	Size                             string                            `json:"size,omitempty"`
	Seed                             *int64                            `json:"seed,omitempty"`
	ResponseFormat                   string                            `json:"response_format,omitempty"`
	OutputFormat                     string                            `json:"output_format,omitempty"`
	Watermark                        *bool                             `json:"watermark,omitempty"`
	SequentialImageGeneration        string                            `json:"sequential_image_generation,omitempty"`
	SequentialImageGenerationOptions *SequentialImageGenerationOptions `json:"sequential_image_generation_options,omitempty"`
	Stream                           bool                              `json:"stream,omitempty"`
}

type SequentialImageGenerationOptions struct {
	MaxImages int `json:"max_images,omitempty"`
}

type compatibleRequest struct {
	Prompt                           string                            `json:"prompt"`
	Image                            json.RawMessage                   `json:"image"`
	Images                           []json.RawMessage                 `json:"images"`
	Mask                             json.RawMessage                   `json:"mask"`
	N                                *int                              `json:"n"`
	Size                             string                            `json:"size"`
	Quality                          string                            `json:"quality"`
	Style                            string                            `json:"style"`
	ResponseFormat                   string                            `json:"response_format"`
	Seed                             *int64                            `json:"seed"`
	OutputFormat                     string                            `json:"output_format"`
	Watermark                        *bool                             `json:"watermark"`
	SequentialImageGeneration        string                            `json:"sequential_image_generation"`
	SequentialImageGenerationOptions *SequentialImageGenerationOptions `json:"sequential_image_generation_options"`
	Stream                           bool                              `json:"stream"`
}

// BuildImageRequest converts an OpenAI-compatible generation request to
// BytePlus ModelArk's image generation schema.
func BuildImageRequest(rawJSON []byte, model string) ([]byte, error) {
	var req compatibleRequest
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}
	if hasJSONValue(req.Image) || len(req.Images) > 0 || hasJSONValue(req.Mask) {
		return nil, errors.New("reference images belong on the image edits endpoint")
	}
	return buildNativeRequest(req, model, nil)
}

// BuildImageEditRequest converts a JSON image edit request to ModelArk's
// unified image generation schema. ModelArk performs edits by adding image.
func BuildImageEditRequest(rawJSON []byte, model string) ([]byte, error) {
	var req compatibleRequest
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}
	if hasJSONValue(req.Mask) {
		return nil, errors.New("mask editing is not supported by BytePlus ModelArk")
	}

	images, err := parseImageField(req.Image)
	if err != nil {
		return nil, err
	}
	for index, raw := range req.Images {
		image, parseErr := parseCompatibleImageSource(raw)
		if parseErr != nil {
			return nil, fmt.Errorf("images[%d]: %w", index, parseErr)
		}
		images = append(images, image)
	}
	if len(images) == 0 {
		return nil, errors.New("at least one reference image is required")
	}
	if len(images) > MaxReferenceImages {
		return nil, fmt.Errorf("BytePlus supports at most %d reference images", MaxReferenceImages)
	}
	for index, image := range images {
		if err := validateReferenceImage(image); err != nil {
			return nil, fmt.Errorf("reference image %d: %w", index+1, err)
		}
	}
	return buildNativeRequest(req, model, images)
}

func parseCompatibleImageSource(raw json.RawMessage) (string, error) {
	var source string
	if err := json.Unmarshal(raw, &source); err == nil {
		source = strings.TrimSpace(source)
		if source == "" {
			return "", errors.New("image source is required")
		}
		return source, nil
	}
	var object struct {
		ImageURL string `json:"image_url"`
		URL      string `json:"url"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", errors.New("must be a URL/data URI string or an object containing image_url")
	}
	source = strings.TrimSpace(object.ImageURL)
	if source == "" {
		source = strings.TrimSpace(object.URL)
	}
	if source == "" {
		return "", errors.New("image_url is required")
	}
	return source, nil
}

func parseImageField(raw json.RawMessage) ([]string, error) {
	if !hasJSONValue(raw) {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if single = strings.TrimSpace(single); single == "" {
			return nil, nil
		}
		return []string{single}, nil
	}
	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err != nil {
		return nil, errors.New("image must be a string or array of strings")
	}
	return multiple, nil
}

func buildNativeRequest(req compatibleRequest, model string, images []string) ([]byte, error) {
	model = canonicalModel(model)
	if model == "" {
		return nil, errors.New("model is required")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if req.N != nil && *req.N != 1 {
		return nil, errors.New("BytePlus uses sequential_image_generation for batch output; n must be 1")
	}
	if quality := strings.TrimSpace(req.Quality); quality != "" && quality != "standard" {
		return nil, fmt.Errorf("unsupported quality: %s", req.Quality)
	}
	if style := strings.TrimSpace(req.Style); style != "" {
		return nil, fmt.Errorf("style is not supported by BytePlus: %s", req.Style)
	}

	responseFormat := strings.TrimSpace(req.ResponseFormat)
	if responseFormat == "" {
		responseFormat = "url"
	}
	if responseFormat != "url" && responseFormat != "b64_json" {
		return nil, fmt.Errorf("response_format must be url or b64_json")
	}
	outputFormat := strings.ToLower(strings.TrimSpace(req.OutputFormat))
	if outputFormat != "" && outputFormat != "png" && outputFormat != "jpeg" {
		return nil, fmt.Errorf("output_format must be png or jpeg")
	}
	sequential := strings.TrimSpace(req.SequentialImageGeneration)
	if sequential != "" && sequential != "auto" && sequential != "disabled" {
		return nil, errors.New("sequential_image_generation must be auto or disabled")
	}
	if options := req.SequentialImageGenerationOptions; options != nil {
		if sequential != "auto" {
			return nil, errors.New("sequential_image_generation_options requires sequential_image_generation=auto")
		}
		if options.MaxImages < 1 || options.MaxImages > 15 {
			return nil, errors.New("sequential_image_generation_options.max_images must be between 1 and 15")
		}
		if len(images)+options.MaxImages > 15 {
			return nil, errors.New("reference image count plus max_images must not exceed 15")
		}
	}
	if req.Stream {
		return nil, errors.New("streaming BytePlus image output is not supported by this endpoint")
	}

	native := ImageRequest{
		Model:                            model,
		Prompt:                           prompt,
		Size:                             strings.TrimSpace(req.Size),
		Seed:                             req.Seed,
		ResponseFormat:                   responseFormat,
		OutputFormat:                     outputFormat,
		Watermark:                        req.Watermark,
		SequentialImageGeneration:        sequential,
		SequentialImageGenerationOptions: req.SequentialImageGenerationOptions,
	}
	switch len(images) {
	case 0:
	case 1:
		native.Image = images[0]
	default:
		native.Image = images
	}
	body, err := json.Marshal(native)
	if err != nil {
		return nil, fmt.Errorf("marshal BytePlus image request: %w", err)
	}
	return body, nil
}

func canonicalModel(model string) string {
	model = strings.TrimSpace(model)
	for _, prefix := range []string{"byteplus/", "bp/"} {
		model = strings.TrimPrefix(model, prefix)
	}
	return model
}

func validateReferenceImage(image string) error {
	if strings.HasPrefix(image, "data:image/") {
		comma := strings.IndexByte(image, ',')
		if comma < 0 || !strings.HasSuffix(image[:comma], ";base64") {
			return errors.New("data URL must use data:image/<format>;base64,<data>")
		}
		format := strings.TrimSuffix(strings.TrimPrefix(image[:comma], "data:image/"), ";base64")
		switch format {
		case "jpeg", "png", "webp", "bmp", "tiff", "gif", "heic", "heif":
		default:
			return fmt.Errorf("unsupported image format: %s", format)
		}
		if _, err := base64.StdEncoding.DecodeString(image[comma+1:]); err != nil {
			return errors.New("data URL contains invalid base64")
		}
		return nil
	}
	parsed, err := url.ParseRequestURI(image)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("image must be an accessible http(s) URL or base64 data URL")
	}
	return nil
}

func hasJSONValue(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null" && trimmed != `""`
}
