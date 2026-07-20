package xai

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

	"github.com/dungnt/dntproxy/internal/adapter/openai"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

const maxImageResponseBytes = 10 << 20

type ImageProvider struct{}

func NewImageProvider() *ImageProvider { return &ImageProvider{} }

func (*ImageProvider) Capabilities(model string) domain.ImageCapabilities {
	if !openai.IsXAIImagesModel(model) {
		return domain.ImageCapabilities{}
	}
	return domain.ImageCapabilities{
		Generate:        true,
		Edit:            true,
		Mask:            true,
		MultiReference:  true,
		MaxReferences:   10,
		InputFormats:    []string{"image/png", "image/jpeg", "image/webp"},
		ResponseFormats: []string{"url", "b64_json"},
	}
}

func (*ImageProvider) Generate(ctx context.Context, req port.ImageRequest) ([]domain.ImageResult, int, error) {
	if req.Form != nil {
		return nil, http.StatusBadRequest, errors.New("xAI image generation accepts JSON only")
	}
	body, _, err := openai.BuildXAIImageRequest(req.Body, req.Model)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("build xAI request: %w", err)
	}
	return executeXAIImage(ctx, "/v1/images/generations", body, req.Credentials, req.Logger, "generation")
}

func (*ImageProvider) Edit(ctx context.Context, req port.ImageRequest) ([]domain.ImageResult, int, error) {
	if req.Form != nil {
		return nil, http.StatusBadRequest, errors.New("multipart image edit not supported for xAI models, use JSON format")
	}
	var editReq struct {
		Prompt string `json:"prompt"`
		Images []struct {
			ImageURL string `json:"image_url"`
		} `json:"images"`
		Image          string `json:"image"`
		Mask           string `json:"mask"`
		N              int    `json:"n"`
		Size           string `json:"size"`
		ResponseFormat string `json:"response_format"`
	}
	if err := json.Unmarshal(req.Body, &editReq); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid edit request: %w", err)
	}
	body := map[string]interface{}{
		"model":           openai.CanonicalXAIModelExport(req.Model),
		"prompt":          editReq.Prompt,
		"n":               editReq.N,
		"response_format": editReq.ResponseFormat,
	}
	if editReq.Size != "" {
		aspectRatio := "1:1"
		switch editReq.Size {
		case "1792x1024":
			aspectRatio = "16:9"
		case "1024x1792":
			aspectRatio = "9:16"
		}
		body["aspect_ratio"] = aspectRatio
	}
	var imageURLs []map[string]string
	for _, image := range editReq.Images {
		if image.ImageURL != "" {
			imageURLs = append(imageURLs, map[string]string{"image_url": image.ImageURL})
		}
	}
	if len(imageURLs) > 0 {
		body["images"] = imageURLs
	} else if editReq.Image != "" {
		body["images"] = []map[string]string{{"image_url": editReq.Image}}
	}
	if editReq.Mask != "" {
		body["mask"] = map[string]string{"image_url": editReq.Mask}
	}
	requestBody, _ := json.Marshal(body)
	return executeXAIImage(ctx, "/v1/images/edits", requestBody, req.Credentials, req.Logger, "edit")
}

func executeXAIImage(ctx context.Context, path string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger, operation string) ([]domain.ImageResult, int, error) {
	if creds == nil {
		return nil, http.StatusUnauthorized, errors.New("xAI credentials are required")
	}
	endpoint := strings.TrimSuffix(openai.ResolveXAIImageBaseURL(creds), "/") + path
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("create xAI image request: %w", err)
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
	response, err := shared.StreamingHTTPClient.Do(request)
	duration := time.Since(start)
	if err != nil {
		if reqlog != nil {
			reqlog.Upstream(endpoint, http.MethodPost, http.StatusBadGateway, duration, err)
		}
		return nil, http.StatusBadGateway, fmt.Errorf("xAI image %s request failed: %w", operation, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxImageResponseBytes+1))
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("read xAI image response: %w", err)
	}
	if len(responseBody) > maxImageResponseBytes {
		return nil, http.StatusBadGateway, errors.New("xAI image response exceeds size limit")
	}
	if reqlog != nil {
		reqlog.Upstream(endpoint, http.MethodPost, response.StatusCode, duration, nil)
	}
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, fmt.Errorf("xAI returned %d: %s", response.StatusCode, string(responseBody))
	}
	var imageResponse domain.ImageGenerationsResponse
	if err := json.Unmarshal(responseBody, &imageResponse); err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("parse xAI image response: %w", err)
	}
	return imageResponse.Data, response.StatusCode, nil
}

var _ port.ImageProvider = (*ImageProvider)(nil)
