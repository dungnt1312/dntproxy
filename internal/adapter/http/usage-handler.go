package http

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/commandcode"
	"github.com/dungnt/dntproxy/internal/adapter/kiro"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
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
	Unit      string `json:"unit,omitempty"`
	Scale     int    `json:"scale,omitempty"`
}

// UsageResponse is the single, canonical response shape for GET /api/usage/:id.
type UsageResponse struct {
	Provider     string                `json:"provider"`
	Plan         string                `json:"plan,omitempty"`
	LimitReached bool                  `json:"limitReached"`
	Message      string                `json:"message,omitempty"`
	Quotas       []QuotaBucket         `json:"quotas"`
	Overages     *OverageInfo          `json:"overages,omitempty"`
	History      []BillingHistoryEntry `json:"history,omitempty"`
	Extras       map[string]any        `json:"extras,omitempty"`
}

// OverageInfo contains overage / on-demand usage when the plan exposes a cap.
type OverageInfo struct {
	Used      float64 `json:"used"`
	Cap       float64 `json:"cap"`
	Remaining float64 `json:"remaining"`
	Status    string  `json:"status,omitempty"`
	Charge    float64 `json:"charge,omitempty"`
	Rate      float64 `json:"rate,omitempty"`
}

// BillingHistoryEntry is one past billing cycle (xAI Grok Chat, etc.).
type BillingHistoryEntry struct {
	Year         int `json:"year"`
	Month        int `json:"month"`
	IncludedUsed int `json:"includedUsed"`
	OnDemandUsed int `json:"onDemandUsed"`
	TotalUsed    int `json:"totalUsed"`
}

func (r *UsageResponse) addBucket(key, label string, used, total int, resetAt string, unlimited bool) {
	r.addBucketWithUnit(key, label, used, total, resetAt, unlimited, "", 0)
}

