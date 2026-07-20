package minimax

import (
	"bytes"
	"context"
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

const maxImageResponseSize = 64 << 20

type ImageProvider struct{}

func NewImageProvider() *ImageProvider { return &ImageProvider{} }

func (*ImageProvider) Capabilities(model string) domain.ImageCapabilities {
	if !strings.Contains(strings.ToLower(model), "image") {
		return domain.ImageCapabilities{}
	}
	return domain.ImageCapabilities{
		Generate:           true,
		Edit:               true,
		MaxReferences:      1,
		MaxInputBytes:      7 << 20,
		MaxTotalInputBytes: 7 << 20,
		InputFormats:       []string{"image/png", "image/jpeg"},
		ResponseFormats:    []string{"url", "b64_json"},
	}
}

func (*ImageProvider) Generate(ctx context.Context, req port.ImageRequest) ([]domain.ImageResult, int, error) {
	if req.Form != nil {
		return nil, http.StatusBadRequest, errors.New("MiniMax image generation accepts JSON only")
	}
	body, err := BuildImageRequest(req.Body, req.Model)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("build MiniMax image request: %w", err)
	}
	return executeNativeImageRequest(ctx, body, req.Credentials, req.Logger)
}

func (*ImageProvider) Edit(ctx context.Context, req port.ImageRequest) ([]domain.ImageResult, int, error) {
	if req.Form != nil {
		return nil, http.StatusBadRequest, errors.New("MiniMax image editing accepts JSON only")
	}
	body, err := BuildImageEditRequest(req.Body, req.Model)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("build MiniMax image edit request: %w", err)
	}
	return executeNativeImageRequest(ctx, body, req.Credentials, req.Logger)
}

func executeNativeImageRequest(ctx context.Context, body []byte, creds *domain.Credentials, reqlog port.RequestLogger) ([]domain.ImageResult, int, error) {
	if creds == nil {
		return nil, http.StatusUnauthorized, errors.New("MiniMax credentials are required")
	}
	endpoint := ResolveImageBaseURL(creds) + "/v1/image_generation"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("create MiniMax image request: %w", err)
	}
	apiKey := creds.APIKey
	if apiKey == "" {
		apiKey = creds.AccessToken
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	if reqlog != nil {
		reqlog.SetBodies(shared.PrepareLoggedBody(body), "")
	}
	start := time.Now()
	response, err := ImageHTTPClient.Do(request)
	duration := time.Since(start)
	if err != nil {
		if reqlog != nil {
			reqlog.Upstream(endpoint, http.MethodPost, http.StatusBadGateway, duration, err)
		}
		return nil, http.StatusBadGateway, fmt.Errorf("MiniMax image request failed: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxImageResponseSize+1))
	if err != nil {
		if reqlog != nil {
			reqlog.Upstream(endpoint, http.MethodPost, http.StatusBadGateway, duration, err)
		}
		return nil, http.StatusBadGateway, fmt.Errorf("read MiniMax image response: %w", err)
	}
	if len(responseBody) > maxImageResponseSize {
		err = fmt.Errorf("MiniMax image response exceeds %d bytes", maxImageResponseSize)
		if reqlog != nil {
			reqlog.Upstream(endpoint, http.MethodPost, http.StatusBadGateway, duration, err)
		}
		return nil, http.StatusBadGateway, err
	}

	results, parseErr := ParseImageResponse(responseBody)
	if parseErr != nil {
		status := HTTPStatus(parseErr)
		if status == 0 {
			status = response.StatusCode
			if status < 400 {
				status = http.StatusBadGateway
			}
		}
		if reqlog != nil {
			reqlog.Upstream(endpoint, http.MethodPost, response.StatusCode, duration, parseErr)
		}
		return nil, status, parseErr
	}
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("MiniMax returned %d", response.StatusCode)
		if reqlog != nil {
			reqlog.Upstream(endpoint, http.MethodPost, response.StatusCode, duration, err)
		}
		return nil, response.StatusCode, err
	}
	if reqlog != nil {
		reqlog.Upstream(endpoint, http.MethodPost, response.StatusCode, duration, nil)
	}
	return results, response.StatusCode, nil
}

var _ port.ImageProvider = (*ImageProvider)(nil)
