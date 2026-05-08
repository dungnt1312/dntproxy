package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// Executor handles making requests to OpenAI or OpenAI-compatible APIs.
// Since these APIs are already OpenAI-compatible, we just proxy the request
// with the appropriate auth header — no request/response translation needed.
type Executor struct{}

// NewExecutor creates a new OpenAI executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// resolveBaseURL returns the appropriate base URL for a provider.
// Uses credentials.BaseURL if set, otherwise falls back to provider config.
// Strips version suffix to avoid double version in the final URL.
func resolveBaseURL(credentials *domain.Credentials) string {
	if credentials.BaseURL != "" {
		return domain.StripVersionSuffix(credentials.BaseURL)
	}

	cfg := domain.GetProviderConfig(credentials.Provider)
	return domain.StripVersionSuffix(cfg.DefaultBaseURL)
}

// resolveChatPath returns the correct API path for chat completions.
// Delegates to the provider config registry.
func resolveChatPath(credentials *domain.Credentials) string {
	cfg := domain.GetProviderConfig(credentials.Provider)
	return cfg.ChatPath
}

// isCodexOAuth returns true if this credential is an OpenAI OAuth token
// (from auth.openai.com) which needs to use the Codex Responses API.
func isCodexOAuth(credentials *domain.Credentials) bool {
	// If there's an API key, use standard API (not OAuth)
	if credentials.APIKey != "" {
		return false
	}
	// If there's a custom base URL, user configured their own endpoint
	if credentials.BaseURL != "" {
		return false
	}
	// Check provider-specific data for authMethod == "oauth"
	if credentials.ProviderSpecificData != nil {
		if method, ok := credentials.ProviderSpecificData["authMethod"].(string); ok {
			return method == "oauth"
		}
	}
	// If we have an access token but no API key, it's likely OAuth
	return credentials.AccessToken != "" && credentials.APIKey == ""
}

// Execute sends a request to OpenAI (or compatible) API and returns a streaming reader.
func (e *Executor) Execute(model string, body []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
	if isCodexOAuth(credentials) {
		return e.executeCodexResponses(model, body, credentials, reqlog)
	}
	return e.executeStandard(model, body, credentials, reqlog)
}

func applyModelPrefix(body []byte, prefix string) ([]byte, error) {
	if prefix == "" {
		return body, nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse request body: %w", err)
	}

	model, ok := payload["model"].(string)
	if ok && strings.HasPrefix(model, prefix) {
		payload["model"] = strings.TrimPrefix(model, prefix)
	}

	rewritten, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("rewrite request body: %w", err)
	}
	return rewritten, nil
}

// executeStandard handles standard OpenAI API key requests (api.openai.com/v1/chat/completions).
func (e *Executor) executeStandard(model string, body []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
	baseURL := resolveBaseURL(credentials)
	chatPath := resolveChatPath(credentials)
	url := baseURL + chatPath
	forwardBody, err := applyModelPrefix(body, credentials.ModelPrefix)
	if err != nil {
		return nil, 400, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(forwardBody))
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

	reqlog.SetBodies(shared.PrepareLoggedBody(forwardBody), "")

	start := time.Now()

	// Execute request using shared client (connection reuse, no stream timeout)
	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(url, "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("openai request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, errRead := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()

		respBodyStr := "Unknown error"
		if errRead == nil {
			respBodyStr = string(bodyBytes)
		}

		reqlog.SetBodies("", shared.PrepareLoggedBody(bodyBytes))
		errUpstream := fmt.Errorf("%s", respBodyStr)
		reqlog.Upstream(url, "POST", resp.StatusCode, duration, errUpstream)

		return nil, resp.StatusCode, fmt.Errorf("returned %d: %s", resp.StatusCode, respBodyStr)
	}

	reqlog.Upstream(url, "POST", resp.StatusCode, duration, nil)

	// Sniff stream to extract usage and preview at the end
	sniffer := &openaiStreamSniffer{
		ReadCloser: resp.Body,
		onClose: func(bodyBytes []byte) {
			if usage := extractUsage(bodyBytes); usage != nil {
				reqlog.SetUsage(usage.PromptTokens, usage.CompletionTokens, "sse_usage")
			}
			if preview, _ := extractResponsePreview(bodyBytes); preview != "" {
				reqlog.SetBodies("", preview)
			}
		},
	}

	return sniffer, 200, nil
}

