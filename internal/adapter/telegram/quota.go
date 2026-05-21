package telegram

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

type quotaUsage struct {
	Provider     string
	Plan         string
	LimitReached bool
	Message      string
	Quotas       []quotaBucket
}

type quotaBucket struct {
	Label     string
	Used      int
	Total     int
	Remaining int
	Pct       int
	ResetAt   string
}

func fetchQuotaUsage(conn *domain.ProviderConnection) (*quotaUsage, error) {
	switch conn.Provider {
	case "kiro":
		return fetchKiroQuotaUsage(conn)
	case "minimax":
		return fetchMiniMaxQuotaUsage(conn)
	case "openai":
		return fetchOpenAIQuotaUsage(conn)
	default:
		return nil, nil
	}
}

func fetchKiroQuotaUsage(conn *domain.ProviderConnection) (*quotaUsage, error) {
	usage := &quotaUsage{Provider: "kiro"}
	profileArn := ""
	if conn.ProviderSpecificData != nil {
		profileArn, _ = conn.ProviderSpecificData["profileArn"].(string)
	}

	client := &http.Client{Timeout: 8 * time.Second}
	var resp *http.Response
	var err error
	if profileArn == "" {
		req, _ := http.NewRequest("GET", "https://q.us-east-1.amazonaws.com/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST", nil)
		req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
		req.Header.Set("Accept", "application/json")
		resp, err = client.Do(req)
	} else {
		payload, _ := json.Marshal(map[string]string{"origin": "AI_EDITOR", "profileArn": profileArn, "resourceType": "AGENTIC_REQUEST"})
		req, _ := http.NewRequest("POST", "https://codewhisperer.us-east-1.amazonaws.com", bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
		req.Header.Set("Content-Type", "application/x-amz-json-1.0")
		req.Header.Set("x-amz-target", "AmazonCodeWhispererService.GetUsageLimits")
		req.Header.Set("Accept", "application/json")
		resp, err = client.Do(req)
	}
	if err != nil {
		return nil, fmt.Errorf("quota unavailable")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		usage.Message = "quota auth failed; token may need refresh"
		return usage, nil
	}
	if resp.StatusCode != 200 {
		usage.Message = fmt.Sprintf("quota HTTP %d", resp.StatusCode)
		return usage, nil
	}
	parseKiroQuotaBody(body, usage)
	return usage, nil
}

func parseKiroQuotaBody(body []byte, usage *quotaUsage) {
	var data map[string]interface{}
	if json.Unmarshal(body, &data) != nil {
		return
	}
	if sub, ok := data["subscriptionInfo"].(map[string]interface{}); ok {
		usage.Plan, _ = sub["subscriptionTitle"].(string)
	}
	resetAt := parseQuotaResetTime(data["nextDateReset"])
	if resetAt == "" {
		resetAt = parseQuotaResetTime(data["resetDate"])
	}
	list, _ := data["usageBreakdownList"].([]interface{})
	for _, item := range list {
		bucket, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		resType, _ := bucket["resourceType"].(string)
		if resType == "" {
			resType, _ = bucket["displayName"].(string)
		}
		used, _ := bucket["currentUsageWithPrecision"].(float64)
		total, _ := bucket["usageLimitWithPrecision"].(float64)
		switch {
		case strings.EqualFold(resType, "AGENTIC_REQUEST") || strings.EqualFold(resType, "CREDIT"):
			usage.addBucket("Requests", int(used), int(total), resetAt)
		case strings.EqualFold(resType, "INLINE_INVOCATION") || strings.EqualFold(resType, "CHAT_REQUEST"):
			usage.addBucket("Tokens", int(used), int(total), resetAt)
		}
	}
}

func fetchMiniMaxQuotaUsage(conn *domain.ProviderConnection) (*quotaUsage, error) {
	if conn.APIKey == "" {
		return &quotaUsage{Provider: "minimax", Message: "no API key configured"}, nil
	}
	req, _ := http.NewRequest("GET", "https://api.minimax.io/v1/api/openplatform/coding_plan/remains", nil)
	req.Header.Set("Authorization", "Bearer "+conn.APIKey)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("quota unavailable")
	}
	defer resp.Body.Close()
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err == nil {
			defer gr.Close()
			reader = gr
		}
	}
	body, _ := io.ReadAll(reader)
	usage := &quotaUsage{Provider: "minimax"}
	if len(body) > 0 && body[0] == '<' {
		usage.Message = "quota blocked by Cloudflare"
		return usage, nil
	}
	parseMiniMaxQuotaBody(body, usage)
	return usage, nil
}

