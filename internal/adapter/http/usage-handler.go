package http

import (
	"bytes"
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
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	Used      int     `json:"used"`
	Total     int     `json:"total"`
	Remaining int     `json:"remaining"`
	Pct       int     `json:"pct"` // 0-100, percent USED
	ResetAt   string  `json:"resetAt,omitempty"`
	Unlimited bool    `json:"unlimited"`
}

// UsageResponse is the single, canonical response shape for GET /api/usage/:id.
type UsageResponse struct {
	Provider     string        `json:"provider"`
	Plan         string        `json:"plan,omitempty"`
	LimitReached bool          `json:"limitReached"`
	Message      string        `json:"message,omitempty"`
	Quotas       []QuotaBucket `json:"quotas"`
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

	// Non-OAuth connections have no usage API.
	if conn.AuthType != "oauth" {
		c.JSON(http.StatusOK, UsageResponse{
			Provider: conn.Provider,
			Message:  "Usage not available for API key connections",
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
