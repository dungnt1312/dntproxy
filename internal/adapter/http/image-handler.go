package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/openai"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/logger"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// imageGenRequestPartial is a partial parse for validation + stream detection.
type imageGenRequestPartial struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	ResponseFormat string `json:"response_format,omitempty"`
	Stream         bool   `json:"stream,omitempty"`
}

type imageEditsRequestPartial struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	ResponseFormat string `json:"response_format,omitempty"`
}

// imageGenerationsHandler handles POST /v1/images/generations
func imageGenerationsHandler(store port.CredentialStore, providers port.ProviderRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isImageGenerationDisabled(store, c) {
			return
		}

		requestID := uuid.New().String()
		rawJSON, err := readBodyLimited(c, 10*1024*1024)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Invalid request: %v", err), "type": "invalid_request_error"}})
			return
		}

		if !json.Valid(rawJSON) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid request: body must be valid JSON", "type": "invalid_request_error"}})
			return
		}

		var partial imageGenRequestPartial
		if err := json.Unmarshal(rawJSON, &partial); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Invalid request: %v", err), "type": "invalid_request_error"}})
			return
		}

		imageModel := strings.TrimSpace(partial.Model)
		if imageModel == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid request: model is required", "type": "invalid_request_error"}})
			return
		}

		prompt := strings.TrimSpace(partial.Prompt)
		if prompt == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid request: prompt is required", "type": "invalid_request_error"}})
			return
		}

		responseFormat := strings.TrimSpace(partial.ResponseFormat)
		if responseFormat == "" {
			responseFormat = "b64_json"
		}
		stream := partial.Stream

		provider, cleanModel, _ := parseImageModel(imageModel)
		exec := providers.GetExecutor(provider)
		if exec == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Unsupported provider for image generation: %s", provider), "type": "invalid_request_error"}})
			return
		}

		accountSelector := service.NewAccountSelector(store)
		creds, selErr := accountSelector.SelectCredentials(provider, nil, cleanModel, nil)
		if selErr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": fmt.Sprintf("No available accounts: %v", selErr), "type": "server_error"}})
			return
		}

		reqlog := logger.NewRequestLog(requestID)

		if stream {
			handleStreamingImageGeneration(c, cleanModel, rawJSON, creds, reqlog, responseFormat)
			return
		}

		result, statusCode, execErr := executeImageGeneration(exec, cleanModel, rawJSON, creds, reqlog, accountSelector)
		if execErr != nil {
			if statusCode == 0 {
				statusCode = http.StatusBadGateway
			}
			c.JSON(statusCode, gin.H{"error": gin.H{"message": execErr.Error(), "type": "api_error"}})
			return
		}

		resp, err := openai.BuildOpenAIImageResponse(result, time.Now().Unix(), responseFormat)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": fmt.Sprintf("Build response: %v", err), "type": "server_error"}})
			return
		}

		c.Header("Content-Type", "application/json")
		c.Data(http.StatusOK, "application/json", resp)
	}
}