func parseMiniMaxQuotaBody(body []byte, usage *quotaUsage) {
	var data map[string]interface{}
	if json.Unmarshal(body, &data) != nil {
		usage.Message = "quota parse failed"
		return
	}
	if code, ok := data["code"].(float64); ok && code != 0 {
		msg, _ := data["msg"].(string)
		usage.Message = msg
		return
	}
	models, _ := data["model_remains"].([]interface{})
	for _, item := range models {
		model, ok := item.(map[string]interface{})
		if !ok || model["model_name"] != "MiniMax-M*" {
			continue
		}
		intervalTotal, _ := model["current_interval_total_count"].(float64)
		intervalRemaining, _ := model["current_interval_usage_count"].(float64)
		if intervalTotal > 0 {
			resetAt := ""
			if ts, _ := model["end_time"].(float64); ts > 0 {
				resetAt = time.UnixMilli(int64(ts)).UTC().Format(time.RFC3339)
			}
			usage.addBucket("MiniMax-M* Interval", int(intervalTotal-intervalRemaining), int(intervalTotal), resetAt)
		}
		weeklyTotal, _ := model["current_weekly_total_count"].(float64)
		weeklyRemaining, _ := model["current_weekly_usage_count"].(float64)
		if weeklyTotal > 0 {
			resetAt := ""
			if ts, _ := model["weekly_end_time"].(float64); ts > 0 {
				resetAt = time.UnixMilli(int64(ts)).UTC().Format(time.RFC3339)
			}
			usage.addBucket("MiniMax-M* Weekly", int(weeklyTotal-weeklyRemaining), int(weeklyTotal), resetAt)
		}
	}
}

func fetchOpenAIQuotaUsage(conn *domain.ProviderConnection) (*quotaUsage, error) {
	if conn.AuthType == "oauth" {
		return &quotaUsage{Provider: "openai", Message: "Codex quota check is available in dashboard; bot skips it to avoid consuming a probe request"}, nil
	}
	baseURL := conn.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	req, _ := http.NewRequest("GET", domain.StripVersionSuffix(baseURL)+"/v1/models", nil)
	if conn.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+conn.APIKey)
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("quota unavailable")
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	usage := &quotaUsage{Provider: "openai"}
	requestsLimit := parseQuotaHeader(resp.Header.Get("x-ratelimit-limit-requests"))
	requestsRemaining := parseQuotaHeader(resp.Header.Get("x-ratelimit-remaining-requests"))
	if requestsLimit >= 0 {
		usage.addBucket("Requests", requestsLimit-requestsRemaining, requestsLimit, resp.Header.Get("x-ratelimit-reset-requests"))
	}
	tokensLimit := parseQuotaHeader(resp.Header.Get("x-ratelimit-limit-tokens"))
	tokensRemaining := parseQuotaHeader(resp.Header.Get("x-ratelimit-remaining-tokens"))
	if tokensLimit >= 0 {
		usage.addBucket("Tokens", tokensLimit-tokensRemaining, tokensLimit, resp.Header.Get("x-ratelimit-reset-tokens"))
	}
	if len(usage.Quotas) == 0 {
		usage.Message = "upstream returned no rate-limit headers"
	}
	return usage, nil
}

func (u *quotaUsage) addBucket(label string, used int, total int, resetAt string) {
	remaining := total - used
	if remaining < 0 {
		remaining = 0
	}
	pct := 0
	if total > 0 {
		pct = used * 100 / total
		if pct > 100 {
			pct = 100
		}
	}
	u.Quotas = append(u.Quotas, quotaBucket{Label: label, Used: used, Total: total, Remaining: remaining, Pct: pct, ResetAt: resetAt})
}

func parseQuotaResetTime(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		if v > 0 {
			return time.Unix(int64(v), 0).UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func parseQuotaHeader(value string) int {
	if value == "" {
		return -1
	}
	n := 0
	if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
		return -1
	}
	return n
}
