package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

const maxOpenAIImageResponseBytes = 10 << 20

type ImageProvider struct {
	allowUnknownModels bool
}

func NewImageProvider() *ImageProvider { return &ImageProvider{} }
func NewCompatibleImageProvider() *ImageProvider {
	return &ImageProvider{allowUnknownModels: true}
}

func (p *ImageProvider) Capabilities(model string) domain.ImageCapabilities {
	model = strings.ToLower(strings.TrimSpace(model))
	if !strings.Contains(model, "gpt-image") && !strings.Contains(model, "dall-e") && !p.allowUnknownModels {
		return domain.ImageCapabilities{}
	}
	capabilities := domain.ImageCapabilities{
		Generate:        true,
		Edit:            true,
		Multipart:       true,
		Mask:            true,
		MultiReference:  true,
		Streaming:       true,
		MaxReferences:   16,
		InputFormats:    []string{"image/png", "image/jpeg", "image/webp"},
		ResponseFormats: []string{"url", "b64_json"},
	}
	if strings.Contains(model, "dall-e-3") {
		capabilities.Edit = false
		capabilities.Multipart = false
		capabilities.Mask = false
		capabilities.MultiReference = false
		capabilities.MaxReferences = 0
	}
	return capabilities
}

func (*ImageProvider) Generate(ctx context.Context, req port.ImageRequest) ([]domain.ImageResult, int, error) {
	if req.Form != nil {
		return nil, http.StatusBadRequest, errors.New("image generation accepts JSON only")
	}
	if IsCodexOAuthExport(req.Credentials) {
		body, err := TranslateImageGenerationsToCodex(req.Body, req.Model)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("translate image request: %w", err)
		}
		return executeCodexImageRequest(ctx, body, req.Credentials, req.Logger)
	}
	return executeOpenAICompatibleImage(ctx, req.Body, req.Credentials, req.Logger, false)
}

func (*ImageProvider) Edit(ctx context.Context, req port.ImageRequest) ([]domain.ImageResult, int, error) {
	if req.Form != nil {
		if !IsCodexOAuthExport(req.Credentials) {
			return nil, http.StatusBadRequest, errors.New("multipart image edit requires OAuth/Codex authentication")
		}
		body, err := TranslateMultipartEditToCodex(req.Form, req.Model)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("translate multipart edit request: %w", err)
		}
		return executeCodexImageRequest(ctx, body, req.Credentials, req.Logger)
	}
	if IsCodexOAuthExport(req.Credentials) {
		body, err := TranslateImageEditsToCodex(req.Body, req.Model)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("translate image edit request: %w", err)
		}
		return executeCodexImageRequest(ctx, body, req.Credentials, req.Logger)
	}
	return executeOpenAICompatibleImage(ctx, req.Body, req.Credentials, req.Logger, true)
}

