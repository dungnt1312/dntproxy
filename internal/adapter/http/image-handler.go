package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/openai"
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
func imageGenerationsHandler(store port.CredentialStore, imageProviders port.ImageProviderRegistry) gin.HandlerFunc {
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

		provider, cleanModel, pinnedConnID := parseImageModel(imageModel)
		imageProvider := imageProviders.GetImageProvider(provider)
		if imageProvider == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Unsupported provider for image generation: %s", provider), "type": "invalid_request_error"}})
			return
		}
		capabilities := imageProvider.Capabilities(cleanModel)
		if !capabilities.Generate {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Image generation not supported for model: %s", imageModel), "type": "invalid_request_error"}})
			return
		}
		if !capabilities.SupportsResponseFormat(responseFormat) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Response format %q is not supported for model: %s", responseFormat, imageModel), "type": "invalid_request_error"}})
			return
		}

		tenantID := GetTenantID(c)
		creds, selErr := selectImageCredentials(c, store, provider, cleanModel, imageModel, pinnedConnID, tenantID)
		if selErr != nil {
			writeImageSelectionError(c, selErr)
			return
		}

		reqlog := logger.NewRequestLogForTenant(requestID, tenantID)
		imageRequest := port.ImageRequest{
			Model:       cleanModel,
			Body:        rawJSON,
			Credentials: creds,
			Logger:      reqlog,
		}

		if stream {
			if !capabilities.Streaming {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Streaming image generation not supported for model: %s", imageModel), "type": "invalid_request_error"}})
				return
			}
			handleStreamingImageGeneration(c, store, imageProvider, imageRequest, responseFormat)
			return
		}

		result, statusCode, execErr := imageProvider.Generate(c.Request.Context(), imageRequest)
		if execErr != nil {
			markImageFailure(store, creds, cleanModel, statusCode, execErr)
			if statusCode == 0 {
				statusCode = http.StatusBadGateway
			}
			c.JSON(statusCode, gin.H{"error": gin.H{"message": execErr.Error(), "type": imageExecutionErrorType(statusCode)}})
			return
		}
		clearImageFailure(store, creds, cleanModel)

		resp, err := openai.BuildOpenAIImageResponse(result, time.Now().Unix(), responseFormat)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": fmt.Sprintf("Build response: %v", err), "type": "server_error"}})
			return
		}

		c.Header("Content-Type", "application/json")
		c.Data(http.StatusOK, "application/json", resp)
	}
}

func imageExecutionErrorType(statusCode int) string {
	if statusCode == http.StatusBadRequest {
		return "invalid_request_error"
	}
	return "api_error"
}

// writeImageSelectionError answers a failed credential selection. A pool that is
// only cooling down is a 429 with the earliest recovery hint, not a 503: the
// caller can retry as soon as the hint says instead of guessing.
func writeImageSelectionError(c *gin.Context, selErr error) {
	if strings.Contains(strings.ToLower(selErr.Error()), "not allowed by api key policy") {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"message": selErr.Error(), "type": "invalid_request_error"}})
		return
	}
	var selectionErr *service.AccountSelectionError
	if errors.As(selErr, &selectionErr) &&
		(selectionErr.Kind == service.SelectionErrRateLimited || selectionErr.Kind == service.SelectionErrModelLocked) {
		writeRetryAfterHeader(c, selectionErr.RetryAfter)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"message": selErr.Error(), "type": "rate_limit_error"}})
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": fmt.Sprintf("No available accounts: %v", selErr), "type": "server_error"}})
}

// markImageFailure records an image-route failure on the connection that was
// selected, so the next request routes around a broken account. Image routes
// resolve credentials directly and bypass ChatService, which is what normally
// records failures; without this an account that always fails (spent credit,
// revoked key, banned account) kept being picked for every following request.
func markImageFailure(store port.CredentialStore, creds *domain.Credentials, model string, statusCode int, err error) {
	if store == nil || creds == nil || creds.ConnectionID == "" {
		return
	}
	status := statusCode
	if status <= 0 {
		status = http.StatusBadGateway
	}
	var message string
	if err != nil {
		message = err.Error()
	}
	_ = service.NewAccountSelector(store).MarkUnavailable(creds.ConnectionID, status, message, model)
}

// clearImageFailure drops a stale cooldown after a successful image request,
// mirroring what ChatService does for chat completions.
func clearImageFailure(store port.CredentialStore, creds *domain.Credentials, model string) {
	if store == nil || creds == nil || creds.ConnectionID == "" {
		return
	}
	_ = service.NewAccountSelector(store).ClearError(creds.ConnectionID, model)
}

