package mcp

import (
	"context"
	"net/http"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/version"
)

const serverName = "dntproxy"

type analyzeImageArgs struct {
	// ImageURL is an https URL or a data:image/...;base64,... URI.
	ImageURL string `json:"image_url" jsonschema:"Image URL (https) or data:image/...;base64,... URI"`
	// Prompt is what to analyze; defaults to describing the image in detail.
	Prompt string `json:"prompt,omitempty" jsonschema:"What to analyze. Default: describe the image in detail."`
	// Model is the vision model, alias, or combo to use; defaults to the
	// server setting mcp.defaultModel.
	Model string `json:"model,omitempty" jsonschema:"Vision model to use (model id, alias or combo). Default: server setting mcp.defaultModel."`
}

type analyzeVideoArgs struct {
	// VideoURL is an https URL to a video file, max 150 MB.
	VideoURL string `json:"video_url" jsonschema:"Video URL (https), max 150 MB"`
	// Prompt is what to analyze; defaults to describing the video in detail.
	Prompt string `json:"prompt,omitempty" jsonschema:"What to analyze. Default: describe the video in detail."`
	// Model is the vision model, alias, or combo to use.
	Model string `json:"model,omitempty" jsonschema:"Vision model to use. Default: server setting mcp.defaultModel."`
	// MaxFrames is how many evenly spaced frames to extract (1-16).
	MaxFrames int `json:"max_frames,omitempty" jsonschema:"Frames to extract (1-16). Default: server setting, 8."`
}

// NewServer builds the MCP server with the analytics tools registered.
func NewServer(store port.CredentialStore, chat port.ChatService) *gomcp.Server {
	analyzer := NewAnalyzer(chat, store)
	srv := gomcp.NewServer(&gomcp.Implementation{Name: serverName, Version: version.Version}, nil)

	gomcp.AddTool[analyzeImageArgs, any](srv, &gomcp.Tool{
		Name:        "analyze_image",
		Description: "Analyze an image with a vision model and return a text description. Accepts an https URL or a data: URI.",
	}, func(ctx context.Context, _ *gomcp.CallToolRequest, args analyzeImageArgs) (*gomcp.CallToolResult, any, error) {
		if args.ImageURL == "" {
			return nil, nil, errImageURLRequired
		}
		text, err := analyzer.Analyze(ctx, "image", args.ImageURL, args.Prompt, args.Model, 0)
		if err != nil {
			return nil, nil, err
		}
		return toolText(text), nil, nil
	})

	gomcp.AddTool[analyzeVideoArgs, any](srv, &gomcp.Tool{
		Name:        "analyze_video",
		Description: "Analyze a video by extracting evenly spaced frames and sending them to a vision model. Returns a text description.",
	}, func(ctx context.Context, _ *gomcp.CallToolRequest, args analyzeVideoArgs) (*gomcp.CallToolResult, any, error) {
		if args.VideoURL == "" {
			return nil, nil, errVideoURLRequired
		}
		text, err := analyzer.Analyze(ctx, "video", args.VideoURL, args.Prompt, args.Model, args.MaxFrames)
		if err != nil {
			return nil, nil, err
		}
		return toolText(text), nil, nil
	})

	return srv
}

// NewStreamableHandler returns the HTTP handler mounted at /mcp. One server
// instance serves all requests; per-request auth identity arrives via context.
func NewStreamableHandler(store port.CredentialStore, chat port.ChatService) http.Handler {
	srv := NewServer(store, chat)
	return gomcp.NewStreamableHTTPHandler(func(*http.Request) *gomcp.Server { return srv }, nil)
}

// toolText wraps analysis text as MCP text content.
func toolText(text string) *gomcp.CallToolResult {
	return &gomcp.CallToolResult{Content: []gomcp.Content{&gomcp.TextContent{Text: text}}}
}
