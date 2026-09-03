package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/google/uuid"

	"github.com/dungnt/dntproxy/internal/adapter/ffmpeg"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/port"
)

// Identity carries the authenticated MCP caller into the chat pipeline.
type Identity struct {
	Policy   *port.APIKeyPolicy
	TenantID string
	APIKeyID string
}

type identityKey struct{}

// WithIdentity returns a context carrying the caller identity.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// IdentityFrom returns the identity in ctx, or nil for unauthenticated use.
func IdentityFrom(ctx context.Context) *Identity {
	id, _ := ctx.Value(identityKey{}).(*Identity)
	return id
}

// Analyzer runs one vision analysis: load media → build an OpenAI chat body →
// ChatService → aggregate SSE → text.
type Analyzer struct {
	chat  port.ChatService
	store port.CredentialStore
	// ExtractVideo extracts frames from a downloaded video file. Swap in
	// tests; production default uses ffmpeg (defaultExtractVideo).
	ExtractVideo func(ctx context.Context, videoPath string, maxFrames int) (frames []string, cleanup func(), err error)
}

// NewAnalyzer creates an Analyzer with the production ffmpeg extractor.
func NewAnalyzer(chat port.ChatService, store port.CredentialStore) *Analyzer {
	return &Analyzer{
		chat:         chat,
		store:        store,
		ExtractVideo: defaultExtractVideo,
	}
}

// noModelConfigured is the actionable error when neither the tool call nor
// settings name a vision model.
var errNoModelConfigured = fmt.Errorf("No default vision model configured — set it in Settings → MCP Analytics, or pass the model parameter")

// Analyze runs vision analysis over one media item.
// kind is "image" or "video"; maxFrames <= 0 uses the settings default.
func (a *Analyzer) Analyze(ctx context.Context, kind, mediaURL, prompt, model string, maxFrames int) (string, error) {
	settings, err := a.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("load settings: %w", err)
	}
	settings.MCP.Normalize()
	if model == "" {
		model = settings.MCP.DefaultModel
	}
	if model == "" {
		return "", errNoModelConfigured
	}
	if maxFrames <= 0 {
		maxFrames = settings.MCP.VideoMaxFrames
	}
	if prompt == "" {
		prompt = defaultPrompt(kind)
	}

	var imageParts []string
	switch kind {
	case "image":
		dataURL, err := FetchImageAsDataURL(ctx, http.DefaultClient, mediaURL)
		if err != nil {
			return "", err
		}
		imageParts = []string{dataURL}
	case "video":
		videoPath, cleanup, err := DownloadVideo(ctx, http.DefaultClient, mediaURL)
		if err != nil {
			return "", err
		}
		defer cleanup()

		frames, framesCleanup, err := a.ExtractVideo(ctx, videoPath, maxFrames)
		if framesCleanup != nil {
			defer framesCleanup()
		}
		if err != nil {
			return "", err
		}
		if len(frames) > maxFrames {
			frames = frames[:maxFrames]
		}
		imageParts = frames
	default:
		return "", fmt.Errorf("unknown media kind %q", kind)
	}

	return a.complete(ctx, model, prompt, imageParts)
}

func (a *Analyzer) complete(ctx context.Context, model, prompt string, imageParts []string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:    model,
		Stream:   true,
		Messages: []chatMessage{userMessage(prompt, imageParts)},
	})
	if err != nil {
		return "", err
	}

	requestID := "mcp-" + uuid.NewString()
	var policy *port.APIKeyPolicy
	meta := port.RequestMetadata{Context: ctx}
	if id := IdentityFrom(ctx); id != nil {
		policy = id.Policy
		meta.TenantID = id.TenantID
		meta.APIKeyID = id.APIKeyID
	}

	result := a.chat.HandleChat(body, model, requestID, policy, meta)
	if result.Error != "" {
		return "", fmt.Errorf("model %s: %s", model, result.Error)
	}
	if result.Stream == nil {
		return "", fmt.Errorf("model %s: empty response (HTTP %d)", model, result.StatusCode)
	}
	defer result.Stream.Close()

	completion, err := shared.AggregateChatCompletion(result.Stream, model, requestID)
	if err != nil {
		return "", fmt.Errorf("aggregate response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("model %s returned no choices", model)
	}
	return completion.Choices[0].Message.Content, nil
}

// defaultExtractVideo locates ffmpeg and extracts frames into a temp dir.
func defaultExtractVideo(ctx context.Context, videoPath string, maxFrames int) ([]string, func(), error) {
	ffmpegPath, ffprobePath, err := ffmpeg.Locate(ctx)
	if err != nil {
		return nil, nil, err
	}
	dir, err := os.MkdirTemp("", "dntproxy-frames-*")
	if err != nil {
		return nil, nil, err
	}
	frames, err := ffmpeg.ExtractFrames(ctx, ffmpegPath, ffprobePath, videoPath, dir, maxFrames)
	if err != nil {
		os.RemoveAll(dir)
		return nil, nil, err
	}
	return frames, func() { os.RemoveAll(dir) }, nil
}
