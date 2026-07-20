package gemini

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
)

// InlineImage is a reference image after it has passed through the shared safe
// image loader.
type InlineImage struct {
	Data     []byte
	MIMEType string
}

type generateContentRequest struct {
	Contents         []content        `json:"contents"`
	GenerationConfig generationConfig `json:"generationConfig"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}

type inlineData struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

type generationConfig struct {
	ResponseModalities []string       `json:"responseModalities"`
	CandidateCount     int            `json:"candidateCount,omitempty"`
	ResponseFormat     responseFormat `json:"responseFormat"`
}

type responseFormat struct {
	Image imageFormat `json:"image"`
}

type imageFormat struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageSize   string `json:"imageSize,omitempty"`
}

type openAIImageInput struct {
	Prompt string            `json:"prompt"`
	N      int               `json:"n,omitempty"`
	Size   string            `json:"size,omitempty"`
	Image  json.RawMessage   `json:"image,omitempty"`
	Images []json.RawMessage `json:"images,omitempty"`
	Mask   json.RawMessage   `json:"mask,omitempty"`
}

// EditInput contains the non-sensitive sources that still need safe loading.
type EditInput struct {
	Sources []string
}

func BuildGenerateContentRequest(rawJSON []byte, model string, images []InlineImage) ([]byte, error) {
	var input openAIImageInput
	if err := json.Unmarshal(rawJSON, &input); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if canonicalModel(model) == "" {
		return nil, errors.New("model is required")
	}

	parts := make([]part, 0, 1+len(images))
	parts = append(parts, part{Text: prompt})
	for i, image := range images {
		mimeType := strings.ToLower(strings.TrimSpace(image.MIMEType))
		if !strings.HasPrefix(mimeType, "image/") || len(image.Data) == 0 {
			return nil, fmt.Errorf("reference image %d is invalid", i+1)
		}
		parts = append(parts, part{InlineData: &inlineData{
			MIMEType: mimeType,
			Data:     base64.StdEncoding.EncodeToString(image.Data),
		}})
	}

	candidates := input.N
	if candidates < 0 {
		return nil, errors.New("n must be greater than zero")
	}
	if candidates == 0 {
		candidates = 1
	}
	aspectRatio, imageSize, err := mapOpenAIImageSize(input.Size, canonicalModel(model))
	if err != nil {
		return nil, err
	}

	native := generateContentRequest{
		Contents: []content{{Parts: parts}},
		GenerationConfig: generationConfig{
			ResponseModalities: []string{"IMAGE"},
			CandidateCount:     candidates,
			ResponseFormat: responseFormat{Image: imageFormat{
				AspectRatio: aspectRatio,
				ImageSize:   imageSize,
			}},
		},
	}
	return json.Marshal(native)
}

func ParseEditInput(rawJSON []byte) (EditInput, error) {
	var input openAIImageInput
	if err := json.Unmarshal(rawJSON, &input); err != nil {
		return EditInput{}, fmt.Errorf("invalid request JSON: %w", err)
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return EditInput{}, errors.New("prompt is required")
	}
	if hasJSONValue(input.Mask) {
		return EditInput{}, errors.New("mask editing is not supported by Gemini image models")
	}

	sources := make([]string, 0, 1+len(input.Images))
	if hasJSONValue(input.Image) {
		source, err := parseImageSource(input.Image)
		if err != nil {
			return EditInput{}, fmt.Errorf("image: %w", err)
		}
		sources = append(sources, source)
	}
	for index, raw := range input.Images {
		source, err := parseImageSource(raw)
		if err != nil {
			return EditInput{}, fmt.Errorf("images[%d]: %w", index, err)
		}
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return EditInput{}, errors.New("at least one reference image is required")
	}
	return EditInput{Sources: sources}, nil
}

func parseImageSource(raw json.RawMessage) (string, error) {
	var source string
	if err := json.Unmarshal(raw, &source); err == nil {
		source = strings.TrimSpace(source)
		if source == "" {
			return "", errors.New("image source is required")
		}
		return source, nil
	}
	var object struct {
		ImageURL string `json:"image_url"`
		URL      string `json:"url"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", errors.New("must be a URL/data URI string or an object containing image_url")
	}
	source = strings.TrimSpace(object.ImageURL)
	if source == "" {
		source = strings.TrimSpace(object.URL)
	}
	if source == "" {
		return "", errors.New("image_url is required")
	}
	return source, nil
}

