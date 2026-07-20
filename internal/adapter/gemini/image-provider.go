package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

const (
	defaultBaseURL       = "https://generativelanguage.googleapis.com"
	maxImageResponseBody = 64 << 20
)

var sensitiveURLPattern = regexp.MustCompile(`(?i)(?:https?://|data:image/)[^\s"'<>]+`)

// ImageLoader resolves an already-validated HTTP(S) URL or data URI into an
// inline image. Production callers must inject the shared SSRF-safe loader.
type ImageLoader func(ctx context.Context, source string) (data []byte, mimeType string, err error)

// ImageProvider implements Gemini's native image generateContent API.
type ImageProvider struct {
	client *http.Client
	load   ImageLoader
}

var _ port.ImageProvider = (*ImageProvider)(nil)

func NewImageProvider(loader ImageLoader) *ImageProvider {
	return NewImageProviderWithClient(loader, &http.Client{Timeout: 180 * time.Second})
}

func NewImageProviderWithClient(loader ImageLoader, client *http.Client) *ImageProvider {
	if client == nil {
		client = &http.Client{Timeout: 180 * time.Second}
	}
	return &ImageProvider{client: client, load: loader}
}

func (p *ImageProvider) Capabilities(model string) domain.ImageCapabilities {
	if !strings.Contains(canonicalModel(model), "image") {
		return domain.ImageCapabilities{}
	}
	maxReferences := 14
	if strings.Contains(canonicalModel(model), "2.5-flash-image") {
		maxReferences = 3
	}
	return domain.ImageCapabilities{
		Generate:           true,
		Edit:               true,
		MultiReference:     true,
		MaxReferences:      maxReferences,
		MaxInputBytes:      7 << 20,
		MaxTotalInputBytes: 7 << 20,
		InputFormats:       []string{"image/png", "image/jpeg", "image/webp"},
		ResponseFormats:    []string{"b64_json"},
	}
}

func (p *ImageProvider) Generate(ctx context.Context, req port.ImageRequest) ([]domain.ImageResult, int, error) {
	if req.Form != nil {
		return nil, http.StatusBadRequest, errors.New("Gemini image generation requires a JSON request")
	}
	body, err := BuildGenerateContentRequest(req.Body, req.Model, nil)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("build Gemini image request: %w", err)
	}
	return p.execute(ctx, req, body)
}

func (p *ImageProvider) Edit(ctx context.Context, req port.ImageRequest) ([]domain.ImageResult, int, error) {
	if req.Form != nil {
		return nil, http.StatusBadRequest, errors.New("Gemini image editing requires a JSON request")
	}
	if p.load == nil {
		return nil, http.StatusInternalServerError, errors.New("Gemini image input loader is not configured")
	}

	input, err := ParseEditInput(req.Body)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("build Gemini image edit request: %w", err)
	}
	capabilities := p.Capabilities(req.Model)
	if len(input.Sources) > capabilities.MaxReferences {
		return nil, http.StatusBadRequest, fmt.Errorf("Gemini model supports at most %d reference images", capabilities.MaxReferences)
	}

	images := make([]InlineImage, 0, len(input.Sources))
	var totalInputBytes int64
	for i, source := range input.Sources {
		data, mimeType, loadErr := p.load(ctx, source)
		if loadErr != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("load Gemini reference image %d: %w", i+1, sanitizeError(loadErr))
		}
		if !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
			return nil, http.StatusBadRequest, fmt.Errorf("load Gemini reference image %d: unsupported media type", i+1)
		}
		if int64(len(data)) > capabilities.MaxInputBytes {
			return nil, http.StatusBadRequest, fmt.Errorf("Gemini reference image %d exceeds the input size limit", i+1)
		}
		totalInputBytes += int64(len(data))
		if totalInputBytes > capabilities.MaxTotalInputBytes {
			return nil, http.StatusBadRequest, errors.New("Gemini reference images exceed the aggregate input size limit")
		}
		images = append(images, InlineImage{Data: data, MIMEType: mimeType})
	}

	body, err := BuildGenerateContentRequest(req.Body, req.Model, images)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("build Gemini image edit request: %w", err)
	}
	return p.execute(ctx, req, body)
}

