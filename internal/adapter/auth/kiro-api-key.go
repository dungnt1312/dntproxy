package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/kiro"
)

// KiroAPIKeyResult holds the validated state of a headless Kiro API key.
type KiroAPIKeyResult struct {
	APIKey string
	Region string
	Models []string
}

// ValidateKiroAPIKey probes a long-lived Kiro/CodeWhisperer API key against
// ListAvailableModels on the Q surface. That endpoint is the only reliable
// probe: ListAvailableProfiles rejects TokenType=API_KEY, and a plain bearer
// call there can answer 200 with an empty list for an arbitrary key.
//
// The key is used verbatim as a bearer credential — there is no token
// exchange and no refresh token, so a successful probe is the only signal
// that the key works before the first chat request.
func ValidateKiroAPIKey(apiKey, region string) (*KiroAPIKeyResult, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("apiKey is required")
	}

	// Validate what the caller actually typed before falling back, so a typo
	// surfaces as an error instead of silently querying the default region.
	if err := kiro.ValidateRegion(region); err != nil {
		return nil, err
	}
	region = kiro.NormalizeRegion(region)

	endpoint := fmt.Sprintf("https://q.%s.amazonaws.com/ListAvailableModels?origin=AI_EDITOR", region)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("TokenType", "API_KEY")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AWS-SDK-JS/3.0.0 kiro-ide/1.0.0")
	req.Header.Set("X-Amz-User-Agent", "aws-sdk-js/3.0.0 kiro-ide/1.0.0")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("validate kiro api key: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("kiro rejected the API key (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kiro model list failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	models := parseKiroModelList(body)
	if len(models) == 0 {
		return nil, fmt.Errorf("API key returned no available models")
	}

	return &KiroAPIKeyResult{APIKey: apiKey, Region: region, Models: models}, nil
}

// parseKiroModelList pulls model ids out of a ListAvailableModels response.
func parseKiroModelList(body []byte) []string {
	var payload struct {
		Models []struct {
			ModelID string `json:"modelId"`
			ID      string `json:"id"`
			Name    string `json:"modelName"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}

	seen := make(map[string]bool, len(payload.Models))
	models := make([]string, 0, len(payload.Models))
	for _, m := range payload.Models {
		id := firstNonEmpty(m.ModelID, m.ID, m.Name)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, id)
	}
	return models
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
