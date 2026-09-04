package mcp

import (
	"fmt"
	"time"
)

type chatRequest struct {
	Model    string        `json:"model"`
	Stream   bool          `json:"stream"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

func userMessage(prompt string, imageDataURLs []string) chatMessage {
	parts := make([]contentPart, 0, len(imageDataURLs)+1)
	parts = append(parts, contentPart{Type: "text", Text: prompt})
	for _, u := range imageDataURLs {
		parts = append(parts, contentPart{Type: "image_url", ImageURL: &imageURL{URL: u}})
	}
	return chatMessage{Role: "user", Content: parts}
}

// timelineMessages builds one user message where each frame is preceded by a
// text label naming the video span it represents, e.g. "[02:11–04:05]".
// Interleaving lets the model attribute observations to specific parts of the
// video instead of guessing.
func timelineMessages(prompt string, frames []TimelineFrame) []chatMessage {
	parts := make([]contentPart, 0, len(frames)*2+1)
	parts = append(parts, contentPart{Type: "text", Text: prompt})
	for _, f := range frames {
		parts = append(parts, contentPart{
			Type: "text",
			Text: fmt.Sprintf("▼ Video segment %s – %s:", formatTimecode(f.Start), formatTimecode(f.End)),
		})
		parts = append(parts, contentPart{Type: "image_url", ImageURL: &imageURL{URL: f.DataURL}})
	}
	return []chatMessage{{Role: "user", Content: parts}}
}

// formatTimecode renders a duration as [h:]mm:ss.
func formatTimecode(d time.Duration) string {
	total := int(d.Seconds())
	h, rem := total/3600, total%3600
	m, s := rem/60, rem%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func defaultPrompt(kind string) string {
	if kind == "video" {
		return "You are given frames sampled from a video. Each frame is preceded by the time span of the video it represents. Describe what happens in the video as a timeline: for each segment, state what is shown and any readable on-screen text. Then summarize the overall content."
	}
	return "Describe this image in detail."
}
