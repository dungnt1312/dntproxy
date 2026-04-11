package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

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

	// Load config
	cfg, err := h.store.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load config"})
		return
	}

	// Find connection
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

	// Only OAuth connections have usage APIs
	if conn.AuthType != "oauth" {
		c.JSON(http.StatusOK, gin.H{"message": "Usage not available for API key connections"})
		return
	}

	// Refresh credentials if needed
	refreshSvc := auth.NewTokenRefreshService(h.store)
	if refreshSvc.NeedsRefresh(conn) {
		log.Printf("[USAGE] Token expiring soon for %s, refreshing...", conn.Name)
		refreshed, err := refreshSvc.Refresh(conn)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Credential refresh failed: %s", err.Error())})
			return
		}
		conn = refreshed
		h.store.UpdateConnection(conn)
	}

	// Fetch usage from provider API
	usage, err := h.getUsageForProvider(conn)
	if err != nil {
		// Check if auth expired
		if isAuthExpiredError(err) && conn.RefreshToken != "" {
			log.Printf("[USAGE] Auth expired, force refreshing token for %s", conn.Name)
			refreshed, refreshErr := refreshSvc.Refresh(conn)
			if refreshErr == nil {
				conn = refreshed
				h.store.UpdateConnection(conn)
				// Retry usage fetch
				usage, err = h.getUsageForProvider(conn)
			}
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, usage)
}

// getUsageForProvider fetches usage data from the provider's API.
func (h *UsageHandler) getUsageForProvider(conn *domain.ProviderConnection) (interface{}, error) {
	switch conn.Provider {
	case "openai":
		return h.getOpenAIUsage(conn)
	case "kiro":
		return h.getKiroUsage(conn)
	default:
		return gin.H{"message": fmt.Sprintf("Usage API not implemented for %s", conn.Provider)}, nil
	}
}

// getOpenAIUsage fetches OpenAI usage/quota.
func (h *UsageHandler) getOpenAIUsage(conn *domain.ProviderConnection) (interface{}, error) {
	usage, err := auth.GetOpenAIUsage(conn.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenAI usage: %w", err)
	}

	// Parse rate limit info
	rateLimit, _ := usage["rate_limit"].(map[string]interface{})
	primaryWindow, _ := rateLimit["primary_window"].(map[string]interface{})
	secondaryWindow, _ := rateLimit["secondary_window"].(map[string]interface{})

	planType, _ := usage["plan_type"].(string)
	limitReached, _ := rateLimit["limit_reached"].(bool)

	sessionUsed, _ := primaryWindow["used_percent"].(float64)
	weeklyUsed, _ := secondaryWindow["used_percent"].(float64)

	sessionResetAt := parseResetTime(primaryWindow["reset_at"])
	weeklyResetAt := parseResetTime(secondaryWindow["reset_at"])

	return gin.H{
		"plan":         planType,
		"limitReached": limitReached,
		"quotas": gin.H{
			"session": gin.H{
				"used":      int(sessionUsed),
				"total":     100,
				"remaining": 100 - int(sessionUsed),
				"resetAt":   sessionResetAt,
				"unlimited": false,
			},
			"weekly": gin.H{
				"used":      int(weeklyUsed),
				"total":     100,
				"remaining": 100 - int(weeklyUsed),
				"resetAt":   weeklyResetAt,
				"unlimited": false,
			},
		},
	}, nil
}

// getKiroUsage fetches Kiro (AWS CodeWhisperer) usage/quota.
func (h *UsageHandler) getKiroUsage(conn *domain.ProviderConnection) (interface{}, error) {
	// Default profileArn
	profileArn := "arn:aws:codewhisperer:us-east-1:638616132270:profile/AAAACCCCXXXX"
	if conn.ProviderSpecificData != nil {
		if arn, ok := conn.ProviderSpecificData["profileArn"].(string); ok && arn != "" {
			profileArn = arn
		}
	}

	// Build request payload
	payload := map[string]interface{}{
		"origin":       "AI_EDITOR",
		"profileArn":   profileArn,
		"resourceType": "AGENTIC_REQUEST",
	}

	payloadBytes, _ := json.Marshal(payload)

	// Make request
	req, err := http.NewRequest("POST", "https://codewhisperer.us-east-1.amazonaws.com", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("x-amz-target", "AmazonCodeWhispererService.GetUsageLimits")
	req.Header.Set("Accept", "application/json")
	req.Body = newJSONReader(payloadBytes)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 401 {
		return gin.H{
			"message": "Kiro quota API authentication expired. Chat may still work.",
			"quotas":  gin.H{},
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kiro API error: %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Parse usage data
	usageList, _ := data["usageBreakdownList"].([]interface{})
	quotaInfo := make(map[string]interface{})
	resetAt := parseResetTime(data["nextDateReset"])

	for _, item := range usageList {
		breakdown, _ := item.(map[string]interface{})
		resourceType, _ := breakdown["resourceType"].(string)
		used, _ := breakdown["currentUsageWithPrecision"].(float64)
		total, _ := breakdown["usageLimitWithPrecision"].(float64)

		quotaInfo[resourceType] = gin.H{
			"used":      int(used),
			"total":     int(total),
			"remaining": int(total - used),
			"resetAt":   resetAt,
			"unlimited": false,
		}
	}

	subscriptionInfo, _ := data["subscriptionInfo"].(map[string]interface{})
	subscriptionTitle, _ := subscriptionInfo["subscriptionTitle"].(string)

	return gin.H{
		"plan":   subscriptionTitle,
		"quotas": quotaInfo,
	}, nil
}

// parseResetTime parses reset time from various formats.
func parseResetTime(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case float64:
		// Unix timestamp in seconds - convert to RFC3339
		// return time.Unix(int64(v), 0).Format(time.RFC3339)
		return ""
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