// handleStreamingImageGeneration handles streaming image generation with SSE.
func handleStreamingImageGeneration(c *gin.Context, store port.CredentialStore, imageProvider port.ImageProvider, request port.ImageRequest, responseFormat string) {
	streamingProvider, ok := imageProvider.(port.StreamingImageProvider)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Streaming image generation is not available for this provider", "type": "invalid_request_error"}})
		return
	}
	events, statusCode, err := streamingProvider.StreamGenerate(c.Request.Context(), request)
	if err != nil {
		markImageFailure(store, request.Credentials, request.Model, statusCode, err)
		if statusCode == 0 {
			statusCode = http.StatusBadGateway
		}
		c.JSON(statusCode, gin.H{"error": gin.H{"message": err.Error(), "type": imageExecutionErrorType(statusCode)}})
		return
	}
	clearImageFailure(store, request.Credentials, request.Model)

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
	var accumulatedImages []domain.ImageResult
	var completed bool

	for !completed {
		select {
		case event, ok := <-events:
			if !ok {
				completed = true
				break
			}

			if event.Done {
				completed = true
				// Send final response with all images
				if len(accumulatedImages) > 0 {
					resp, err := openai.BuildOpenAIImageResponse(accumulatedImages, event.Created, responseFormat)
					if err == nil {
						fmt.Fprintf(c.Writer, "data: %s\n\n", resp)
						flusher.Flush()
					}
				}
				fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
				flusher.Flush()
				break
			}

			if event.Partial && event.Result != nil {
				// Send partial frame
				frame := openai.BuildImageSSEStreamFrame(*event.Result, len(accumulatedImages), false, responseFormat)
				if frame != nil {
					c.Writer.Write(frame)
					flusher.Flush()
				}
			} else if event.Result != nil {
				// Complete image - accumulate
				accumulatedImages = append(accumulatedImages, *event.Result)
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
func imageEditsHandler(store port.CredentialStore, imageProviders port.ImageProviderRegistry) gin.HandlerFunc {
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

			provider, cleanModel, pinnedConnID := parseImageModel(imageModel)
			imageProvider := imageProviders.GetImageProvider(provider)
			if imageProvider == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Unsupported provider: %s", provider), "type": "invalid_request_error"}})
				return
			}
			capabilities := imageProvider.Capabilities(cleanModel)
			if !capabilities.Edit || !capabilities.Multipart {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Multipart image editing not supported for model: %s", imageModel), "type": "invalid_request_error"}})
				return
			}

			tenantID := GetTenantID(c)
			creds, selErr := selectImageCredentials(c, store, provider, cleanModel, imageModel, pinnedConnID, tenantID)
			if selErr != nil {
				writeImageSelectionError(c, selErr)
				return
			}

			responseFormat := formValue(form, "response_format")
			if responseFormat == "" {
				responseFormat = "b64_json"
			}
			if !capabilities.SupportsResponseFormat(responseFormat) {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Response format %q is not supported for model: %s", responseFormat, imageModel), "type": "invalid_request_error"}})
				return
			}

			reqlog := logger.NewRequestLogForTenant(requestID, tenantID)
			result, statusCode, execErr := imageProvider.Edit(c.Request.Context(), port.ImageRequest{
				Model:       cleanModel,
				Form:        form,
				Credentials: creds,
				Logger:      reqlog,
			})
			if execErr != nil {
				markImageFailure(store, creds, cleanModel, statusCode, execErr)
				if statusCode == 0 {
					statusCode = http.StatusBadGateway
				}
				c.JSON(statusCode, gin.H{"error": gin.H{"message": execErr.Error(), "type": imageExecutionErrorType(statusCode)}})
				return
			}
			clearImageFailure(store, creds, cleanModel)

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

		provider, cleanModel, pinnedConnID := parseImageModel(imageModel)
		imageProvider := imageProviders.GetImageProvider(provider)
		if imageProvider == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Unsupported provider: %s", provider), "type": "invalid_request_error"}})
			return
		}
		capabilities := imageProvider.Capabilities(cleanModel)
		if !capabilities.Edit {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Image editing not supported for model: %s", imageModel), "type": "invalid_request_error"}})
			return
		}

		tenantID := GetTenantID(c)
		creds, selErr := selectImageCredentials(c, store, provider, cleanModel, imageModel, pinnedConnID, tenantID)
		if selErr != nil {
			writeImageSelectionError(c, selErr)
			return
		}

		responseFormat := strings.TrimSpace(partial.ResponseFormat)
		if responseFormat == "" {
			responseFormat = "b64_json"
		}
		if !capabilities.SupportsResponseFormat(responseFormat) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": fmt.Sprintf("Response format %q is not supported for model: %s", responseFormat, imageModel), "type": "invalid_request_error"}})
			return
		}

		reqlog := logger.NewRequestLogForTenant(requestID, tenantID)
		result, statusCode, execErr := imageProvider.Edit(c.Request.Context(), port.ImageRequest{
			Model:       cleanModel,
			Body:        rawJSON,
			Credentials: creds,
			Logger:      reqlog,
		})
		if execErr != nil {
			markImageFailure(store, creds, cleanModel, statusCode, execErr)
			if statusCode == 0 {
				statusCode = http.StatusBadGateway
			}
			c.JSON(statusCode, gin.H{"error": gin.H{"message": execErr.Error(), "type": imageExecutionErrorType(statusCode)}})
			return
		}
		clearImageFailure(store, creds, cleanModel)

		resp, err := openai.BuildOpenAIImageResponse(result, time.Now().Unix(), responseFormat)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": fmt.Sprintf("Build response: %v", err), "type": "server_error"}})
			return
		}

		c.Header("Content-Type", "application/json")
		c.Data(http.StatusOK, "application/json", resp)
	}
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

// selectImageCredentials applies API-key policy and optional @connection pin for image routes.
func selectImageCredentials(c *gin.Context, store port.CredentialStore, provider, cleanModel, fullModel, pinnedConnID, tenantID string) (*domain.Credentials, error) {
	policy := extractAPIKeyPolicy(c)
	if policy != nil && !service.ModelAllowedByPolicy(fullModel, policy) {
		return nil, fmt.Errorf("model not allowed by API key policy")
	}
	if pinnedConnID != "" && !service.ConnectionAllowed(pinnedConnID, policy) {
		return nil, fmt.Errorf("connection not allowed by API key policy")
	}
	var allowed []string
	if policy != nil {
		allowed = append(allowed, policy.AllowedConnectionIDs...)
	}
	if pinnedConnID != "" {
		allowed = []string{pinnedConnID}
	}
	return service.NewAccountSelector(store).SelectCredentials(provider, nil, cleanModel, allowed, tenantID)
}
