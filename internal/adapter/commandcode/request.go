package commandcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/google/uuid"
)

const (
	defaultBaseURL     = "https://api.commandcode.ai"
	generatePath       = "/alpha/generate"
	defaultTemperature = 0.3
	defaultMaxTokens   = 64000
	maxAllowedTokens   = 200000
)

func generateURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return baseURL + generatePath
}

func buildRequest(model string, body []byte) (ccRequestBody, error) {
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return ccRequestBody{}, fmt.Errorf("parse request body: %w", err)
	}
	if len(req.Messages) == 0 {
		return ccRequestBody{}, fmt.Errorf("messages array is required")
	}
	if model == "" {
		model = req.Model
	}
	model = MapModel(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(model, "cmc/"), "cmd/"), "commandcode/"))

	system, msgs := extractSystem(req.Messages)
	temperature := defaultTemperature
	if req.Temperature != nil {
		temperature = *req.Temperature
	}
	maxTokens := defaultMaxTokens
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	if req.MaxCompletionTokens != nil {
		maxTokens = *req.MaxCompletionTokens
	}
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	if maxTokens > maxAllowedTokens {
		maxTokens = maxAllowedTokens
	}

	return ccRequestBody{
		Config: ccConfig{
			WorkingDir:    ".",
			Date:          time.Now().Format("2006-01-02"),
			Environment:   "cli",
			Structure:     []string{},
			IsGitRepo:     false,
			CurrentBranch: "",
			MainBranch:    "main",
			GitStatus:     "",
			RecentCommits: []string{},
		},
		Memory: "",
		Taste:  "",
		Skills: "",
		Params: ccChatParams{
			Model:       model,
			Messages:    convertMessages(msgs),
			Tools:       convertTools(req.Tools),
			System:      system,
			MaxTokens:   maxTokens,
			Temperature: temperature,
			Stream:      true,
		},
		ThreadID: uuid.NewString(),
	}, nil
}

func newUpstreamRequest(ctx context.Context, baseURL, apiKey string, ccBody ccRequestBody) (*http.Request, []byte, error) {
	payload, err := json.Marshal(ccBody)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal commandcode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, generateURL(baseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("create commandcode request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("x-command-code-version", commandCodeVersion())
	req.Header.Set("x-cli-environment", "production")
	return req, payload, nil
}

func apiKeyFromCredentials(credentials *domain.Credentials) string {
	if credentials == nil {
		return ""
	}
	if credentials.APIKey != "" {
		return credentials.APIKey
	}
	return credentials.AccessToken
}