func mapOpenAIImageSize(size, model string) (string, string, error) {
	size = strings.TrimSpace(strings.ToLower(size))
	if size == "" || size == "auto" {
		return "1:1", "1K", nil
	}
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return "", "", errors.New("size must use WIDTHxHEIGHT format")
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil || width <= 0 {
		return "", "", errors.New("image width must be a positive integer")
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil || height <= 0 {
		return "", "", errors.New("image height must be a positive integer")
	}

	divisor := greatestCommonDivisor(width, height)
	ratio := fmt.Sprintf("%d:%d", width/divisor, height/divisor)
	// OpenAI's historical 1792x1024 sizes are intended as widescreen and
	// portrait outputs even though their reduced ratio is 7:4, which Gemini
	// does not expose. Map these two standard values to Gemini's 16:9 pair.
	switch {
	case width == 1792 && height == 1024:
		ratio = "16:9"
	case width == 1024 && height == 1792:
		ratio = "9:16"
	}
	if !supportedGeminiAspectRatio(ratio) || (strings.Contains(model, "flash-lite-image") && !supportedGeminiFlashLiteAspectRatio(ratio)) {
		return "", "", fmt.Errorf("unsupported Gemini image aspect ratio: %s", ratio)
	}
	longest := width
	if height > longest {
		longest = height
	}
	imageSize := "1K"
	if longest > 2048 {
		imageSize = "4K"
	} else if longest > 1024 {
		imageSize = "2K"
	}
	if strings.Contains(model, "flash-lite-image") {
		imageSize = "1K"
	}
	return ratio, imageSize, nil
}

func supportedGeminiAspectRatio(value string) bool {
	switch value {
	case "1:1", "1:4", "1:8", "2:3", "3:2", "3:4", "4:1", "4:3",
		"4:5", "5:4", "8:1", "9:16", "16:9", "21:9":
		return true
	default:
		return false
	}
}

func supportedGeminiFlashLiteAspectRatio(value string) bool {
	switch value {
	case "1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9":
		return true
	default:
		return false
	}
}

func greatestCommonDivisor(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func hasJSONValue(raw json.RawMessage) bool {
	value := strings.TrimSpace(string(raw))
	return value != "" && value != "null" && value != `""`
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []responsePart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type responsePart struct {
	Thought         bool                `json:"thought"`
	InlineData      *responseInlineData `json:"inlineData"`
	InlineDataSnake *responseInlineData `json:"inline_data"`
}

type responseInlineData struct {
	MIMEType      string `json:"mimeType"`
	MIMETypeSnake string `json:"mime_type"`
	Data          string `json:"data"`
}

func ParseGenerateContentResponse(body []byte) ([]domain.ImageResult, error) {
	var response generateContentResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, errors.New("parse Gemini image response")
	}
	results := make([]domain.ImageResult, 0)
	for _, candidate := range response.Candidates {
		for _, outputPart := range candidate.Content.Parts {
			if outputPart.Thought {
				continue
			}
			data := outputPart.InlineData
			if data == nil {
				data = outputPart.InlineDataSnake
			}
			if data == nil || !strings.HasPrefix(strings.ToLower(data.mimeType()), "image/") {
				continue
			}
			encoded := strings.TrimSpace(data.Data)
			if encoded == "" {
				continue
			}
			if _, err := base64.StdEncoding.DecodeString(encoded); err != nil {
				return nil, errors.New("Gemini response contained invalid base64 image data")
			}
			results = append(results, domain.ImageResult{B64JSON: encoded})
		}
	}
	if len(results) == 0 {
		return nil, errors.New("Gemini response did not contain a final image")
	}
	return results, nil
}

func (d *responseInlineData) mimeType() string {
	if d == nil {
		return ""
	}
	if strings.TrimSpace(d.MIMEType) != "" {
		return d.MIMEType
	}
	return d.MIMETypeSnake
}
