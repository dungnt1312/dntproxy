package mcp

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

func defaultPrompt(kind string) string {
	if kind == "video" {
		return "Describe this video in detail."
	}
	return "Describe this image in detail."
}
