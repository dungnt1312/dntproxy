package byteplus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

const (
	DefaultBaseURL       = "https://ark.ap-southeast.bytepluses.com/api/v3"
	maxImageResponseSize = 64 * 1024 * 1024
)

// ImageResponseHeaderTimeout accounts for synchronous image generation latency.
const ImageResponseHeaderTimeout = 180 * time.Second

type ImageClient struct {
	HTTPClient *http.Client
}

func NewImageClient() *ImageClient {
	return &ImageClient{HTTPClient: &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:          20,
			MaxIdleConnsPerHost:   5,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: ImageResponseHeaderTimeout,
			ForceAttemptHTTP2:     true,
		},
		CheckRedirect: shared.CheckRedirectSafe,
	}}
}

// Execute sends an already translated request to ModelArk.
func (client *ImageClient) Execute(
	ctx context.Context,
	nativeBody []byte,
	creds *domain.Credentials,
	reqlog port.RequestLogger,
) ([]domain.ImageResult, int, error) {
	if creds == nil {
		return nil, http.StatusInternalServerError, errors.New("BytePlus credentials are required")
	}
	endpoint, err := ResolveImageEndpoint(creds.BaseURL)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	apiKey := strings.TrimSpace(creds.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(creds.AccessToken)
	}
	if apiKey == "" {
		return nil, http.StatusUnauthorized, errors.New("BytePlus API key is required")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(nativeBody))
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("create BytePlus image request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	if reqlog != nil {
		reqlog.SetBodies(shared.PrepareLoggedBody(nativeBody), "")
	}
	start := time.Now()
	response, err := client.httpClient().Do(request)
	duration := time.Since(start)
	if err != nil {
		logUpstream(reqlog, endpoint, http.StatusBadGateway, duration, err)
		return nil, http.StatusBadGateway, fmt.Errorf("BytePlus image request failed: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxImageResponseSize+1))
	if err != nil {
		logUpstream(reqlog, endpoint, http.StatusBadGateway, duration, err)
		return nil, http.StatusBadGateway, fmt.Errorf("read BytePlus image response: %w", err)
	}
	if len(responseBody) > maxImageResponseSize {
		err = fmt.Errorf("BytePlus image response exceeds %d bytes", maxImageResponseSize)
		logUpstream(reqlog, endpoint, http.StatusBadGateway, duration, err)
		return nil, http.StatusBadGateway, err
	}
	if reqlog != nil {
		reqlog.SetBodies(shared.PrepareLoggedBody(nativeBody), shared.PrepareLoggedBody(sanitizeBytePlusBody(responseBody)))
	}

	results, parseErr := ParseImageResponse(responseBody)
	if parseErr != nil {
		status := HTTPStatus(parseErr)
		if status == 0 {
			status = response.StatusCode
			if status < http.StatusBadRequest {
				status = http.StatusBadGateway
			}
		}
		logUpstream(reqlog, endpoint, response.StatusCode, duration, parseErr)
		return nil, status, parseErr
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf("BytePlus returned HTTP %d", response.StatusCode)
		logUpstream(reqlog, endpoint, response.StatusCode, duration, err)
		return nil, response.StatusCode, err
	}
	logUpstream(reqlog, endpoint, response.StatusCode, duration, nil)
	return results, response.StatusCode, nil
}

func ResolveImageEndpoint(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid BytePlus base URL: %q", baseURL)
	}
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, "/images/generations") {
		path += "/images/generations"
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.String(), nil
}

func (client *ImageClient) httpClient() *http.Client {
	if client != nil && client.HTTPClient != nil {
		return client.HTTPClient
	}
	return http.DefaultClient
}

func logUpstream(logger port.RequestLogger, endpoint string, status int, duration time.Duration, err error) {
	if logger != nil {
		logger.Upstream(endpoint, http.MethodPost, status, duration, err)
	}
}
