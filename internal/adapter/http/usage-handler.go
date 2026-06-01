package http

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// ─── Unified response types ────────────────────────────────────────────────

// QuotaBucket is one named quota slot (e.g. "session", "requests", "free trial").
type QuotaBucket struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Used      int    `json:"used"`
	Total     int    `json:"total"`
	Remaining int    `json:"remaining"`
	Pct       int    `json:"pct"` // 0-100, percent USED
	ResetAt   string `json:"resetAt,omitempty"`
	Unlimited bool   `json:"unlimited"`
}

// UsageResponse is the single, canonical response shape for GET /api/usage/:id.
type UsageResponse struct {
	Provider     string        `json:"provider"`
	Plan         string        `json:"plan,omitempty"`
	LimitReached bool          `json:"limitReached"`
	Message      string        `json:"message,omitempty"`
	Quotas       []QuotaBucket `json:"quotas"`
	Overages     *OverageInfo  `json:"overages,omitempty"`
}

// OverageInfo contains overage usage when user has exceeded their plan limits.
type OverageInfo struct {
	Used      float64 `json:"used"`
	Cap       float64 `json:"cap"`
	Remaining float64 `json:"remaining"`
	Status    string  `json:"status,omitempty"`
	Charge    float64 `json:"charge,omitempty"`
	Rate      float64 `json:"rate,omitempty"`
}

func (r *UsageResponse) addBucket(key, label string, used, total int, resetAt string, unlimited bool) {
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
	r.Quotas = append(r.Quotas, QuotaBucket{
		Key: key, Label: label,
		Used: used, Total: total, Remaining: remaining,
		Pct: pct, ResetAt: resetAt, Unlimited: unlimited,
	})
}

// ─── Handler ───────────────────────────────────────────────────────────────

// UsageHandler handles usage/quota API requests.
type UsageHandler struct {
	store port.CredentialStore
}

// NewUsageHandler creates a new UsageHandler.
func NewUsageHandler(store port.CredentialStore) *UsageHandler {
	return &UsageHandler{store: store}
}

// GetUsage handles GET /api/usage/:connectionId
func (h *UsageHandler) GetUsage(c *gin.Context) {
	connectionID := c.Param("connectionId")

	cfg, err := h.store.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load config"})
		return
	}

	var conn *domain.ProviderConnection
	for i := range cfg.ProviderConnections {
		if cfg.ProviderConnections[i].ID == connectionID {
			conn = &cfg.ProviderConnections[i]
			break
		}
	}
	if conn == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connection not found"})
		return
	}

	// Non-OAuth connections without API key have no usage API.
	// API key connections (MiniMax, GLM, etc.) can still have quota endpoints.
	if conn.AuthType != "oauth" && conn.APIKey == "" {
		c.JSON(http.StatusOK, UsageResponse{
			Provider: conn.Provider,
			Message:  "Usage not available for this connection",
			Quotas:   []QuotaBucket{},
		})
		return
	}

	// Refresh token proactively if needed.
	refreshSvc := auth.NewTokenRefreshService(h.store)
	if refreshSvc.NeedsRefresh(conn) {
		log.Printf("[USAGE] Token expiring soon for %s, refreshing...", conn.Name)
		if refreshed, err := refreshSvc.Refresh(conn); err == nil {
			conn = refreshed
			h.store.UpdateConnection(conn)
		}
	}

	resp, err := h.fetchUsage(conn)
	log.Printf("[USAGE] Fetched usage for %s: %+v (err=%v)", conn.Name, resp, err)
	if err != nil {
		// Try a forced refresh on auth errors.
		if isAuthExpiredError(err) && conn.RefreshToken != "" {
			log.Printf("[USAGE] Auth expired, force refreshing for %s", conn.Name)
			if refreshed, rerr := refreshSvc.Refresh(conn); rerr == nil {
				conn = refreshed
				h.store.UpdateConnection(conn)
				resp, err = h.fetchUsage(conn)
			}
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, resp)
}

