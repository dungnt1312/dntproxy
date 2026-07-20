package minimax

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dungnt/dntproxy/internal/domain"
)

const (
	ImageModel          = "image-01"
	MaxImagePromptChars = 1500
	MaxImagesPerRequest = 9
)

var supportedAspectRatios = map[string]struct{}{
	"1:1":  {},
	"16:9": {},
	"4:3":  {},
	"3:2":  {},
	"2:3":  {},
	"3:4":  {},
	"9:16": {},
	"21:9": {},
}

// ImageRequest is MiniMax's native synchronous image-generation request.
type ImageRequest struct {
	Model            string             `json:"model"`
	Prompt           string             `json:"prompt"`
	AspectRatio      string             `json:"aspect_ratio,omitempty"`
	Width            int                `json:"width,omitempty"`
	Height           int                `json:"height,omitempty"`
	ResponseFormat   string             `json:"response_format"`
	Seed             *int64             `json:"seed,omitempty"`
	N                int                `json:"n"`
	PromptOptimizer  *bool              `json:"prompt_optimizer,omitempty"`
	SubjectReference []SubjectReference `json:"subject_reference,omitempty"`
}

type SubjectReference struct {
	Type      string `json:"type"`
	ImageFile string `json:"image_file"`
}

type imageResponse struct {
	Data struct {
		ImageURLs   []string `json:"image_urls"`
		ImageBase64 []string `json:"image_base64"`
	} `json:"data"`
	Metadata struct {
		FailedCount json.RawMessage `json:"failed_count"`
	} `json:"metadata"`
	BaseResp *struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// APIError represents an error encoded in MiniMax's base_resp envelope.
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("MiniMax error %d", e.Code)
	}
	return fmt.Sprintf("MiniMax error %d: %s", e.Code, e.Message)
}

// BuildImageRequest converts an OpenAI-compatible image request to MiniMax format.
func BuildImageRequest(rawJSON []byte, model string) ([]byte, error) {
	var req struct {
		Prompt          string `json:"prompt"`
		N               *int   `json:"n"`
		Size            string `json:"size"`
		ResponseFormat  string `json:"response_format"`
		AspectRatio     string `json:"aspect_ratio"`
		Width           int    `json:"width"`
		Height          int    `json:"height"`
		Seed            *int64 `json:"seed"`
		PromptOptimizer *bool  `json:"prompt_optimizer"`
	}
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}

	canonicalModel := canonicalImageModel(model)
	if canonicalModel != ImageModel {
		return nil, fmt.Errorf("unsupported MiniMax image model: %s", model)
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if utf8.RuneCountInString(prompt) > MaxImagePromptChars {
		return nil, fmt.Errorf("prompt exceeds %d characters", MaxImagePromptChars)
	}

	n := 1
	if req.N != nil {
		n = *req.N
	}
	if n < 1 || n > MaxImagesPerRequest {
		return nil, fmt.Errorf("n must be between 1 and %d", MaxImagesPerRequest)
	}

	responseFormat, err := normalizeResponseFormat(req.ResponseFormat)
	if err != nil {
		return nil, err
	}

	native := ImageRequest{
		Model:           canonicalModel,
		Prompt:          prompt,
		ResponseFormat:  responseFormat,
		Seed:            req.Seed,
		N:               n,
		PromptOptimizer: req.PromptOptimizer,
	}

	switch {
	case strings.TrimSpace(req.AspectRatio) != "":
		native.AspectRatio, err = validateAspectRatio(req.AspectRatio)
	case req.Width != 0 || req.Height != 0:
		native.Width, native.Height, err = validateDimensions(req.Width, req.Height)
	default:
		native.AspectRatio, native.Width, native.Height, err = dimensionsFromSize(req.Size)
	}
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(native)
	if err != nil {
		return nil, fmt.Errorf("marshal MiniMax image request: %w", err)
	}
	return body, nil
}

// BuildImageEditRequest converts an OpenAI-compatible JSON image edit request
// to MiniMax image-to-image generation with one character reference.
func BuildImageEditRequest(rawJSON []byte, model string) ([]byte, error) {
	nativeBody, err := BuildImageRequest(rawJSON, model)
	if err != nil {
		return nil, err
	}

	var req struct {
		Image  string            `json:"image"`
		Images []json.RawMessage `json:"images"`
		Mask   json.RawMessage   `json:"mask"`
	}
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}
	if hasJSONValue(req.Mask) {
		return nil, errors.New("mask editing is not supported by MiniMax image-01")
	}

	references := make([]string, 0, 1+len(req.Images))
	if image := strings.TrimSpace(req.Image); image != "" {
		references = append(references, image)
	}
	for index, raw := range req.Images {
		image, err := parseEditImage(raw)
		if err != nil {
			return nil, fmt.Errorf("images[%d]: %w", index, err)
		}
		if image != "" {
			references = append(references, image)
		}
	}
	if len(references) == 0 {
		return nil, errors.New("exactly one reference image is required")
	}
	if len(references) > 1 {
		return nil, errors.New("MiniMax image-01 supports exactly one reference image")
	}
	if err := validateReferenceImage(references[0]); err != nil {
		return nil, err
	}

	var native ImageRequest
	if err := json.Unmarshal(nativeBody, &native); err != nil {
		return nil, fmt.Errorf("decode MiniMax image request: %w", err)
	}
	native.SubjectReference = []SubjectReference{{
		Type:      "character",
		ImageFile: references[0],
	}}

	body, err := json.Marshal(native)
	if err != nil {
		return nil, fmt.Errorf("marshal MiniMax image edit request: %w", err)
	}
	return body, nil
}

