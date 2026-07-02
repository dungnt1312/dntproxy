package domain

// ImageGenerationsRequest represents an OpenAI-compatible image generation request.
type ImageGenerationsRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"` // "url" or "b64_json"
	Style          string `json:"style,omitempty"`
	User           string `json:"user,omitempty"`
}

// ImageEditsRequest represents an OpenAI-compatible image edit request (JSON format).
type ImageEditsRequest struct {
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt"`
	Image          string   `json:"image,omitempty"`  // data URL or file_id
	Images         []string `json:"images,omitempty"` // multiple images for inpainting
	Mask           string   `json:"mask,omitempty"`   // data URL or file_id
	N              int      `json:"n,omitempty"`
	Size           string   `json:"size,omitempty"`
	ResponseFormat string   `json:"response_format,omitempty"` // "url" or "b64_json"
	User           string   `json:"user,omitempty"`
}

// ImageResult holds a single generated/edited image.
type ImageResult struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ImageGenerationsResponse is the OpenAI-compatible image generations response.
type ImageGenerationsResponse struct {
	Created int64         `json:"created"`
	Data    []ImageResult `json:"data"`
}

// ImageStreamFrame represents a streaming frame for partial image results.
type ImageStreamFrame struct {
	Type    string       `json:"type,omitempty"`     // "partial" or "complete"
	Index   int          `json:"index,omitempty"`    // index of this image in the results
	B64JSON string       `json:"b64_json,omitempty"` // partial base64 data
	Result  *ImageResult `json:"result,omitempty"`   // complete result (for final frame)
}