// fetchUsage dispatches to the provider-specific fetcher.
func (h *UsageHandler) fetchUsage(conn *domain.ProviderConnection) (*UsageResponse, error) {
	switch conn.Provider {
	case "openai":
		return fetchOpenAIUsage(conn)
	case "kiro":
		return fetchKiroUsage(conn)
	case "minimax":
		return fetchMiniMaxUsage(conn)
	case "xai":
		return &UsageResponse{
			Provider: "xai",
			Message:  "xAI does not expose live quota for Grok Build OAuth; local usage appears in logs after successful requests.",
			Quotas:   []QuotaBucket{},
		}, nil
	default:
		return &UsageResponse{
			Provider: conn.Provider,
			Message:  fmt.Sprintf("Usage API not implemented for %s", conn.Provider),
			Quotas:   []QuotaBucket{},
		}, nil
	}
}

// ─── OpenAI ────────────────────────────────────────────────────────────────

func fetchOpenAIUsage(conn *domain.ProviderConnection) (*UsageResponse, error) {
	raw, err := auth.GetOpenAIUsage(conn.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("openai usage API: %w", err)
	}

	rateLimit, _ := raw["rate_limit"].(map[string]interface{})
	primary, _ := rateLimit["primary_window"].(map[string]interface{})
	secondary, _ := rateLimit["secondary_window"].(map[string]interface{})

	planType, _ := raw["plan_type"].(string)
	limitReached, _ := rateLimit["limit_reached"].(bool)

	sessionUsed, _ := primary["used_percent"].(float64)
	weeklyUsed, _ := secondary["used_percent"].(float64)
	sessionResetAt := parseResetTime(primary["reset_at"])
	weeklyResetAt := parseResetTime(secondary["reset_at"])

	resp := &UsageResponse{
		Provider:     "openai",
		Plan:         planType,
		LimitReached: limitReached,
		Quotas:       []QuotaBucket{},
	}
	resp.addBucket("session", "Session", int(sessionUsed), 100, sessionResetAt, false)
	resp.addBucket("weekly", "Weekly", int(weeklyUsed), 100, weeklyResetAt, false)
	return resp, nil
}

// ─── Kiro ──────────────────────────────────────────────────────────────────