// ParseImageResponse converts MiniMax's native response to dntproxy image results.
func ParseImageResponse(body []byte) ([]domain.ImageResult, error) {
	var resp imageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse MiniMax image response: %w", err)
	}
	if resp.BaseResp == nil {
		return nil, errors.New("MiniMax response missing base_resp")
	}
	if resp.BaseResp.StatusCode != 0 {
		return nil, &APIError{Code: resp.BaseResp.StatusCode, Message: resp.BaseResp.StatusMsg}
	}
	failedCount, err := parseCount(resp.Metadata.FailedCount)
	if err != nil {
		return nil, fmt.Errorf("parse MiniMax failed_count: %w", err)
	}
	if failedCount > 0 {
		return nil, fmt.Errorf("MiniMax image batch partially failed: %d image(s) failed", failedCount)
	}

	results := make([]domain.ImageResult, 0, len(resp.Data.ImageURLs)+len(resp.Data.ImageBase64))
	for _, imageURL := range resp.Data.ImageURLs {
		if imageURL = strings.TrimSpace(imageURL); imageURL != "" {
			results = append(results, domain.ImageResult{URL: imageURL})
		}
	}
	for _, imageBase64 := range resp.Data.ImageBase64 {
		if imageBase64 = strings.TrimSpace(imageBase64); imageBase64 != "" {
			results = append(results, domain.ImageResult{B64JSON: imageBase64})
		}
	}
	if len(results) == 0 {
		return nil, errors.New("MiniMax response did not contain image output")
	}
	return results, nil
}

// HTTPStatus maps a MiniMax business error to an OpenAI-compatible HTTP status.
func HTTPStatus(err error) int {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return 0
	}
	switch apiErr.Code {
	case 1001:
		return http.StatusGatewayTimeout
	case 1002:
		return http.StatusTooManyRequests
	case 1004, 2049:
		return http.StatusUnauthorized
	case 1008:
		return http.StatusPaymentRequired
	case 1026, 1027, 2013:
		return http.StatusBadRequest
	default:
		return 0
	}
}

func ResolveImageBaseURL(credentials *domain.Credentials) string {
	if credentials != nil && strings.TrimSpace(credentials.BaseURL) != "" {
		return domain.StripVersionSuffix(strings.TrimRight(strings.TrimSpace(credentials.BaseURL), "/"))
	}
	return "https://api.minimax.io"
}

func parseEditImage(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var image string
	if err := json.Unmarshal(raw, &image); err == nil {
		return strings.TrimSpace(image), nil
	}
	var object struct {
		ImageURL string `json:"image_url"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", errors.New("must be a URL string or an object containing image_url")
	}
	image = strings.TrimSpace(object.ImageURL)
	if image == "" {
		return "", errors.New("image_url is required")
	}
	return image, nil
}

func validateReferenceImage(image string) error {
	lower := strings.ToLower(image)
	if strings.HasPrefix(lower, "data:image/") {
		comma := strings.IndexByte(image, ',')
		if comma < 0 || !strings.HasSuffix(strings.ToLower(image[:comma]), ";base64") {
			return errors.New("reference image data URL must be base64 encoded")
		}
		mediaType := strings.ToLower(strings.TrimSuffix(image[:comma], ";base64"))
		if mediaType != "data:image/png" && mediaType != "data:image/jpeg" && mediaType != "data:image/jpg" {
			return errors.New("MiniMax reference image must be PNG or JPEG")
		}
		if comma == len(image)-1 {
			return errors.New("reference image data URL is empty")
		}
		if _, err := base64.StdEncoding.DecodeString(image[comma+1:]); err != nil {
			return errors.New("reference image data URL contains invalid base64")
		}
		return nil
	}
	parsed, err := url.Parse(image)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("reference image must be an HTTP(S) URL or image data URL")
	}
	return nil
}

func hasJSONValue(raw json.RawMessage) bool {
	value := strings.TrimSpace(string(raw))
	return value != "" && value != "null" && value != `""`
}

func canonicalImageModel(model string) string {
	model = strings.TrimSpace(model)
	if idx := strings.LastIndex(model, "/"); idx >= 0 && idx < len(model)-1 {
		model = model[idx+1:]
	}
	return strings.ToLower(model)
}

func normalizeResponseFormat(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "b64_json", "base64":
		return "base64", nil
	case "url":
		return "url", nil
	default:
		return "", fmt.Errorf("response_format must be url or b64_json")
	}
}

func validateAspectRatio(value string) (string, error) {
	value = strings.TrimSpace(value)
	if _, ok := supportedAspectRatios[value]; !ok {
		return "", fmt.Errorf("unsupported aspect_ratio: %s", value)
	}
	return value, nil
}

func dimensionsFromSize(size string) (aspectRatio string, width, height int, err error) {
	size = strings.ToLower(strings.TrimSpace(size))
	if size == "" || size == "auto" {
		return "1:1", 0, 0, nil
	}
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return "", 0, 0, fmt.Errorf("size must use WIDTHxHEIGHT format")
	}
	width, err = strconv.Atoi(parts[0])
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid image width: %s", parts[0])
	}
	height, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid image height: %s", parts[1])
	}
	width, height, err = validateDimensions(width, height)
	return "", width, height, err
}

func validateDimensions(width, height int) (int, int, error) {
	if width < 512 || width > 2048 || height < 512 || height > 2048 {
		return 0, 0, errors.New("width and height must each be between 512 and 2048")
	}
	if width%8 != 0 || height%8 != 0 {
		return 0, 0, errors.New("width and height must be divisible by 8")
	}
	return width, height, nil
}

func parseCount(raw json.RawMessage) (int, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	value := strings.Trim(string(raw), `"`)
	count, err := strconv.Atoi(value)
	if err != nil || count < 0 {
		return 0, fmt.Errorf("invalid count %q", value)
	}
	return count, nil
}
