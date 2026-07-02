package openai

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

const (
	codexImagesMainModel   = "gpt-5.4-mini"
	codexDefaultImageModel = "gpt-image-2"
	codexGPTImage15Model   = "gpt-image-1.5"

	codexImageToolType       = "image_generation"
	codexImageCallType       = "image_generation_call"
	codexImageActionGenerate = "generate"
	codexImageActionEdit     = "edit"
)

// --- Request Translation ---

type imageGenRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	Style          string `json:"style,omitempty"`
}

type imageEditRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Image   string `json:"image,omitempty"`
	Images  []struct {
		ImageURL string `json:"image_url"`
	} `json:"images,omitempty"`
	MaskStr        string `json:"mask,omitempty"`
	MaskObj        *struct {
		ImageURL string `json:"image_url"`
	} `json:"-"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

func TranslateImageGenerationsToCodex(rawJSON []byte, routeModel string) ([]byte, error) {
	var req imageGenRequest
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return nil, fmt.Errorf("invalid image generation request JSON: %w", err)
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required for image generation")
	}

	tool := buildImageGenTool(req, routeModel, codexImageActionGenerate)
	toolJSON, _ := json.Marshal(tool)
	return buildImagesResponsesBody(prompt, nil, toolJSON)
}

func TranslateImageEditsToCodex(rawJSON []byte, routeModel string) ([]byte, error) {
	var req imageEditRequest
	if err := json.Unmarshal(rawJSON, &req); err != nil {
		return nil, fmt.Errorf("invalid image edit request JSON: %w", err)
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required for image editing")
	}

	var images []string
	for _, img := range req.Images {
		if url := strings.TrimSpace(img.ImageURL); url != "" {
			images = append(images, url)
		}
	}
	if singleImg := strings.TrimSpace(req.Image); singleImg != "" {
		images = append(images, singleImg)
	}

	tool := buildImageGenToolFromEdit(req, routeModel)
	if mask := strings.TrimSpace(req.MaskStr); mask != "" {
		tool["input_image_mask"] = map[string]string{"image_url": mask}
	} else if req.MaskObj != nil && req.MaskObj.ImageURL != "" {
		tool["input_image_mask"] = map[string]string{"image_url": req.MaskObj.ImageURL}
	}

	toolJSON, _ := json.Marshal(tool)
	return buildImagesResponsesBody(prompt, images, toolJSON)
}

func TranslateMultipartEditToCodex(form *multipart.Form, routeModel string) ([]byte, error) {
	prompt := strings.TrimSpace(formValue(form, "prompt"))
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required for image editing")
	}

	imageModel := imageToolModel(formValue(form, "model"), routeModel)

	tool := map[string]interface{}{
		"type":   codexImageToolType,
		"action": codexImageActionEdit,
		"model":  imageModel,
	}

	for _, field := range []string{"size", "quality", "background", "input_fidelity", "moderation"} {
		if v := strings.TrimSpace(formValue(form, field)); v != "" {
			tool[field] = v
		}
	}

	var images []string
	for _, fh := range multipartImageFiles(form) {
		dataURL, err := multipartFileToDataURL(fh)
		if err != nil {
			return nil, err
		}
		images = append(images, dataURL)
	}

	if maskFiles := form.File["mask"]; len(maskFiles) > 0 && maskFiles[0] != nil {
		dataURL, err := multipartFileToDataURL(maskFiles[0])
		if err != nil {
			return nil, err
		}
		tool["input_image_mask"] = map[string]string{"image_url": dataURL}
	}

	toolJSON, _ := json.Marshal(tool)
	return buildImagesResponsesBody(prompt, images, toolJSON)
}

func buildImageGenTool(req imageGenRequest, routeModel string, action string) map[string]interface{} {
	tool := map[string]interface{}{
		"type":   codexImageToolType,
		"action": action,
		"model":  imageToolModel(req.Model, routeModel),
	}
	if req.Size != "" {
		tool["size"] = req.Size
	}
	if req.Quality != "" {
		tool["quality"] = req.Quality
	}
	return tool
}

func buildImageGenToolFromEdit(req imageEditRequest, routeModel string) map[string]interface{} {
	tool := map[string]interface{}{
		"type":   codexImageToolType,
		"action": codexImageActionEdit,
		"model":  imageToolModel(req.Model, routeModel),
	}
	if req.Size != "" {
		tool["size"] = req.Size
	}
	return tool
}

func buildImagesResponsesBody(prompt string, images []string, toolJSON []byte) ([]byte, error) {
	inputContent := []map[string]interface{}{
		{"type": "input_text", "text": prompt},
	}
	for _, img := range images {
		if strings.TrimSpace(img) == "" {
			continue
		}
		inputContent = append(inputContent, map[string]interface{}{
			"type":      "input_image",
			"image_url": img,
		})
	}

	input := []map[string]interface{}{
		{
			"type":    "message",
			"role":    "user",
			"content": inputContent,
		},
	}

	req := map[string]interface{}{
		"model":        codexImagesMainModel,
		"instructions": "Generate an image based on the prompt.",
		"input":        input,
		"stream":       true,
		"store":        false,
		"tool_choice": map[string]string{
			"type": codexImageToolType,
		},
	}

	if len(toolJSON) > 0 {
		var toolObj interface{}
		if err := json.Unmarshal(toolJSON, &toolObj); err == nil {
			req["tools"] = []interface{}{toolObj}
		}
	}

	return json.Marshal(req)
}

func normalizeImageResponseFormat(format string) string {
	if strings.EqualFold(strings.TrimSpace(format), "url") {
		return "url"
	}
	return "b64_json"
}

func imageToolModel(requestModel string, routeModel string) string {
	model := strings.TrimSpace(requestModel)
	if model == "" {
		model = strings.TrimSpace(routeModel)
	}
	if model == "" {
		model = codexDefaultImageModel
	}
	return model
}

// --- Response Translation ---

// ImageStreamChunk represents a chunk in the image generation stream.
type ImageStreamChunk struct {
	B64JSON       string // partial or complete base64 data
	RevisedPrompt string // extracted from text output after image
	IsPartial     bool   // true for partial frames, false for final
	IsDone        bool   // stream is complete
	CreatedAt     int64
}

// partialImageEvent is the response.image_generation_call.partial_image event.
type partialImageEvent struct {
	Type            string `json:"type"`
	PartialImageB64 string `json:"partial_image_b64"`
	OutputFormat    string `json:"output_format"`
	ItemID          string `json:"item_id"`
	OutputIndex     int64  `json:"output_index"`
}

// ParseCodexImageStream reads a Codex SSE stream and returns a channel of image stream chunks.
// This enables real-time streaming of partial images as they are generated.
func ParseCodexImageStream(body io.Reader) <-chan ImageStreamChunk {
	ch := make(chan ImageStreamChunk, 10)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 0, 64*1024), 50*1024*1024)

		// Accumulate partial images per output index
		imageParts := make(map[int64][]string)
		var revisedPrompt strings.Builder
		var createdAt int64

		for scanner.Scan() {
			line := scanner.Bytes()
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}
			eventData := bytes.TrimSpace(line[6:])
			eventType := gjsonGet(eventData, "type")

			switch eventType {
			case "response.image_generation_call.partial_image":
				var evt partialImageEvent
				if err := json.Unmarshal(eventData, &evt); err == nil && evt.PartialImageB64 != "" {
					imageParts[evt.OutputIndex] = append(imageParts[evt.OutputIndex], evt.PartialImageB64)
					// Send partial chunk immediately
					select {
					case ch <- ImageStreamChunk{
						B64JSON:   evt.PartialImageB64,
						IsPartial: true,
					}:
					default:
					}
				}

			case "response.output_text.delta":
				// Extract revised prompt from text output after image generation
				delta := gjsonGet(eventData, "delta")
				if delta != "" {
					revisedPrompt.WriteString(delta)
				}

			case "response.completed":
				createdAt = gjsonGetInt(eventData, "response.created_at")
				if createdAt == 0 {
					createdAt = time.Now().Unix()
				}

				// Build final images from accumulated parts
				for idx := int64(0); ; idx++ {
					parts, ok := imageParts[idx]
					if !ok {
						break
					}
					var fullB64 strings.Builder
					for _, p := range parts {
						fullB64.WriteString(p)
					}
					select {
					case ch <- ImageStreamChunk{
						B64JSON:       fullB64.String(),
						RevisedPrompt: strings.TrimSpace(revisedPrompt.String()),
						IsPartial:     false,
						CreatedAt:     createdAt,
					}:
					default:
					}
				}

				// Signal stream end
				select {
				case ch <- ImageStreamChunk{IsDone: true, CreatedAt: createdAt}:
				default:
				}
				return
			}
		}

		// If stream ended without completed, send what we have
		if len(imageParts) > 0 {
			for idx := int64(0); ; idx++ {
				parts, ok := imageParts[idx]
				if !ok {
					break
				}
				var fullB64 strings.Builder
				for _, p := range parts {
					fullB64.WriteString(p)
				}
				ch <- ImageStreamChunk{
					B64JSON:       fullB64.String(),
					RevisedPrompt: strings.TrimSpace(revisedPrompt.String()),
					IsPartial:     false,
					CreatedAt:     createdAt,
				}
			}
		}
		ch <- ImageStreamChunk{IsDone: true, CreatedAt: createdAt}
	}()

	return ch
}

// ParseCodexImageResponse reads a Codex SSE stream and extracts image results (non-streaming).
func ParseCodexImageResponse(body io.Reader) ([]domain.ImageResult, int64, []byte, error) {
	result := domain.ImageResult{}
	var createdAt int64

	for chunk := range ParseCodexImageStream(body) {
		if chunk.IsDone {
			if createdAt == 0 {
				createdAt = chunk.CreatedAt
			}
			break
		}
		if !chunk.IsPartial {
			result = domain.ImageResult{
				B64JSON:       chunk.B64JSON,
				RevisedPrompt: chunk.RevisedPrompt,
			}
			if createdAt == 0 {
				createdAt = chunk.CreatedAt
			}
		}
	}

	if result.B64JSON == "" {
		return nil, 0, nil, fmt.Errorf("stream ended without image results")
	}

	return []domain.ImageResult{result}, createdAt, nil, nil
}

// --- Lightweight gjson-like helpers (avoid adding dependency) ---

func gjsonGet(data []byte, path string) string {
	// Simple path-based JSON field extraction
	parts := strings.Split(path, ".")
	var current interface{}
	if err := json.Unmarshal(data, &current); err != nil {
		return ""
	}
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = m[part]
	}
	if s, ok := current.(string); ok {
		return s
	}
	return ""
}

func gjsonGetInt(data []byte, path string) int64 {
	parts := strings.Split(path, ".")
	var current interface{}
	if err := json.Unmarshal(data, &current); err != nil {
		return 0
	}
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return 0
		}
		current = m[part]
	}
	switch v := current.(type) {
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	}
	return 0
}

func gjsonGetRaw(data []byte, path string) json.RawMessage {
	parts := strings.Split(path, ".")
	var current interface{}
	if err := json.Unmarshal(data, &current); err != nil {
		return nil
	}
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[part]
	}
	raw, _ := json.Marshal(current)
	return raw
}

// BuildOpenAIImageResponse builds an OpenAI-compatible image generations response.
func BuildOpenAIImageResponse(results []domain.ImageResult, createdAt int64, responseFormat string) ([]byte, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no image results to return")
	}

	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}
	format := normalizeImageResponseFormat(responseFormat)

	resp := domain.ImageGenerationsResponse{
		Created: createdAt,
		Data:    make([]domain.ImageResult, len(results)),
	}

	for i, img := range results {
		resp.Data[i] = domain.ImageResult{
			RevisedPrompt: img.RevisedPrompt,
		}
		if format == "url" && img.URL != "" {
			resp.Data[i].URL = img.URL
		} else if img.B64JSON != "" {
			resp.Data[i].B64JSON = img.B64JSON
		} else if img.URL != "" {
			resp.Data[i].URL = img.URL
		}
	}

	return json.Marshal(resp)
}

// BuildImageSSEStreamFrame builds an SSE frame for streaming image generation.
func BuildImageSSEStreamFrame(result domain.ImageResult, index int, final bool, responseFormat string) []byte {
	format := normalizeImageResponseFormat(responseFormat)
	frame := domain.ImageStreamFrame{
		Index: index,
	}

	if final {
		frame.Type = "complete"
		frame.Result = &result
	} else {
		frame.Type = "partial"
		if format == "b64_json" && result.B64JSON != "" {
			frame.B64JSON = result.B64JSON
		} else if result.URL != "" {
			frame.Result = &result
		}
	}

	data, err := json.Marshal(frame)
	if err != nil {
		return nil
	}

	return []byte(fmt.Sprintf("data: %s\n\n", data))
}

// --- Direct Forwarding Helpers ---

func ForwardImageGeneration(rawJSON []byte, baseURL string, apiKey string) (*http.Request, error) {
	url := strings.TrimSuffix(domain.StripVersionSuffix(baseURL), "/") + "/v1/images/generations"
	req, err := http.NewRequest("POST", url, bytes.NewReader(rawJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	return req, nil
}

func ForwardImageEdit(rawJSON []byte, baseURL string, apiKey string) (*http.Request, error) {
	url := strings.TrimSuffix(domain.StripVersionSuffix(baseURL), "/") + "/v1/images/edits"
	req, err := http.NewRequest("POST", url, bytes.NewReader(rawJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	return req, nil
}

// --- Multipart Form Helpers ---

func NeedsMultipartEdit(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(mediaType), "multipart/form-data")
}

func formValue(form *multipart.Form, key string) string {
	if form == nil || len(form.Value[key]) == 0 {
		return ""
	}
	return strings.TrimSpace(form.Value[key][0])
}

func multipartImageFiles(form *multipart.Form) []*multipart.FileHeader {
	if form == nil {
		return nil
	}
	if files := form.File["image[]"]; len(files) > 0 {
		return files
	}
	return form.File["image"]
}

func multipartFileToDataURL(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader == nil {
		return "", fmt.Errorf("upload file is nil")
	}
	f, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("open upload file failed: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("read upload file failed: %w", err)
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data)), nil
}
