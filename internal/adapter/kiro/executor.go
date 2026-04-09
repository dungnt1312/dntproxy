package kiro

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/google/uuid"
)

const (
	kiroBaseURL = "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse"
)

// Executor handles making requests to Kiro (AWS CodeWhisperer) API.
type Executor struct{}

// NewExecutor creates a new Kiro executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Execute sends a translated request to Kiro and returns a streaming reader
// that emits OpenAI-compatible SSE data.
func (e *Executor) Execute(model string, body []byte, credentials *domain.Credentials) (io.ReadCloser, int, error) {
	// Parse the OpenAI request
	var openaiReq OpenAIRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, 400, fmt.Errorf("parse request: %w", err)
	}

	// Translate to Kiro format
	kiroPayload, err := BuildKiroPayload(&openaiReq, model, credentials)
	if err != nil {
		return nil, 400, fmt.Errorf("build kiro payload: %w", err)
	}

	payloadBytes, err := json.Marshal(kiroPayload)
	if err != nil {
		return nil, 500, fmt.Errorf("marshal kiro payload: %w", err)
	}

	// Build HTTP request
	req, err := http.NewRequest("POST", kiroBaseURL, io.NopCloser(
		newBytesReader(payloadBytes),
	))
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.amazon.eventstream")
	req.Header.Set("X-Amz-Target", "AmazonCodeWhispererStreamingService.GenerateAssistantResponse")
	req.Header.Set("User-Agent", "AWS-SDK-JS/3.0.0 kiro-ide/1.0.0")
	req.Header.Set("X-Amz-User-Agent", "aws-sdk-js/3.0.0 kiro-ide/1.0.0")
	req.Header.Set("Amz-Sdk-Request", "attempt=1; max=3")
	req.Header.Set("Amz-Sdk-Invocation-Id", uuid.New().String())

	if credentials.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	}

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 502, fmt.Errorf("kiro request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, resp.StatusCode, fmt.Errorf("kiro returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Create a pipe to transform EventStream → SSE
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		defer resp.Body.Close()

		transformer := NewResponseTransformer(model)
		buf := make([]byte, 0, 64*1024)
		readBuf := make([]byte, 32*1024)

		for {
			n, err := resp.Body.Read(readBuf)
			if n > 0 {
				buf = append(buf, readBuf[:n]...)

				// Parse complete frames from buffer
				frames, remaining := ParseEventFrames(buf)
				buf = remaining

				for _, frame := range frames {
					sseChunks := transformer.TransformFrame(&frame)
					for _, chunk := range sseChunks {
						if _, writeErr := pw.Write(chunk); writeErr != nil {
							return
						}
					}
				}
			}

			if err != nil {
				if err != io.EOF {
					pw.CloseWithError(err)
					return
				}
				break
			}
		}

		// Flush remaining
		flushChunks := transformer.Flush()
		for _, chunk := range flushChunks {
			pw.Write(chunk)
		}
	}()

	return pr, 200, nil
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
