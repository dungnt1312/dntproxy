package http

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gin-gonic/gin"
)

// ─── GLM (Zhipu AI) ─────────────────────────────────────────────────────────
//
// GET <base>/api/monitor/usage/quota/limit with `Authorization: Bearer <apiKey>`
// returns the coding-plan quota windows. The key is sent verbatim — the coding
// surface accepts raw API keys, so there is no client-side JWT minting.

const glmQuotaPath = "/api/monitor/usage/quota/limit"

type glmQuotaLimit struct {
	Type          string  `json:"type"`
	Unit          int     `json:"unit"`
	Number        int     `json:"number"`
	Usage         float64 `json:"usage"`
	CurrentValue  float64 `json:"currentValue"`
	Percentage    float64 `json:"percentage"`
	NextResetTime int64   `json:"nextResetTime"`
	UsageDetails  []struct {
		ModelCode string  `json:"modelCode"`
		Usage     float64 `json:"usage"`
	} `json:"usageDetails"`
}

type glmQuotaResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
	Data    struct {
		Level  string          `json:"level"`
		Limits []glmQuotaLimit `json:"limits"`
	} `json:"data"`
}

func fetchGLMUsage(conn *domain.ProviderConnection) (*UsageResponse, error) {
	apiKey := conn.APIKey
	if apiKey == "" {
		apiKey = conn.AccessToken
	}
	if apiKey == "" {
		return &UsageResponse{
			Provider: "glm",
			Message:  "No API key configured for GLM",
			Quotas:   []QuotaBucket{},
		}, nil
	}

	baseURL := glmQuotaBaseURL(conn.BaseURL)
	if err := shared.ValidateOutboundURL(baseURL, shared.AllowPrivateOutbound(conn.TenantID)); err != nil {
		return nil, fmt.Errorf("invalid GLM base URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, baseURL+glmQuotaPath, nil)
	if err != nil {
		return nil, fmt.Errorf("create GLM quota request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := shared.NewSafeHTTPClient(12 * time.Second).Do(req)
	if err != nil {
		return nil, fmt.Errorf("glm quota request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read GLM quota response: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("glm quota returned HTTP %d: API key invalid or expired", resp.StatusCode)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &UsageResponse{
			Provider: "glm",
			Message:  fmt.Sprintf("GLM quota API error (HTTP %d). Chat may still work.", resp.StatusCode),
			Quotas:   []QuotaBucket{},
		}, nil
	}

	var parsed glmQuotaResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse GLM quota response: %w", err)
	}

	result := &UsageResponse{
		Provider: "glm",
		Plan:     glmPlanName(parsed.Data.Level),
		Quotas:   []QuotaBucket{},
	}
	if parsed.Code != 0 && parsed.Code != http.StatusOK {
		result.Message = fmt.Sprintf("GLM quota API error (code %d): %s", parsed.Code, parsed.Msg)
		return result, nil
	}

	toolUsage := map[string]float64{}
	for i, limit := range parsed.Data.Limits {
		key, label := glmWindowName(limit, i)
		resetAt := ""
		if limit.NextResetTime > 0 {
			resetAt = time.UnixMilli(limit.NextResetTime).UTC().Format(time.RFC3339)
		}

		// TIME_LIMIT windows report absolute call counts; TOKENS_LIMIT only a percentage.
		if limit.Usage > 0 {
			result.addBucket(key, label, int(math.Round(limit.CurrentValue)), int(math.Round(limit.Usage)), resetAt, false)
		} else {
			result.addBucket(key, label, int(math.Round(limit.Percentage)), 100, resetAt, false)
		}

		for _, detail := range limit.UsageDetails {
			if detail.ModelCode != "" {
				toolUsage[detail.ModelCode] = detail.Usage
			}
		}
	}

	for _, bucket := range result.Quotas {
		if bucket.Pct >= 100 {
			result.LimitReached = true
			break
		}
	}
	if len(result.Quotas) == 0 {
		result.Message = "No quota data returned by GLM"
	}

	result.Extras = map[string]any{"level": parsed.Data.Level}
	if len(toolUsage) > 0 {
		result.Extras["toolUsage"] = toolUsage
	}
	return result, nil
}

// glmQuotaBaseURL normalises a connection base URL down to the API host root.
// Users often paste the full chat endpoint, which would otherwise be prefixed
// onto the monitor path.
func glmQuotaBaseURL(configured string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(configured), "/")
	if baseURL == "" {
		return "https://api.z.ai"
	}
	for _, suffix := range []string{
		"/api/coding/paas/v4/chat/completions",
		"/api/anthropic/v1/messages",
		"/api/coding/paas/v4",
		"/api/paas/v4",
		"/api/anthropic",
	} {
		if trimmed, ok := strings.CutSuffix(baseURL, suffix); ok {
			baseURL = strings.TrimRight(trimmed, "/")
			break
		}
	}
	return strings.TrimRight(domain.StripVersionSuffix(baseURL), "/")
}

// glmUnitNames maps GLM's numeric window units to names. Only units observed on
// live coding plans are mapped; anything else falls back to a positional label.
var glmUnitNames = map[int]string{
	3: "hour",
	4: "day",
	5: "month",
	6: "week",
}

var glmUnitAdjectives = map[string]string{
	"hour":  "Hourly",
	"day":   "Daily",
	"month": "Monthly",
	"week":  "Weekly",
}

// glmWindowName builds a stable key and human label for one quota window.
// The key includes the window size so multiple TOKENS_LIMIT entries (5-hour and
// weekly, for instance) do not collide.
func glmWindowName(limit glmQuotaLimit, index int) (string, string) {
	noun := "tokens"
	if limit.Type == "TIME_LIMIT" {
		noun = "tool calls"
	}

	unit, known := glmUnitNames[limit.Unit]
	if !known || limit.Number <= 0 {
		return fmt.Sprintf("glm-window-%d", index+1), fmt.Sprintf("Window %d", index+1)
	}

	window := fmt.Sprintf("%d-%s", limit.Number, unit)
	if limit.Number == 1 {
		return fmt.Sprintf("%s-%s", unit, strings.ReplaceAll(noun, " ", "-")),
			fmt.Sprintf("%s %s", glmUnitAdjectives[unit], noun)
	}
	return fmt.Sprintf("%s-%s", window, strings.ReplaceAll(noun, " ", "-")),
		fmt.Sprintf("%s %s", window, noun)
}

// glmPlanName title-cases the plan level ("pro" → "Pro").
func glmPlanName(level string) string {
	level = strings.TrimSpace(level)
	if level == "" {
		return ""
	}
	return strings.ToUpper(level[:1]) + strings.ToLower(level[1:])
}

// handleGLMQuota serves the legacy POST /connections/:id/check-quota shape by
// flattening the canonical usage response, so both endpoints share one fetcher.
func handleGLMQuota(c *gin.Context, conn *domain.ProviderConnection, result gin.H) {
	usage, err := fetchGLMUsage(conn)
	if err != nil {
		result["hasData"] = false
		result["message"] = err.Error()
		result["buckets"] = []interface{}{}
		c.JSON(200, result)
		return
	}

	buckets := make([]map[string]interface{}, 0, len(usage.Quotas))
	for _, bucket := range usage.Quotas {
		buckets = append(buckets, map[string]interface{}{
			"name":      bucket.Label,
			"used":      bucket.Used,
			"limit":     bucket.Total,
			"remaining": bucket.Remaining,
			"pctUsed":   bucket.Pct,
			"resetsAt":  bucket.ResetAt,
		})
	}

	result["hasData"] = len(buckets) > 0
	result["buckets"] = buckets
	result["planType"] = usage.Plan
	result["limitReached"] = usage.LimitReached
	if usage.Message != "" {
		result["message"] = usage.Message
	}
	c.JSON(200, result)
}
