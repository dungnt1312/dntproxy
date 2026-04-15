package kiro

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
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
func (e *Executor) Execute(model string, body []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
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
	req, err := http.NewRequest("POST", kiroBaseURL, bytes.NewReader(payloadBytes))
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

	reqlog.SetBodies(string(shared.TruncateBody(shared.SanitizeBody(payloadBytes), 8192)), "")

	start := time.Now()

	// Execute request using shared client (connection reuse, no stream timeout)
	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(kiroBaseURL, "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("kiro request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, errRead := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()

		respBodyStr := "Unknown error"
		if errRead == nil {
			respBodyStr = string(bodyBytes)
		}
		
		reqlog.SetBodies("", string(shared.TruncateBody(shared.SanitizeBody(bodyBytes), 8192)))
		errUpstream := fmt.Errorf("%s", respBodyStr)
		reqlog.Upstream(kiroBaseURL, "POST", resp.StatusCode, duration, errUpstream)

		return nil, resp.StatusCode, fmt.Errorf("returned %d: %s", resp.StatusCode, respBodyStr)
	}

	reqlog.Upstream(kiroBaseURL, "POST", resp.StatusCode, duration, nil)

	// Create a pipe to transform EventStream → SSE
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		defer resp.Body.Close()

		transformer := NewResponseTransformer(model)
		transformer.SetUsageCallback(func(usage UsageReport) {
			reqlog.SetUsage(usage.InputTokens, usage.OutputTokens, usage.Source)
		})
		transformer.SetPayloadCallback(func(payload PayloadReport) {
			reqlog.SetBodies("", payload.ResponsePreview)
		})
		
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