// handleStreamingImageGeneration handles streaming image generation with SSE.
func handleStreamingImageGeneration(c *gin.Context, model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger, responseFormat string) {
	if !openai.IsCodexOAuthExport(creds) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Streaming image generation requires OAuth/Codex authentication", "type": "invalid_request_error"}})
		return
	}

	translatedBody, err := openai.TranslateImageGenerationsToCodex(body, model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": fmt.Sprintf("Translate request: %v", err), "type": "server_error"}})
		return
	}

	url := openai.CodexResponsesURLExport()
	req, err := http.NewRequest("POST", url, bytes.NewReader(translatedBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "Create request failed", "type": "server_error"}})
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("originator", "codex-cli")
	req.Header.Set("User-Agent", "codex-cli/1.0.18 (macOS; arm64)")
	req.Header.Set("session_id", fmt.Sprintf("%d-%s", time.Now().UnixMilli(), openai.RandomAlphaNumExport(9)))

	reqlog.SetBodies(shared.PrepareLoggedBody(translatedBody), "")
	start := time.Now()

	resp, err := openai.CodexHTTPClientExport().Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(url, "POST", 502, duration, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": fmt.Sprintf("Codex request failed: %v", err), "type": "api_error"}})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		reqlog.Upstream(url, "POST", resp.StatusCode, duration, fmt.Errorf("%s", string(bodyBytes)))
		c.JSON(resp.StatusCode, gin.H{"error": gin.H{"message": fmt.Sprintf("Codex returned %d: %s", resp.StatusCode, string(bodyBytes)), "type": "api_error"}})
		return
	}

	reqlog.Upstream(url, "POST", resp.StatusCode, duration, nil)

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "Streaming not supported", "type": "server_error"}})
		return
	}

	// Keepalive ticker
	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	// Start streaming
	chunks := openai.ParseCodexImageStream(resp.Body)
	var accumulatedImages []domain.ImageResult
	var completed bool

	for !completed {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				completed = true
				break
			}

			if chunk.IsDone {
				completed = true
				// Send final response with all images
				if len(accumulatedImages) > 0 {
					resp, err := openai.BuildOpenAIImageResponse(accumulatedImages, chunk.CreatedAt, responseFormat)
					if err == nil {
						fmt.Fprintf(c.Writer, "data: %s\n\n", resp)
						flusher.Flush()
					}
				}
				fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
				flusher.Flush()
				break
			}

			if chunk.IsPartial {
				// Send partial frame
				frame := openai.BuildImageSSEStreamFrame(domain.ImageResult{
					B64JSON: chunk.B64JSON,
				}, len(accumulatedImages), false, responseFormat)
				if frame != nil {
					c.Writer.Write(frame)
					flusher.Flush()
				}
			} else {
				// Complete image - accumulate
				accumulatedImages = append(accumulatedImages, domain.ImageResult{
					B64JSON:       chunk.B64JSON,
					RevisedPrompt: chunk.RevisedPrompt,
				})
			}

		case <-keepAlive.C:
			fmt.Fprintf(c.Writer, ": keep-alive\n\n")
			flusher.Flush()

		case <-c.Request.Context().Done():
			return
		}
	}
}

// imageEditsHandler handles POST /v1/images/edits
func imageEditsHandler(store port.CredentialStore, providers port.ProviderRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isImageGenerationDisabled(store, c) {
			return
		}

		requestID := uuid.New().String()
		contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))

		if openai.NeedsMultipartEdit(contentType) {
			form, err := c.MultipartForm()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Invalid multipart form: %v", err), "type": "invalid_request_error"}})
				return
			}

			imageModel := formValue(form, "model")
			if imageModel == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid request: model is required", "type": "invalid_request_error"}})
				return
			}

			provider, cleanModel, _ := parseImageModel(imageModel)
			exec := providers.GetExecutor(provider)
			if exec == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Unsupported provider: %s", provider), "type": "invalid_request_error"}})
				return
			}

			accountSelector := service.NewAccountSelector(store)
			creds, selErr := accountSelector.SelectCredentials(provider, nil, cleanModel, nil)
			if selErr != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": fmt.Sprintf("No available accounts: %v", selErr), "type": "server_error"}})
				return
			}

			responseFormat := formValue(form, "response_format")
			if responseFormat == "" {
				responseFormat = "b64_json"
			}

			reqlog := logger.NewRequestLog(requestID)
			result, statusCode, execErr := executeImageEdit(exec, cleanModel, form, creds, reqlog, accountSelector)
			if execErr != nil {
				if statusCode == 0 {
					statusCode = http.StatusBadGateway
				}
				c.JSON(statusCode, gin.H{"error": gin.H{"message": execErr.Error(), "type": "api_error"}})
				return
			}

			resp, err := openai.BuildOpenAIImageResponse(result, time.Now().Unix(), responseFormat)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": fmt.Sprintf("Build response: %v", err), "type": "server_error"}})
				return
			}

			c.Header("Content-Type", "application/json")
			c.Data(http.StatusOK, "application/json", resp)
			return
		}

		// JSON body
		rawJSON, err := readBodyLimited(c, 10*1024*1024)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Invalid request: %v", err), "type": "invalid_request_error"}})
			return
		}

		if !json.Valid(rawJSON) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid request: body must be valid JSON", "type": "invalid_request_error"}})
			return
		}

		var partial imageEditsRequestPartial
		if err := json.Unmarshal(rawJSON, &partial); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Invalid request: %v", err), "type": "invalid_request_error"}})
			return
		}

		imageModel := strings.TrimSpace(partial.Model)
		if imageModel == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid request: model is required", "type": "invalid_request_error"}})
			return
		}

		provider, cleanModel, _ := parseImageModel(imageModel)
		exec := providers.GetExecutor(provider)
		if exec == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Unsupported provider: %s", provider), "type": "invalid_request_error"}})
			return
		}

		accountSelector := service.NewAccountSelector(store)
		creds, selErr := accountSelector.SelectCredentials(provider, nil, cleanModel, nil)
		if selErr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": fmt.Sprintf("No available accounts: %v", selErr), "type": "server_error"}})
			return
		}

		responseFormat := strings.TrimSpace(partial.ResponseFormat)
		if responseFormat == "" {
			responseFormat = "b64_json"
		}

		reqlog := logger.NewRequestLog(requestID)
		result, statusCode, execErr := executeImageEditJSON(exec, cleanModel, rawJSON, creds, reqlog, accountSelector)
		if execErr != nil {
			if statusCode == 0 {
				statusCode = http.StatusBadGateway
			}
			c.JSON(statusCode, gin.H{"error": gin.H{"message": execErr.Error(), "type": "api_error"}})
			return
		}

		resp, err := openai.BuildOpenAIImageResponse(result, time.Now().Unix(), responseFormat)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": fmt.Sprintf("Build response: %v", err), "type": "server_error"}})
			return
		}

		c.Header("Content-Type", "application/json")
		c.Data(http.StatusOK, "application/json", resp)
	}
}