func (p *ImageProvider) execute(ctx context.Context, imageReq port.ImageRequest, body []byte) ([]domain.ImageResult, int, error) {
	if imageReq.Credentials == nil || strings.TrimSpace(imageReq.Credentials.APIKey) == "" {
		return nil, http.StatusUnauthorized, errors.New("Gemini API key is required")
	}

	endpoint := imageEndpoint(imageReq.Credentials.BaseURL, imageReq.Model)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, http.StatusInternalServerError, errors.New("create Gemini image request")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("x-goog-api-key", imageReq.Credentials.APIKey)

	if imageReq.Logger != nil {
		imageReq.Logger.SetBodies(prepareLoggedGeminiBody(body), "")
	}
	start := time.Now()
	response, err := p.client.Do(request)
	duration := time.Since(start)
	if err != nil {
		if imageReq.Logger != nil {
			imageReq.Logger.Upstream(endpoint, http.MethodPost, http.StatusBadGateway, duration, err)
		}
		return nil, http.StatusBadGateway, fmt.Errorf("Gemini image request failed: %w", sanitizeError(err))
	}
	defer response.Body.Close()

	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxImageResponseBody+1))
	if readErr != nil {
		if imageReq.Logger != nil {
			imageReq.Logger.Upstream(endpoint, http.MethodPost, response.StatusCode, duration, readErr)
		}
		return nil, http.StatusBadGateway, errors.New("read Gemini image response")
	}
	if len(responseBody) > maxImageResponseBody {
		if imageReq.Logger != nil {
			imageReq.Logger.Upstream(endpoint, http.MethodPost, response.StatusCode, duration, errors.New("response too large"))
		}
		return nil, http.StatusBadGateway, errors.New("Gemini image response exceeds size limit")
	}

	if imageReq.Logger != nil {
		imageReq.Logger.SetBodies(prepareLoggedGeminiBody(body), prepareLoggedGeminiBody(responseBody))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		upstreamErr := ParseAPIError(responseBody, response.StatusCode)
		if imageReq.Logger != nil {
			imageReq.Logger.Upstream(endpoint, http.MethodPost, response.StatusCode, duration, upstreamErr)
		}
		return nil, response.StatusCode, upstreamErr
	}

	results, parseErr := ParseGenerateContentResponse(responseBody)
	if imageReq.Logger != nil {
		imageReq.Logger.Upstream(endpoint, http.MethodPost, response.StatusCode, duration, parseErr)
	}
	if parseErr != nil {
		return nil, http.StatusBadGateway, parseErr
	}
	return results, response.StatusCode, nil
}

func imageEndpoint(baseURL, model string) string {
	base := normalizeBaseURL(baseURL)
	return base + "/v1/models/" + url.PathEscape(canonicalModel(model)) + ":generateContent"
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return defaultBaseURL
	}
	for _, suffix := range []string{
		"/v1beta/openai/chat/completions",
		"/v1beta/openai",
		"/v1beta",
		"/v1",
	} {
		if strings.HasSuffix(baseURL, suffix) {
			return strings.TrimSuffix(baseURL, suffix)
		}
	}
	return baseURL
}

func canonicalModel(model string) string {
	model = strings.TrimSpace(model)
	model = strings.TrimPrefix(model, "models/")
	if index := strings.LastIndex(model, "/"); index >= 0 {
		model = model[index+1:]
	}
	return model
}

func ParseAPIError(body []byte, status int) error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		message := strings.TrimSpace(envelope.Error.Message)
		if message != "" {
			message = sensitiveURLPattern.ReplaceAllString(message, "[redacted-url]")
			if len(message) > 1000 {
				message = message[:1000]
			}
			return fmt.Errorf("Gemini image API returned %d: %s", status, message)
		}
	}
	return fmt.Errorf("Gemini image API returned HTTP %d", status)
}

func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	message := sensitiveURLPattern.ReplaceAllString(err.Error(), "[redacted-url]")
	if len(message) > 1000 {
		message = message[:1000]
	}
	return errors.New(message)
}

func prepareLoggedGeminiBody(body []byte) string {
	var value interface{}
	if json.Unmarshal(body, &value) != nil {
		return shared.PrepareLoggedBody(body)
	}
	redactInlineImageData(value)
	redacted, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return shared.PrepareLoggedBody(redacted)
}

func redactInlineImageData(value interface{}) {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			redactInlineImageData(item)
		}
	case map[string]interface{}:
		isInlineImage := false
		for _, key := range []string{"mimeType", "mime_type"} {
			if mimeType, ok := typed[key].(string); ok && strings.HasPrefix(strings.ToLower(mimeType), "image/") {
				isInlineImage = true
				break
			}
		}
		if isInlineImage {
			if _, exists := typed["data"]; exists {
				typed["data"] = "***REDACTED***"
			}
		}
		for _, item := range typed {
			redactInlineImageData(item)
		}
	}
}