func (r *UsageResponse) addBucketWithUnit(key, label string, used, total int, resetAt string, unlimited bool, unit string, scale int) {
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
		Pct: pct, ResetAt: resetAt, Unlimited: unlimited, Unit: unit, Scale: scale,
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

	// Verify ownership before making any upstream calls with the connection's credentials.
	conn, ok := requireTenantOwnsConnection(c, h.store, connectionID)
	if !ok {
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
		return fetchXAIGrokChatUsage(conn)
	case "commandcode":
		return fetchCommandCodeUsage(conn)
	case "glm":
		return fetchGLMUsage(conn)
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
		kiro.ApplyConnectionAuth(req.Header, conn)
		req.Header.Set("Accept", "application/json")
		httpResp, err = client.Do(req)
	} else {
		// IDC/imported path: POST to CodeWhisperer, fallback to Q
		payload, _ := json.Marshal(map[string]string{
			"origin": "AI_EDITOR", "profileArn": profileArn, "resourceType": "AGENTIC_REQUEST",
		})
		req, _ := http.NewRequest("POST", "https://codewhisperer.us-east-1.amazonaws.com",
			bytes.NewReader(payload))
		kiro.ApplyConnectionAuth(req.Header, conn)
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
			kiro.ApplyConnectionAuth(req2.Header, conn)
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

// ─── Command Code ───────────────────────────────────────────────────────────

type commandCodeWhoAmI struct {
	Org *struct {
		ID string `json:"id"`
	} `json:"org"`
}

type commandCodeSubscription struct {
	Success bool `json:"success"`
	Data    struct {
		Status             string `json:"status"`
		PlanID             string `json:"planId"`
		CurrentPeriodStart string `json:"currentPeriodStart"`
		CurrentPeriodEnd   string `json:"currentPeriodEnd"`
	} `json:"data"`
}

type commandCodeUsageSummary struct {
	TotalCount     int     `json:"totalCount"`
	TotalCost      float64 `json:"totalCost"`
	SuccessRate    float64 `json:"successRate"`
	CompletedCount int     `json:"completedCount"`
	FailedCount    int     `json:"failedCount"`
	TotalTokensIn  int     `json:"totalTokensIn"`
	TotalTokensOut int     `json:"totalTokensOut"`
	TotalTokens    int     `json:"totalTokens"`
	TotalCredits   float64 `json:"totalCredits"`
	PeriodBasis    string  `json:"periodBasis"`
}

type commandCodeCredits struct {
	Credits struct {
		BelowThreshold bool    `json:"belowThreshold"`
		MonthlyCredits float64 `json:"monthlyCredits"`
	} `json:"credits"`
	WindowLimits struct {
		Exceeded bool `json:"exceeded"`
		FiveHour struct {
			Used     float64 `json:"used"`
			Cap      float64 `json:"cap"`
			Exceeded bool    `json:"exceeded"`
			ResetAt  int64   `json:"resetAt"`
		} `json:"fiveHour"`
		Weekly struct {
			Used     float64 `json:"used"`
			Cap      float64 `json:"cap"`
			Exceeded bool    `json:"exceeded"`
			ResetAt  int64   `json:"resetAt"`
		} `json:"weekly"`
	} `json:"windowLimits"`
}

func fetchCommandCodeUsage(conn *domain.ProviderConnection) (*UsageResponse, error) {
	apiKey := conn.APIKey
	if apiKey == "" {
		apiKey = conn.AccessToken
	}
	if apiKey == "" {
		return &UsageResponse{
			Provider: "commandcode",
			Message:  "No API key configured for Command Code",
			Quotas:   []QuotaBucket{},
		}, nil
	}

	baseURL := strings.TrimRight(strings.TrimSpace(conn.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.commandcode.ai"
	}
	if err := shared.ValidateOutboundURL(baseURL, shared.AllowPrivateOutbound(conn.TenantID)); err != nil {
		return nil, fmt.Errorf("invalid Command Code base URL: %w", err)
	}

	client := shared.NewSafeHTTPClient(12 * time.Second)
	var whoami commandCodeWhoAmI
	if err := commandCodeGet(client, baseURL, apiKey, "/alpha/whoami", nil, &whoami); err != nil {
		return nil, err
	}
	params := url.Values{}
	if whoami.Org != nil && strings.TrimSpace(whoami.Org.ID) != "" {
		params.Set("orgId", whoami.Org.ID)
	}
	var subscription commandCodeSubscription
	if err := commandCodeGet(client, baseURL, apiKey, "/alpha/billing/subscriptions", params, &subscription); err != nil {
		return nil, err
	}

	var credits commandCodeCredits
	var summary commandCodeUsageSummary
	var creditsErr, summaryErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		creditsErr = commandCodeGet(client, baseURL, apiKey, "/alpha/billing/credits", params, &credits)
	}()
	go func() {
		defer wg.Done()
		summaryParams := url.Values{}
		for key, values := range params {
			summaryParams[key] = append([]string(nil), values...)
		}
		if subscription.Data.CurrentPeriodStart != "" {
			summaryParams.Set("since", subscription.Data.CurrentPeriodStart)
		}
		summaryErr = commandCodeGet(client, baseURL, apiKey, "/alpha/usage/summary", summaryParams, &summary)
	}()
	wg.Wait()
	if creditsErr != nil {
		return nil, creditsErr
	}
	if summaryErr != nil {
		return nil, summaryErr
	}

	result := &UsageResponse{
		Provider:     "commandcode",
		Plan:         subscription.Data.PlanID,
		LimitReached: credits.WindowLimits.Exceeded || credits.WindowLimits.FiveHour.Exceeded || credits.WindowLimits.Weekly.Exceeded,
		Quotas:       []QuotaBucket{},
		Extras: map[string]any{
			"subscriptionStatus": subscription.Data.Status,
			"currentPeriodStart": subscription.Data.CurrentPeriodStart,
			"currentPeriodEnd":   subscription.Data.CurrentPeriodEnd,
			"totalCount":         summary.TotalCount,
			"totalCost":          summary.TotalCost,
			"successRate":        summary.SuccessRate,
			"completedCount":     summary.CompletedCount,
			"failedCount":        summary.FailedCount,
			"totalTokensIn":      summary.TotalTokensIn,
			"totalTokensOut":     summary.TotalTokensOut,
			"totalTokens":        summary.TotalTokens,
			"totalCredits":       summary.TotalCredits,
			"periodBasis":        summary.PeriodBasis,
		},
	}
	const creditScale = 1000
	monthlyUsed := int(math.Round(summary.TotalCredits * creditScale))
	monthlyRemaining := int(math.Round(credits.Credits.MonthlyCredits * creditScale))
	if monthlyUsed > 0 || monthlyRemaining > 0 {
		result.addBucketWithUnit("monthly-credits", "Monthly credits", monthlyUsed, monthlyUsed+monthlyRemaining, subscription.Data.CurrentPeriodEnd, false, "credits", creditScale)
	}
	if credits.WindowLimits.FiveHour.Cap > 0 {
		result.addBucketWithUnit("five-hour", "5-hour", int(math.Round(credits.WindowLimits.FiveHour.Used*creditScale)), int(math.Round(credits.WindowLimits.FiveHour.Cap*creditScale)), commandCodeResetAt(credits.WindowLimits.FiveHour.ResetAt), false, "credits", creditScale)
	}
	if credits.WindowLimits.Weekly.Cap > 0 {
		result.addBucketWithUnit("weekly", "Weekly", int(math.Round(credits.WindowLimits.Weekly.Used*creditScale)), int(math.Round(credits.WindowLimits.Weekly.Cap*creditScale)), commandCodeResetAt(credits.WindowLimits.Weekly.ResetAt), false, "credits", creditScale)
	}
	if credits.Credits.BelowThreshold {
		result.Message = "Command Code credits are below the configured threshold."
	} else if subscription.Data.Status != "" && subscription.Data.Status != "active" {
		result.Message = fmt.Sprintf("Command Code subscription status: %s", subscription.Data.Status)
	}
	return result, nil
}

func commandCodeGet(client *http.Client, baseURL, apiKey, path string, params url.Values, target any) error {
	endpoint := baseURL + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create Command Code request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-command-code-version", commandcode.CommandCodeVersion())
	req.Header.Set("x-cli-environment", "production")
	req.Header.Set("User-Agent", "command-code/"+commandcode.CommandCodeVersion())

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Command Code %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read Command Code %s response: %w", path, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Command Code %s returned HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("parse Command Code %s response: %w", path, err)
	}
	return nil
}

func commandCodeResetAt(millis int64) string {
	if millis <= 0 {
		return ""
	}
	return time.UnixMilli(millis).UTC().Format(time.RFC3339)
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

// nestedValInt reads {"val": N} style numbers from Grok billing payloads.
func nestedValInt(m map[string]interface{}, key string) (int, bool) {
	raw, ok := m[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case map[string]interface{}:
		if n, ok := v["val"].(float64); ok {
			return int(n), true
		}
	case float64:
		return int(v), true
	}
	return 0, false
}

func nestedValFloat(m map[string]interface{}, key string) (float64, bool) {
	raw, ok := m[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case map[string]interface{}:
		if n, ok := v["val"].(float64); ok {
			return n, true
		}
	case float64:
		return v, true
	}
	return 0, false
}

func fetchGrokBillingJSON(client *http.Client, accessToken, url string) (map[string]interface{}, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, resp.StatusCode, nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// addPercentBucket models Grok credit percentages as used/total on a 0-100 scale.
func (r *UsageResponse) addPercentBucket(key, label string, pct float64, resetAt string) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	used := int(pct + 0.5) // round
	if used > 100 {
		used = 100
	}
	r.addBucket(key, label, used, 100, resetAt, false)
	if used >= 100 {
		r.LimitReached = true
	}
}

func applyGrokCreditsConfig(respObj *UsageResponse, config map[string]interface{}) {
	periodEnd := ""
	periodStart := ""
	periodType := ""

	if period, ok := config["currentPeriod"].(map[string]interface{}); ok {
		if t, ok := period["type"].(string); ok {
			periodType = t
		}
		if s, ok := period["start"].(string); ok {
			periodStart = s
		}
		if e, ok := period["end"].(string); ok {
			periodEnd = e
		}
	}
	if periodEnd == "" {
		if e, ok := config["billingPeriodEnd"].(string); ok {
			periodEnd = e
		}
	}
	if periodStart == "" {
		if s, ok := config["billingPeriodStart"].(string); ok {
			periodStart = s
		}
	}

	// Plan label prefers weekly credits period when present.
	if periodStart != "" && periodEnd != "" {
		startLabel, endLabel := periodStart, periodEnd
		if len(startLabel) >= 10 {
			startLabel = startLabel[:10]
		}
		if len(endLabel) >= 10 {
			endLabel = endLabel[:10]
		}
		label := "Credits"
		if strings.Contains(strings.ToUpper(periodType), "WEEKLY") {
			label = "Weekly credits"
		} else if strings.Contains(strings.ToUpper(periodType), "MONTHLY") {
			label = "Monthly credits"
		}
		respObj.Plan = fmt.Sprintf("%s: %s → %s", label, startLabel, endLabel)
	}

	// Overall credit usage (primary signal for api.x.ai spending-limit).
	if pct, ok := config["creditUsagePercent"].(float64); ok {
		respObj.addPercentBucket("credits", "API Credits", pct, periodEnd)
	}

	// Per-product breakdown (Api, GrokBuild, ...).
	if products, ok := config["productUsage"].([]interface{}); ok {
		for _, item := range products {
			row, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			product, _ := row["product"].(string)
			if product == "" {
				continue
			}
			pct, hasPct := row["usagePercent"].(float64)
			if !hasPct {
				// Product listed without percent — skip numeric bucket.
				continue
			}
			key := "product-" + strings.ToLower(product)
			label := product
			switch strings.ToLower(product) {
			case "api":
				label = "API"
			case "grokbuild":
				label = "Grok Build"
			}
			respObj.addPercentBucket(key, label, pct, periodEnd)
		}
	}

	// On-demand from credits payload (includes onDemandUsed directly).
	onDemandCap, hasCap := nestedValFloat(config, "onDemandCap")
	onDemandUsed, _ := nestedValFloat(config, "onDemandUsed")
	if hasCap {
		status := "DISABLED"
		if onDemandCap > 0 {
			status = "ENABLED"
		}
		rem := onDemandCap - onDemandUsed
		if rem < 0 {
			rem = 0
		}
		respObj.Overages = &OverageInfo{
			Used:      onDemandUsed,
			Cap:       onDemandCap,
			Remaining: rem,
			Status:    status,
		}
	}

	if bal, ok := nestedValFloat(config, "prepaidBalance"); ok && bal > 0 {
		respObj.Message = fmt.Sprintf("Prepaid balance: %.2f", bal)
	}
}

func applyGrokRequestsConfig(respObj *UsageResponse, config map[string]interface{}) {
	monthlyLimit, _ := nestedValInt(config, "monthlyLimit")
	used, _ := nestedValInt(config, "used")

	periodEnd := ""
	if end, ok := config["billingPeriodEnd"].(string); ok {
		periodEnd = end
	}

	// Only set plan from requests payload if credits did not already set it.
	if respObj.Plan == "" {
		if start, ok := config["billingPeriodStart"].(string); ok && len(start) >= 10 {
			endLabel := periodEnd
			if len(endLabel) >= 10 {
				endLabel = endLabel[:10]
			}
			respObj.Plan = fmt.Sprintf("Billing: %s → %s", start[:10], endLabel)
		}
	}

	if monthlyLimit > 0 {
		respObj.addBucket("requests", "Monthly Requests", used, monthlyLimit, periodEnd, false)
		if used >= monthlyLimit {
			respObj.LimitReached = true
		}
	}

	// Fill overages only if credits path did not already set them.
	if respObj.Overages == nil {
		if onDemandCap, hasOnDemand := nestedValInt(config, "onDemandCap"); hasOnDemand {
			status := "DISABLED"
			if onDemandCap > 0 {
				status = "ENABLED"
			}
			respObj.Overages = &OverageInfo{
				Used:      0,
				Cap:       float64(onDemandCap),
				Remaining: float64(onDemandCap),
				Status:    status,
			}
		}
	}

	if hist, ok := config["history"].([]interface{}); ok {
		for _, item := range hist {
			row, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			entry := BillingHistoryEntry{}
			if cycle, ok := row["billingCycle"].(map[string]interface{}); ok {
				if y, ok := cycle["year"].(float64); ok {
					entry.Year = int(y)
				}
				if m, ok := cycle["month"].(float64); ok {
					entry.Month = int(m)
				}
			}
			if v, ok := nestedValInt(row, "includedUsed"); ok {
				entry.IncludedUsed = v
			}
			if v, ok := nestedValInt(row, "onDemandUsed"); ok {
				entry.OnDemandUsed = v
			}
			if v, ok := nestedValInt(row, "totalUsed"); ok {
				entry.TotalUsed = v
			} else {
				entry.TotalUsed = entry.IncludedUsed + entry.OnDemandUsed
			}
			respObj.History = append(respObj.History, entry)
		}
	}

	if respObj.Overages != nil && respObj.Overages.Cap > 0 && respObj.Overages.Used == 0 && len(respObj.History) > 0 {
		newest := respObj.History[0]
		if newest.OnDemandUsed > 0 {
			usedOD := float64(newest.OnDemandUsed)
			respObj.Overages.Used = usedOD
			rem := respObj.Overages.Cap - usedOD
			if rem < 0 {
				rem = 0
			}
			respObj.Overages.Remaining = rem
		}
	}
}

// fetchXAIGrokChatUsage loads Grok billing:
//  1. ?format=credits → weekly API credits / product usage (explains spending-limit)
//  2. default billing → monthly request counters + history
func fetchXAIGrokChatUsage(conn *domain.ProviderConnection) (*UsageResponse, error) {
	if conn.AccessToken == "" {
		return &UsageResponse{
			Provider: "xai",
			Message:  "No access token available for xAI Grok Chat",
			Quotas:   []QuotaBucket{},
		}, nil
	}

	client := &http.Client{Timeout: 15 * time.Second}
	respObj := &UsageResponse{
		Provider: "xai",
		Message:  "xAI Grok billing info retrieved",
		Quotas:   []QuotaBucket{},
	}

	creditsData, creditsStatus, creditsErr := fetchGrokBillingJSON(
		client, conn.AccessToken,
		"https://cli-chat-proxy.grok.com/v1/billing?format=credits",
	)
	requestsData, requestsStatus, requestsErr := fetchGrokBillingJSON(
		client, conn.AccessToken,
		"https://cli-chat-proxy.grok.com/v1/billing",
	)

	// Prefer credits shape when available (weekly API credits).
	if creditsErr == nil && creditsStatus == 200 && creditsData != nil {
		if config, ok := creditsData["config"].(map[string]interface{}); ok && config != nil {
			applyGrokCreditsConfig(respObj, config)
		}
	}

	if requestsErr == nil && requestsStatus == 200 && requestsData != nil {
		if config, ok := requestsData["config"].(map[string]interface{}); ok && config != nil {
			applyGrokRequestsConfig(respObj, config)
		}
	}

	if len(respObj.Quotas) == 0 && respObj.Overages == nil {
		// Both failed or empty — surface status for debugging.
		if creditsErr != nil {
			return nil, fmt.Errorf("xai credits billing: %w", creditsErr)
		}
		if requestsErr != nil {
			return nil, fmt.Errorf("xai requests billing: %w", requestsErr)
		}
		if creditsStatus != 200 && requestsStatus != 200 {
			respObj.Message = fmt.Sprintf("Billing endpoints returned HTTP credits=%d requests=%d", creditsStatus, requestsStatus)
		}
	}

	return respObj, nil
}