// --- Image Execution Functions (unchanged) ---

func executeImageGeneration(exec port.ProviderExecutor, model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger, selector port.AccountSelector) ([]domain.ImageResult, int, error) {
	if openai.IsXAIImagesModel(model) {
		return executeXAIImageGeneration(model, body, creds, reqlog)
	}
	if creds.Provider == "openai" || creds.Provider == "openai-compatible" {
		return executeOpenAIImageGeneration(model, body, creds, reqlog, selector)
	}
	return nil, http.StatusBadRequest, fmt.Errorf("image generation not supported for provider: %s", creds.Provider)
}

func executeImageEdit(exec port.ProviderExecutor, model string, form *multipart.Form, creds *domain.Credentials, reqlog port.RequestLogger, selector port.AccountSelector) ([]domain.ImageResult, int, error) {
	if openai.IsXAIImagesModel(model) {
		return nil, http.StatusBadRequest, fmt.Errorf("multipart image edit not supported for xAI models, use JSON format")
	}
	if creds.Provider == "openai" || creds.Provider == "openai-compatible" {
		return executeOpenAIImageEditMultipart(model, form, creds, reqlog, selector)
	}
	return nil, http.StatusBadRequest, fmt.Errorf("image editing not supported for provider: %s", creds.Provider)
}

func executeImageEditJSON(exec port.ProviderExecutor, model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger, selector port.AccountSelector) ([]domain.ImageResult, int, error) {
	if openai.IsXAIImagesModel(model) {
		return executeXAIImageEdit(model, body, creds, reqlog)
	}
	if creds.Provider == "openai" || creds.Provider == "openai-compatible" {
		return executeOpenAIImageEditJSON(model, body, creds, reqlog, selector)
	}
	return nil, http.StatusBadRequest, fmt.Errorf("image editing not supported for provider: %s", creds.Provider)
}

func executeOpenAIImageGeneration(model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger, selector port.AccountSelector) ([]domain.ImageResult, int, error) {
	if openai.IsCodexOAuthExport(creds) {
		return executeCodexImageGeneration(model, body, creds, reqlog)
	}
	return executeOpenAICompatImageGeneration(model, body, creds, reqlog)
}

func executeOpenAIImageEditJSON(model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger, selector port.AccountSelector) ([]domain.ImageResult, int, error) {
	if openai.IsCodexOAuthExport(creds) {
		return executeCodexImageEditJSON(model, body, creds, reqlog)
	}
	return executeOpenAICompatImageEdit(model, body, creds, reqlog)
}

func executeOpenAIImageEditMultipart(model string, form *multipart.Form, creds *domain.Credentials, reqlog port.RequestLogger, selector port.AccountSelector) ([]domain.ImageResult, int, error) {
	if openai.IsCodexOAuthExport(creds) {
		return executeCodexImageEditMultipart(model, form, creds, reqlog)
	}
	return nil, http.StatusBadRequest, fmt.Errorf("multipart image edit requires OAuth/Codex authentication")
}