// fetchKiroUsage mirrors the proven handleKiroQuota logic from quota-handler.go.
func fetchKiroUsage(conn *domain.ProviderConnection) (*UsageResponse, error) {
	resp := &UsageResponse{
		Provider: "kiro",
		Quotas:   []QuotaBucket{},
	}

	profileArn := ""
	if conn.ProviderSpecificData != nil {
		if v, ok := conn.ProviderSpecificData["profileArn"].(string); ok {
			profileArn = v
		}
	}

	client := &http.Client{Timeout: 12 * time.Second}
	var httpResp *http.Response
	var err error

	if profileArn == "" {
		// Builder-ID path: GET endpoint
		req, _ := http.NewRequest("GET",
			"https://q.us-east-1.amazonaws.com/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST", nil)
		req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
		req.Header.Set("Accept", "application/json")
		httpResp, err = client.Do(req)
	} else {
		// IDC/imported path: POST to CodeWhisperer, fallback to Q
		payload, _ := json.Marshal(map[string]string{
			"origin": "AI_EDITOR", "profileArn": profileArn, "resourceType": "AGENTIC_REQUEST",
		})
		req, _ := http.NewRequest("POST", "https://codewhisperer.us-east-1.amazonaws.com",
			bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
		req.Header.Set("Content-Type", "application/x-amz-json-1.0")
		req.Header.Set("x-amz-target", "AmazonCodeWhispererService.GetUsageLimits")
		req.Header.Set("Accept", "application/json")
		httpResp, err = client.Do(req)
		// Fallback to Q endpoint if CodeWhisperer returned non-200/401/403
		if err == nil && httpResp.StatusCode != 200 &&
			httpResp.StatusCode != 401 && httpResp.StatusCode != 403 {
			httpResp.Body.Close()
			qURL := fmt.Sprintf(
				"https://q.us-east-1.amazonaws.com/getUsageLimits?origin=AI_EDITOR&profileArn=%s&resourceType=AGENTIC_REQUEST",
				profileArn)
			req2, _ := http.NewRequest("GET", qURL, nil)
			req2.Header.Set("Authorization", "Bearer "+conn.AccessToken)
			req2.Header.Set("Accept", "application/json")
			httpResp, err = client.Do(req2)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("kiro quota request: %w", err)
	}
	defer httpResp.Body.Close()
	body, _ := io.ReadAll(httpResp.Body)

	if httpResp.StatusCode == 401 || httpResp.StatusCode == 403 {
		resp.Message = "Kiro quota API authentication expired. Chat may still work."
		return resp, nil
	}
	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("kiro API error: HTTP %d: %s", httpResp.StatusCode, string(body))
	}

	parseKiroUsageBody(body, resp)
	return resp, nil
}

// parseKiroUsageBody fills UsageResponse from the CodeWhisperer GetUsageLimits JSON body
// using the same field mapping as parseKiroQuotaResponse in quota-handler.go.
func parseKiroUsageBody(body []byte, resp *UsageResponse) {
	var data map[string]interface{}
	if json.Unmarshal(body, &data) != nil {
		return
	}

	// Subscription plan name
	if sub, ok := data["subscriptionInfo"].(map[string]interface{}); ok {
		if title, ok := sub["subscriptionTitle"].(string); ok {
			resp.Plan = title
		}
	}

	// Reset date (shared across all buckets)
	resetAt := parseResetTime(data["nextDateReset"])
	if resetAt == "" {
		resetAt = parseResetTime(data["resetDate"])
	}

	list, _ := data["usageBreakdownList"].([]interface{})
	for _, item := range list {
		b, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		resType, _ := b["resourceType"].(string)
		if resType == "" {
			resType, _ = b["displayName"].(string)
		}
		used, _ := b["currentUsageWithPrecision"].(float64)
		total, _ := b["usageLimitWithPrecision"].(float64)

		// Parse overage data from first bucket (CREDIT/AGENTIC_REQUEST)
		if resp.Overages == nil {
			overageUsed, _ := b["currentOveragesWithPrecision"].(float64)
			overageCap, _ := b["overageCapWithPrecision"].(float64)
			overageCharge, _ := b["overageCharges"].(float64)
			overageRate, _ := b["overageRate"].(float64)

			if overageUsed > 0 {
				// Get overage status from overageConfiguration
				status := ""
				if overageCfg, ok := data["overageConfiguration"].(map[string]interface{}); ok {
					if s, ok := overageCfg["overageStatus"].(string); ok {
						status = s
					}
				}

				resp.Overages = &OverageInfo{
					Used:      overageUsed,
					Cap:       overageCap,
					Remaining: overageCap - overageUsed,
					Status:    status,
					Charge:    overageCharge,
					Rate:      overageRate,
				}
			}
		}

		if strings.EqualFold(resType, "AGENTIC_REQUEST") || strings.EqualFold(resType, "CREDIT") {
			resp.addBucket("requests", "Requests", int(used), int(total), resetAt, false)

			// Free trial sub-bucket
			if ft, ok := b["freeTrialInfo"].(map[string]interface{}); ok {
				if status, _ := ft["freeTrialStatus"].(string); status == "ACTIVE" {
					ftUsed, _ := ft["currentUsageWithPrecision"].(float64)
					ftTotal, _ := ft["usageLimitWithPrecision"].(float64)
					ftExpiry := ""
					if ts, _ := ft["freeTrialExpiry"].(float64); ts > 0 {
						ftExpiry = time.Unix(int64(ts), 0).UTC().Format(time.RFC3339)
					}
					resp.addBucket("free-trial", "Free Trial", int(ftUsed), int(ftTotal), ftExpiry, false)
				}
			}

		} else if strings.EqualFold(resType, "INLINE_INVOCATION") ||
			strings.EqualFold(resType, "CHAT_REQUEST") {
			resp.addBucket("tokens", "Tokens", int(used), int(total), resetAt, false)
		}
	}
}

// ─── MiniMax ────────────────────────────────────────────────────────────────

// fetchMiniMaxUsage calls MiniMax's coding_plan/remains API.
// Uses GET method with browser-like headers to bypass Cloudflare.
func fetchMiniMaxUsage(conn *domain.ProviderConnection) (*UsageResponse, error) {
	apiKey := conn.APIKey
	if apiKey == "" {
		return &UsageResponse{
			Provider: "minimax",
			Message:  "No API key configured",
			Quotas:   []QuotaBucket{},
		}, nil
	}

	// Use GET method with browser-like headers (matches test.go)
	quotaURL := "https://api.minimax.io/v1/api/openplatform/coding_plan/remains"

	req, err := http.NewRequest("GET", quotaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("minimax quota request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "none")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[MiniMax Quota] Request error: %v", err)
		return &UsageResponse{
			Provider: "minimax",
			Message:  "Quota API unavailable. Chat may still work.",
			Quotas:   []QuotaBucket{},
		}, nil
	}
	defer resp.Body.Close()

	// Handle gzip encoding
	var bodyReader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("minimax gzip parse: %w", err)
		}
		defer gr.Close()
		bodyReader = gr
	}

	bodyBytes, _ := io.ReadAll(bodyReader)

	result := &UsageResponse{
		Provider: "minimax",
		Quotas:   []QuotaBucket{},
	}

	// Check if response is HTML (Cloudflare block)
	if len(bodyBytes) > 0 && bodyBytes[0] == '<' {
		log.Printf("[MiniMax Quota] Received HTML response (Cloudflare block)")
		result.Message = "Quota API blocked by Cloudflare. Chat may still work."
		return result, nil
	}

	// Parse response
	var data map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		log.Printf("[MiniMax Quota] JSON parse error: %v", err)
		return result, nil
	}

	// Check for error code
	if code, ok := data["code"].(float64); ok && code != 0 {
		msg, _ := data["msg"].(string)
		result.Message = fmt.Sprintf("API error (code %d): %s", int(code), msg)
		return result, nil
	}

	// Parse model_remains array (per-model quotas)
	modelRemains, ok := data["model_remains"].([]interface{})
	if !ok {
		result.Message = "No model_remains found in response"
		return result, nil
	}

	// Extract quota per model
	for _, item := range modelRemains {
		mObj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		modelName, _ := mObj["model_name"].(string)
		if modelName == "" {
			continue
		}

		// Interval quota
		// current_interval_usage_count = remaining count, not used count
		intervalTotal, _ := mObj["current_interval_total_count"].(float64)
		intervalRemaining, _ := mObj["current_interval_usage_count"].(float64)
		intervalUsed := intervalTotal - intervalRemaining

		// Weekly quota
		weeklyTotal, _ := mObj["current_weekly_total_count"].(float64)
		weeklyRemaining, _ := mObj["current_weekly_usage_count"].(float64)
		weeklyUsed := weeklyTotal - weeklyRemaining

		// Only show MiniMax-M* model
		if modelName == "MiniMax-M*" {
			if intervalTotal > 0 {
				intervalResetAt := ""
				if endTime, ok := mObj["end_time"].(float64); ok && endTime > 0 {
					intervalResetAt = time.UnixMilli(int64(endTime)).UTC().Format(time.RFC3339)
				}
				result.addBucket(
					"minimax-m*-interval",
					"MiniMax-M* (Interval)",
					int(intervalUsed),
					int(intervalTotal),
					intervalResetAt,
					false,
				)
			}
			if weeklyTotal > 0 {
				weeklyResetAt := ""
				if weeklyEnd, ok := mObj["weekly_end_time"].(float64); ok && weeklyEnd > 0 {
					weeklyResetAt = time.UnixMilli(int64(weeklyEnd)).UTC().Format(time.RFC3339)
				}
				result.addBucket(
					"minimax-m*-weekly",
					"MiniMax-M* (Weekly)",
					int(weeklyUsed),
					int(weeklyTotal),
					weeklyResetAt,
					false,
				)
			}
		}
	}

	// Fallback message
	if len(result.Quotas) == 0 {
		if totalRemains, ok := data["total_remains"].(float64); ok {
			result.Message = fmt.Sprintf("Total remains: %.0f", totalRemains)
		} else {
			result.Message = "No quota data found"
		}
	}

	return result, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// parseResetTime converts various reset-time values to RFC3339 string.
func parseResetTime(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case float64:
		if v <= 0 {
			return ""
		}
		return time.Unix(int64(v), 0).UTC().Format(time.RFC3339)
	default:
		return ""
	}
}

// isAuthExpiredError checks if error indicates auth expiration.
func isAuthExpiredError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{"expired", "authentication", "unauthorized", "401", "403"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
