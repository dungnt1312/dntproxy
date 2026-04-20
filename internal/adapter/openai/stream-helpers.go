package openai

import (
	"encoding/json"
	"io"
	"strings"
	"sync"
)

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func extractUsage(body []byte) *openAIUsage {
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}
		var chunk struct {
			Usage *openAIUsage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err == nil && chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
			return chunk.Usage
		}
	}
	return nil
}

func extractResponsePreview(body []byte) (string, bool) {
	var builder strings.Builder
	const maxPreviewBytes = 4000
	appendContent := func(content string) bool {
		for _, r := range content {
			runeBytes := len(string(r))
			if builder.Len()+runeBytes > maxPreviewBytes {
				return true
			}
			builder.WriteRune(r)
		}
		return false
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			content := choice.Delta.Content
			if content == "" {
				content = choice.Message.Content
			}
			if content == "" {
				continue
			}
			if appendContent(content) {
				return strings.TrimSpace(builder.String()), true
			}
		}
	}

	if preview := strings.TrimSpace(builder.String()); preview != "" {
		return preview, false
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		OutputText string `json:"output_text"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", false
	}
	if response.OutputText != "" && appendContent(response.OutputText) {
		return strings.TrimSpace(builder.String()), true
	}
	for _, choice := range response.Choices {
		content := choice.Message.Content
		if content == "" {
			content = choice.Text
		}
		if appendContent(content) {
			return strings.TrimSpace(builder.String()), true
		}
	}
	return strings.TrimSpace(builder.String()), false
}

// openaiStreamSniffer wraps an io.ReadCloser to capture the stream body
// for usage extraction and response preview after the stream ends.
type openaiStreamSniffer struct {
	io.ReadCloser
	mu      sync.Mutex
	bodyBuf []byte
	onClose func([]byte)
}

func (s *openaiStreamSniffer) Read(p []byte) (n int, err error) {
	n, err = s.ReadCloser.Read(p)
	if n > 0 {
		s.mu.Lock()
		// keep up to 100KB to avoid memory explosion if stream is huge
		if len(s.bodyBuf) < 100*1024 {
			s.bodyBuf = append(s.bodyBuf, p[:n]...)
		}
		s.mu.Unlock()
	}
	return n, err
}

func (s *openaiStreamSniffer) Close() error {
	err := s.ReadCloser.Close()
	s.mu.Lock()
	buf := s.bodyBuf
	s.mu.Unlock()
	if s.onClose != nil && len(buf) > 0 {
		s.onClose(buf)
	}
	return err
}
