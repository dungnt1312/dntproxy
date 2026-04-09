package openai

import (
	"fmt"
	"io"
	"net/http"

	"github.com/dungnt/dntproxy/internal/domain"
)

const defaultOpenAIBaseURL = "https://api.openai.com"

// Executor handles making requests to OpenAI or OpenAI-compatible APIs.
// Since these APIs are already OpenAI-compatible, we just proxy the request
// with the appropriate auth header — no request/response translation needed.
type Executor struct{}

// NewExecutor creates a new OpenAI executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Execute sends a request to OpenAI (or compatible) API and returns a streaming reader.
func (e *Executor) Execute(model string, body []byte, credentials *domain.Credentials) (io.ReadCloser, int, error) {
	baseURL := credentials.BaseURL
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}

	url := baseURL + "/v1/chat/completions"

	req, err := http.NewRequest("POST", url, io.NopCloser(newBytesReader(body)))
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Auth: API key takes priority
	apiKey := credentials.APIKey
	if apiKey == "" {
		apiKey = credentials.AccessToken
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 502, fmt.Errorf("openai request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, resp.StatusCode, fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// OpenAI-compatible responses are already SSE formatted, pass through directly
	return resp.Body, 200, nil
}

// bytesReader wraps a byte slice as an io.Reader.
type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