func (*ImageProvider) StreamGenerate(ctx context.Context, req port.ImageRequest) (<-chan domain.ImageStreamEvent, int, error) {
	if !IsCodexOAuthExport(req.Credentials) {
		return nil, http.StatusBadRequest, errors.New("streaming image generation requires OAuth/Codex authentication")
	}
	body, err := TranslateImageGenerationsToCodex(req.Body, req.Model)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("translate image request: %w", err)
	}
	response, status, err := performCodexImageRequest(ctx, body, req.Credentials, req.Logger)
	if err != nil {
		return nil, status, err
	}
	events := make(chan domain.ImageStreamEvent)
	go func() {
		defer close(events)
		defer response.Body.Close()
		for chunk := range ParseCodexImageStream(response.Body) {
			event := domain.ImageStreamEvent{
				Partial: chunk.IsPartial,
				Done:    chunk.IsDone,
				Created: chunk.CreatedAt,
			}
			if chunk.B64JSON != "" || chunk.RevisedPrompt != "" {
				event.Result = &domain.ImageResult{
					B64JSON:       chunk.B64JSON,
					RevisedPrompt: chunk.RevisedPrompt,
				}
			}
			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return events, status, nil
}

func executeOpenAICompatibleImage(ctx context.Context, body []byte, creds *domain.Credentials, reqlog port.RequestLogger, edit bool) ([]domain.ImageResult, int, error) {
	if creds == nil {
		return nil, http.StatusUnauthorized, errors.New("OpenAI credentials are required")
	}
	var request *http.Request
	var err error
	if edit {
		request, err = ForwardImageEdit(body, creds.BaseURL, creds.APIKey)
	} else {
		request, err = ForwardImageGeneration(body, creds.BaseURL, creds.APIKey)
	}
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("create image request: %w", err)
	}
	request = request.WithContext(ctx)
	if reqlog != nil {
		reqlog.SetBodies(shared.PrepareLoggedBody(body), "")
	}
	start := time.Now()
	response, err := shared.StreamingHTTPClient.Do(request)
	duration := time.Since(start)
	if err != nil {
		if reqlog != nil {
			reqlog.Upstream(request.URL.String(), http.MethodPost, http.StatusBadGateway, duration, err)
		}
		return nil, http.StatusBadGateway, fmt.Errorf("image request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxOpenAIImageResponseBytes+1))
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("read image response: %w", err)
	}
	if len(responseBody) > maxOpenAIImageResponseBytes {
		return nil, http.StatusBadGateway, errors.New("image response exceeds size limit")
	}
	if reqlog != nil {
		reqlog.Upstream(request.URL.String(), http.MethodPost, response.StatusCode, duration, nil)
	}
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, fmt.Errorf("upstream returned %d: %s", response.StatusCode, string(responseBody))
	}
	var imageResponse domain.ImageGenerationsResponse
	if err := json.Unmarshal(responseBody, &imageResponse); err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("parse image response: %w", err)
	}
	return imageResponse.Data, response.StatusCode, nil
}

func executeCodexImageRequest(ctx context.Context, body []byte, creds *domain.Credentials, reqlog port.RequestLogger) ([]domain.ImageResult, int, error) {
	response, status, err := performCodexImageRequest(ctx, body, creds, reqlog)
	if err != nil {
		return nil, status, err
	}
	defer response.Body.Close()
	results, _, _, err := ParseCodexImageResponse(response.Body)
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("parse Codex image response: %w", err)
	}
	if len(results) == 0 {
		return nil, http.StatusBadGateway, errors.New("upstream did not return image output")
	}
	return results, status, nil
}

func performCodexImageRequest(ctx context.Context, body []byte, creds *domain.Credentials, reqlog port.RequestLogger) (*http.Response, int, error) {
	if creds == nil || creds.AccessToken == "" {
		return nil, http.StatusUnauthorized, errors.New("Codex OAuth credentials are required")
	}
	endpoint := CodexResponsesURLExport()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, http.StatusInternalServerError, errors.New("create Codex image request")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	request.Header.Set("originator", "codex-cli")
	request.Header.Set("User-Agent", "codex-cli/1.0.18 (macOS; arm64)")
	request.Header.Set("session_id", fmt.Sprintf("%d-%s", time.Now().UnixMilli(), RandomAlphaNumExport(9)))
	if reqlog != nil {
		reqlog.SetBodies(shared.PrepareLoggedBody(body), "")
	}
	start := time.Now()
	response, err := CodexHTTPClientExport().Do(request)
	duration := time.Since(start)
	if err != nil {
		if reqlog != nil {
			reqlog.Upstream(endpoint, http.MethodPost, http.StatusBadGateway, duration, err)
		}
		return nil, http.StatusBadGateway, fmt.Errorf("Codex image request failed: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		upstreamErr := fmt.Errorf("Codex returned %d: %s", response.StatusCode, string(responseBody))
		if reqlog != nil {
			reqlog.Upstream(endpoint, http.MethodPost, response.StatusCode, duration, upstreamErr)
		}
		return nil, response.StatusCode, upstreamErr
	}
	if reqlog != nil {
		reqlog.Upstream(endpoint, http.MethodPost, response.StatusCode, duration, nil)
	}
	return response, response.StatusCode, nil
}

var (
	_ port.ImageProvider          = (*ImageProvider)(nil)
	_ port.StreamingImageProvider = (*ImageProvider)(nil)
)