// executeCodexResponses handles OpenAI OAuth tokens via the Codex Responses API.
// Translates: Chat Completions request → Codex Responses API request
// And:        Codex Responses API SSE → Chat Completions SSE
func (e *Executor) executeCodexResponses(model string, body []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
	// Translate request: Chat Completions → Codex Responses API
	translatedBody, err := TranslateChatToCodexResponses(body)
	if err != nil {
		return nil, 500, fmt.Errorf("translate request to codex format: %w", err)
	}

	url := codexResponsesURL

	req, err := http.NewRequest("POST", url, bytes.NewReader(translatedBody))
	if err != nil {
		return nil, 500, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	// Codex CLI headers
	req.Header.Set("originator", "codex-cli")
	req.Header.Set("User-Agent", "codex-cli/1.0.18 (macOS; arm64)")
	// Random session_id per request (matches real Codex CLI behavior)
	req.Header.Set("session_id", fmt.Sprintf("%d-%s", time.Now().UnixMilli(), randomAlphaNum(9)))

	reqlog.SetBodies(shared.PrepareLoggedBody(translatedBody), "")

	start := time.Now()

	// Execute request using shared client (connection reuse, no stream timeout)
	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		reqlog.Upstream(url, "POST", 502, duration, err)
		return nil, 502, fmt.Errorf("codex request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, errRead := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()

		respBodyStr := "Unknown error"
		if errRead == nil {
			respBodyStr = string(bodyBytes)
		}

		reqlog.SetBodies("", shared.PrepareLoggedBody(bodyBytes))
		errUpstream := fmt.Errorf("%s", respBodyStr)
		reqlog.Upstream(url, "POST", resp.StatusCode, duration, errUpstream)
		return nil, resp.StatusCode, fmt.Errorf("codex returned %d: %s", resp.StatusCode, respBodyStr)
	}

	reqlog.Upstream(url, "POST", resp.StatusCode, duration, nil)

	// Create a pipe to transform the Codex SSE stream into Chat Completions SSE
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer resp.Body.Close()
		state := NewCodexResponseState(model)
		scanner := bufio.NewScanner(resp.Body)
		// Increase scanner buffer for large SSE events
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		currentEvent := ""

		var completePayloadBuilder strings.Builder

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				dataStr := strings.TrimPrefix(line, "data: ")
				if currentEvent != "" {
					translated := TranslateCodexEvent(currentEvent, []byte(dataStr), state)
					if translated != "" {
						pw.Write([]byte(translated))
						completePayloadBuilder.WriteString(translated)
					}
					currentEvent = ""
				}
				continue
			}

			if line == "" {
				currentEvent = ""
			}
		}

		// If stream ended without response.completed, send finish
		if !state.FinishReasonSent {
			finishReason := "stop"
			if state.ToolCallIndex > 0 {
				finishReason = "tool_calls"
			}
			translated := formatSSEChunk(state, map[string]interface{}{}, &finishReason)
			pw.Write([]byte(translated))
			completePayloadBuilder.WriteString(translated)
			pw.Write([]byte("data: [DONE]\n\n"))
		}

		// Set usage metrics
		reqlog.SetUsage(state.PromptTokens, state.CompletionTokens, "codex_metrics")
		if preview, _ := extractResponsePreview([]byte(completePayloadBuilder.String())); preview != "" {
			reqlog.SetBodies("", preview)
		}
	}()

	return pr, 200, nil
}