func executeOpenAICompatImageGeneration(model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger) ([]domain.ImageResult, int, error) {
	req, err := openai.ForwardImageGeneration(body, creds.BaseURL, creds.APIKey)
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	reqlog.SetBodies(shared.PrepareLoggedBody(body), "")
	start := time.Now()

	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(req.URL.String(), "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("image generation request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		reqlog.Upstream(req.URL.String(), "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("read response: %w", err)
	}

	reqlog.Upstream(req.URL.String(), "POST", resp.StatusCode, duration, nil)

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var genResp domain.ImageGenerationsResponse
	if err := json.Unmarshal(respBytes, &genResp); err != nil {
		return nil, 500, fmt.Errorf("parse image response: %w", err)
	}

	return genResp.Data, resp.StatusCode, nil
}

func executeOpenAICompatImageEdit(model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger) ([]domain.ImageResult, int, error) {
	req, err := openai.ForwardImageEdit(body, creds.BaseURL, creds.APIKey)
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	reqlog.SetBodies(shared.PrepareLoggedBody(body), "")
	start := time.Now()

	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(req.URL.String(), "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("image edit request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		reqlog.Upstream(req.URL.String(), "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("read response: %w", err)
	}

	reqlog.Upstream(req.URL.String(), "POST", resp.StatusCode, duration, nil)

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var genResp domain.ImageGenerationsResponse
	if err := json.Unmarshal(respBytes, &genResp); err != nil {
		return nil, 500, fmt.Errorf("parse image edit response: %w", err)
	}

	return genResp.Data, resp.StatusCode, nil
}

func executeCodexImageGeneration(model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger) ([]domain.ImageResult, int, error) {
	translatedBody, err := openai.TranslateImageGenerationsToCodex(body, model)
	if err != nil {
		return nil, 500, fmt.Errorf("translate image request: %w", err)
	}
	return executeCodexImageRequest(translatedBody, creds, reqlog)
}

func executeCodexImageEditJSON(model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger) ([]domain.ImageResult, int, error) {
	translatedBody, err := openai.TranslateImageEditsToCodex(body, model)
	if err != nil {
		return nil, 500, fmt.Errorf("translate image edit request: %w", err)
	}
	return executeCodexImageRequest(translatedBody, creds, reqlog)
}

func executeCodexImageEditMultipart(model string, form *multipart.Form, creds *domain.Credentials, reqlog port.RequestLogger) ([]domain.ImageResult, int, error) {
	translatedBody, err := openai.TranslateMultipartEditToCodex(form, model)
	if err != nil {
		return nil, 500, fmt.Errorf("translate multipart edit request: %w", err)
	}
	return executeCodexImageRequest(translatedBody, creds, reqlog)
}

func executeCodexImageRequest(translatedBody []byte, creds *domain.Credentials, reqlog port.RequestLogger) ([]domain.ImageResult, int, error) {
	url := openai.CodexResponsesURLExport()

	req, err := http.NewRequest("POST", url, bytes.NewReader(translatedBody))
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("originator", "codex-cli")
	req.Header.Set("User-Agent", "codex-cli/1.0.18 (macOS; arm64)")
	req.Header.Set("session_id", fmt.Sprintf("%d-%s", time.Now().UnixMilli(), openai.RandomAlphaNumExport(9)))

	reqlog.SetBodies(shared.PrepareLoggedBody(translatedBody), "")
	start := time.Now()

	resp, err := openai.CodexHTTPClientExport().Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(url, "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("codex image request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		reqlog.Upstream(url, "POST", resp.StatusCode, duration, fmt.Errorf("%s", string(bodyBytes)))
		return nil, resp.StatusCode, fmt.Errorf("codex returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	reqlog.Upstream(url, "POST", resp.StatusCode, duration, nil)

	results, _, _, parseErr := openai.ParseCodexImageResponse(resp.Body)
	if parseErr != nil {
		return nil, 502, fmt.Errorf("parse codex image response: %w", parseErr)
	}

	if len(results) == 0 {
		return nil, 502, fmt.Errorf("upstream did not return image output")
	}

	return results, 200, nil
}

// --- xAI Image Generation ---

func executeXAIImageGeneration(model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger) ([]domain.ImageResult, int, error) {
	xaiBody, _, err := openai.BuildXAIImageRequest(body, model)
	if err != nil {
		return nil, 400, fmt.Errorf("build xAI request: %w", err)
	}

	baseURL := openai.ResolveXAIImageBaseURL(creds)
	url := strings.TrimSuffix(baseURL, "/") + "/v1/images/generations"

	req, err := http.NewRequest("POST", url, bytes.NewReader(xaiBody))
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	apiKey := creds.APIKey
	if apiKey == "" {
		apiKey = creds.AccessToken
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	reqlog.SetBodies(shared.PrepareLoggedBody(xaiBody), "")
	start := time.Now()

	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(url, "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("xAI image request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		reqlog.Upstream(url, "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("read response: %w", err)
	}

	reqlog.Upstream(url, "POST", resp.StatusCode, duration, nil)

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("xAI returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var genResp domain.ImageGenerationsResponse
	if err := json.Unmarshal(respBytes, &genResp); err != nil {
		return nil, 500, fmt.Errorf("parse xAI image response: %w", err)
	}

	return genResp.Data, resp.StatusCode, nil
}

// xAI image edit - builds a generate request since xAI /v1/images/edits is similar
func executeXAIImageEdit(model string, body []byte, creds *domain.Credentials, reqlog port.RequestLogger) ([]domain.ImageResult, int, error) {
	// xAI uses the same /v1/images/edits endpoint with similar format
	baseURL := openai.ResolveXAIImageBaseURL(creds)
	url := strings.TrimSuffix(baseURL, "/") + "/v1/images/edits"

	// Parse the original request to keep images and mask
	var editReq struct {
		Model          string `json:"model"`
		Prompt         string `json:"prompt"`
		Images         []struct {
			ImageURL string `json:"image_url"`
		} `json:"images"`
		Image          string `json:"image"`
		Mask           string `json:"mask"`
		N              int    `json:"n"`
		Size           string `json:"size"`
		ResponseFormat string `json:"response_format"`
	}
	if err := json.Unmarshal(body, &editReq); err != nil {
		return nil, 400, fmt.Errorf("invalid edit request: %w", err)
	}

	// Build xAI format - same as generation but with images included
	xaiBody := map[string]interface{}{
		"model":           openai.CanonicalXAIModelExport(model),
		"prompt":          editReq.Prompt,
		"n":               editReq.N,
		"response_format": editReq.ResponseFormat,
	}

	// Map size to aspect_ratio
	if editReq.Size != "" {
		aspectRatio := "1:1"
		switch editReq.Size {
		case "1792x1024":
			aspectRatio = "16:9"
		case "1024x1792":
			aspectRatio = "9:16"
		}
		xaiBody["aspect_ratio"] = aspectRatio
	}

	// Add images
	var imageURLs []map[string]string
	for _, img := range editReq.Images {
		if img.ImageURL != "" {
			imageURLs = append(imageURLs, map[string]string{"image_url": img.ImageURL})
		}
	}
	if len(imageURLs) > 0 {
		xaiBody["images"] = imageURLs
	}
	if editReq.Mask != "" {
		xaiBody["mask"] = map[string]string{"image_url": editReq.Mask}
	}

	reqJSON, _ := json.Marshal(xaiBody)

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	apiKey := creds.APIKey
	if apiKey == "" {
		apiKey = creds.AccessToken
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	reqlog.SetBodies(shared.PrepareLoggedBody(reqJSON), "")
	start := time.Now()

	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(url, "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("xAI image edit request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		reqlog.Upstream(url, "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("read response: %w", err)
	}

	reqlog.Upstream(url, "POST", resp.StatusCode, duration, nil)

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("xAI returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var genResp domain.ImageGenerationsResponse
	if err := json.Unmarshal(respBytes, &genResp); err != nil {
		return nil, 500, fmt.Errorf("parse xAI image edit response: %w", err)
	}

	return genResp.Data, resp.StatusCode, nil
}

func parseImageModel(modelStr string) (provider, model, connectionID string) {
	info, err := service.ParseModelString(modelStr)
	if err != nil {
		return "openai", modelStr, ""
	}
	return info.Provider, info.Model, info.ConnectionID
}

func readBodyLimited(c *gin.Context, maxSize int64) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, fmt.Errorf("empty request body")
	}
	limitedReader := io.LimitReader(c.Request.Body, maxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if len(data) > int(maxSize) {
		return nil, fmt.Errorf("request body too large (max %d bytes)", maxSize)
	}
	return data, nil
}

func formValue(form *multipart.Form, key string) string {
	if form == nil || len(form.Value[key]) == 0 {
		return ""
	}
	return strings.TrimSpace(form.Value[key][0])
}

func isImageGenerationDisabled(store port.CredentialStore, c *gin.Context) bool {
	settings, err := store.GetSettings()
	if err != nil || settings == nil {
		return false
	}
	if settings.DisableImageGeneration {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "Image generation is disabled", "type": "not_found"}})
		return true
	}
	return false
}
