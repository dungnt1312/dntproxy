package http

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
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// === Quota Check ===
// Makes a lightweight upstream call and reads rate-limit headers.
// For OpenAI: GET /v1/models → reads x-ratelimit-* headers.
// For Kiro: returns token expiry info only (no quota API available).

func apiCheckQuota(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load config"})
			return
		}

		var conn *domain.ProviderConnection
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == id {
				conn = &cfg.ProviderConnections[i]
				break
			}
		}
		if conn == nil {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}

		result := gin.H{
			"provider": conn.Provider,
			"name":     conn.Name,
		}

		// For Kiro: Fetch actual usage limits from Amazon Q / CodeWhisperer
		if conn.Provider == "kiro" {
			handleKiroQuota(c, conn, result)
			return
		}

		// For OpenAI OAuth (ChatGPT web)
		if conn.Provider == "openai" && conn.AuthType == "oauth" {
			handleOpenAIOAuthQuota(c, conn, store, result)
			return
		}

	// For OpenAI API key: call /v1/models and read rate-limit headers.
	// OpenAI-compatible endpoints vary too much, so respect SupportsQuota=false.
	if conn.Provider == "openai" {
		handleOpenAIAPIKeyQuota(c, conn, store, result)
		return
	}

		// For MiniMax: call /v1/api/openplatform/coding_plan/remains
		if conn.Provider == "minimax" {
			handleMiniMaxQuota(c, conn, result)
			return
		}

		// For providers without quota check support (GLM, Qwen, etc.)
		providerCfg := domain.GetProviderConfig(conn.Provider)
		c.JSON(200, gin.H{
			"provider": conn.Provider,
			"name":     conn.Name,
			"hasData":  false,
			"message":  fmt.Sprintf("Quota check not supported for %s", providerCfg.Name),
			"buckets":  []interface{}{},
		})
	}
}

func handleKiroQuota(c *gin.Context, conn *domain.ProviderConnection, result gin.H) {
	if conn.ExpiresAt != "" {
		expTime, parseErr := time.Parse(time.RFC3339, conn.ExpiresAt)
		if parseErr == nil {
			secsLeft := int(time.Until(expTime).Seconds())
			pct := 0
			if conn.ExpiresIn > 0 {
				pct = secsLeft * 100 / conn.ExpiresIn
				if pct < 0 {
					pct = 0
				}
				if pct > 100 {
					pct = 100
				}
			}
			result["tokenSecsLeft"] = secsLeft
			result["tokenPct"] = pct
			result["expiresAt"] = conn.ExpiresAt
			result["expired"] = secsLeft <= 0
		}
	}

	profileArn := ""
	if conn.ProviderSpecificData != nil {
		if v, ok := conn.ProviderSpecificData["profileArn"].(string); ok {
			profileArn = v
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var qResp *http.Response
	var err error

	if profileArn == "" {
		req, _ := http.NewRequest("GET", "https://q.us-east-1.amazonaws.com/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST", nil)
		req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
		req.Header.Set("Accept", "application/json")
		qResp, err = client.Do(req)
	} else {
		payload := map[string]string{"origin": "AI_EDITOR", "profileArn": profileArn, "resourceType": "AGENTIC_REQUEST"}
		payloadBytes, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "https://codewhisperer.us-east-1.amazonaws.com", bytes.NewReader(payloadBytes))
		req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
		req.Header.Set("Content-Type", "application/x-amz-json-1.0")
		req.Header.Set("x-amz-target", "AmazonCodeWhispererService.GetUsageLimits")
		req.Header.Set("Accept", "application/json")
		qResp, err = client.Do(req)
		if err == nil && (qResp.StatusCode != 200 && qResp.StatusCode != 401 && qResp.StatusCode != 403) {
			qResp.Body.Close()
			// Fallback to Q endpoint
			qUrl := fmt.Sprintf("https://q.us-east-1.amazonaws.com/getUsageLimits?origin=AI_EDITOR&profileArn=%s&resourceType=AGENTIC_REQUEST", profileArn)
			req2, _ := http.NewRequest("GET", qUrl, nil)
			req2.Header.Set("Authorization", "Bearer "+conn.AccessToken)
			req2.Header.Set("Accept", "application/json")
			qResp, err = client.Do(req2)
		}
	}

	result["quotaSupported"] = false // default, will be overridden if we parse info

	if err == nil && qResp != nil {
		defer qResp.Body.Close()
		qBodyBytes, _ := io.ReadAll(qResp.Body)
		fmt.Printf("[Kiro Quota] StatusCode: %d\n", qResp.StatusCode)

		// Check for error responses (401, 403, etc.)
		if qResp.StatusCode != 200 {
			var errData map[string]interface{}
			if json.Unmarshal(qBodyBytes, &errData) == nil {
				if msg, ok := errData["message"].(string); ok && msg != "" {
					result["quotaError"] = msg
					if reason, ok := errData["reason"].(string); ok && reason != "" {
						result["quotaErrorReason"] = reason
					}
				} else {
					result["quotaError"] = fmt.Sprintf("HTTP %d: %s", qResp.StatusCode, string(qBodyBytes))
				}
			} else {
				result["quotaError"] = fmt.Sprintf("HTTP %d: Unable to parse error response", qResp.StatusCode)
			}
			result["quotaSupported"] = false
			c.JSON(200, result)
			return
		}

		if qResp.StatusCode == 200 {
			parseKiroQuotaResponse(qBodyBytes, result)
		}
	}

	c.JSON(200, result)
}

func parseKiroQuotaResponse(body []byte, result gin.H) {
	var data map[string]interface{}
	if json.Unmarshal(body, &data) != nil {
		return
	}

	if list, ok := data["usageBreakdownList"].([]interface{}); ok && len(list) > 0 {
		for _, obj := range list {
			b, ok := obj.(map[string]interface{})
			if !ok {
				continue
			}

			resType, _ := b["resourceType"].(string)
			if resType == "" {
				if disp, ok := b["displayName"].(string); ok {
					resType = disp
				}
			}
			used, _ := b["currentUsageWithPrecision"].(float64)
			limit, _ := b["usageLimitWithPrecision"].(float64)

			if strings.EqualFold(resType, "AGENTIC_REQUEST") || strings.EqualFold(resType, "CREDIT") {
				result["requestsRemaining"] = int(limit - used)
				result["requestsLimit"] = int(limit)
				if limit > 0 {
					pct := int((used / limit) * 100)
					if pct > 100 {
						pct = 100
					}
					result["requestsPct"] = pct
				}
				result["quotaSupported"] = true

				if ft, ok := b["freeTrialInfo"].(map[string]interface{}); ok && ft != nil {
					if ftStatus, _ := ft["freeTrialStatus"].(string); ftStatus == "ACTIVE" {
						ftUsed, _ := ft["currentUsageWithPrecision"].(float64)
						ftLimit, _ := ft["usageLimitWithPrecision"].(float64)
						result["freeTrialRemaining"] = int(ftLimit - ftUsed)
						result["freeTrialLimit"] = int(ftLimit)
						if ftLimit > 0 {
							pct := int((ftUsed / ftLimit) * 100)
							if pct > 100 {
								pct = 100
							}
							result["freeTrialPct"] = pct
						}
						if ftExpiry, _ := ft["freeTrialExpiry"].(float64); ftExpiry > 0 {
							result["freeTrialExpiresAt"] = time.Unix(int64(ftExpiry), 0).Format("2006-01-02 15:04")
						}
					}
				}

				// Overage info
				overageUsed, _ := b["currentOveragesWithPrecision"].(float64)
				overageCap, _ := b["overageCapWithPrecision"].(float64)
				overageCharge, _ := b["overageCharges"].(float64)
				overageRate, _ := b["overageRate"].(float64)

				if overageUsed > 0 {
					result["overageUsed"] = overageUsed
					result["overageCap"] = overageCap
					result["overageRemaining"] = overageCap - overageUsed
					result["overageCharge"] = overageCharge
					result["overageRate"] = overageRate

					if overageCfg, ok := data["overageConfiguration"].(map[string]interface{}); ok {
						if status, ok := overageCfg["overageStatus"].(string); ok {
							result["overageStatus"] = status
						}
					}
				}

				if oc, _ := b["overageCharges"].(float64); oc > 0 {
					result["overageCharges"] = oc
				}

			} else if strings.EqualFold(resType, "INLINE_INVOCATION") || strings.EqualFold(resType, "CHAT_REQUEST") {
				result["tokensRemaining"] = int(limit - used)
				result["tokensLimit"] = int(limit)
				if limit > 0 {
					pct := int((used / limit) * 100)
					if pct > 100 {
						pct = 100
					}
					result["tokensPct"] = pct
				}
				result["quotaSupported"] = true
			}
		}
	}

	// Try multiple reset fields
	if resetStr, ok := data["nextDateReset"].(string); ok {
		result["resetRequests"] = resetStr
	} else if resetNum, ok := data["nextDateReset"].(float64); ok {
		result["resetRequests"] = time.Unix(int64(resetNum), 0).Format("2006-01-02 15:04")
	} else if resetStr, ok := data["resetDate"].(string); ok {
		result["resetRequests"] = resetStr
	} else if resetNum, ok := data["resetDate"].(float64); ok {
		result["resetRequests"] = time.Unix(int64(resetNum), 0).Format("2006-01-02 15:04")
	}
}

func handleOpenAIOAuthQuota(c *gin.Context, conn *domain.ProviderConnection, store port.CredentialStore, result gin.H) {
	if conn.ExpiresAt != "" {
		expTime, parseErr := time.Parse(time.RFC3339, conn.ExpiresAt)
		if parseErr == nil {
			secsLeft := int(time.Until(expTime).Seconds())
			elapsed := 0
			pct := 0
			if conn.ExpiresIn > 0 {
				elapsed = conn.ExpiresIn - secsLeft
				if elapsed < 0 {
					elapsed = 0
				}
				pct = elapsed * 100 / conn.ExpiresIn
				if pct < 0 {
					pct = 0
				}
				if pct > 100 {
					pct = 100
				}
			} else {
				elapsed = 3600 - secsLeft
				if elapsed < 0 {
					elapsed = 0
				}
			}
			result["tokenSecsLeft"] = elapsed
			result["tokenPct"] = pct
			result["expiresAt"] = conn.ExpiresAt
			result["expired"] = secsLeft <= 0
		}
	}

	// Try to refresh if expired
	if expired, ok := result["expired"].(bool); ok && expired && conn.RefreshToken != "" {
		updatedConn, refErr := refreshOpenAIConnection(conn, store)
		if refErr == nil {
			conn = updatedConn
			result["expired"] = false
			result["tokenSecsLeft"] = conn.ExpiresIn
			result["tokenPct"] = 100
			result["expiresAt"] = conn.ExpiresAt
		} else {
			c.JSON(401, gin.H{"error": "Token expired and auto-refresh failed: " + refErr.Error()})
			return
		}
	}

	// Probe the Codex Responses API to get real quota usage status.
	codexQuota := checkCodexQuota(conn.AccessToken)
	if codexQuota != nil {
		for k, v := range codexQuota {
			result[k] = v
		}
	} else {
		result["quotaSupported"] = false
	}
	c.JSON(200, result)
}

func handleOpenAIAPIKeyQuota(c *gin.Context, conn *domain.ProviderConnection, store port.CredentialStore, result gin.H) {
	baseURL := conn.BaseURL
	if baseURL == "" && conn.Provider == "openai" {
		baseURL = "https://api.openai.com"
	}
	if baseURL == "" {
		c.JSON(400, gin.H{"error": "No base URL configured"})
		return
	}
	baseURL = domain.StripVersionSuffix(baseURL)

	req, _ := http.NewRequest("GET", baseURL+"/v1/models", nil)
	if conn.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+conn.APIKey)
	} else if conn.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
	}
	req.Header.Set("User-Agent", "dntproxy/1.0")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(502, gin.H{"error": "Request failed: " + err.Error()})
		return
	}

	if (resp.StatusCode == 401 || resp.StatusCode == 403) && conn.Provider == "openai" && conn.RefreshToken != "" {
		resp.Body.Close()
		updatedConn, refErr := refreshOpenAIConnection(conn, store)
		if refErr == nil {
			conn = updatedConn
			req, _ = http.NewRequest("GET", baseURL+"/v1/models", nil)
			req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
			req.Header.Set("User-Agent", "dntproxy/1.0")
			resp, err = httpClient.Do(req)
			if err != nil {
				c.JSON(502, gin.H{"error": "Retry failed: " + err.Error()})
				return
			}
		} else {
			c.JSON(resp.StatusCode, gin.H{"error": "Token expired and auto-refresh failed: " + refErr.Error()})
			return
		}
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) // drain

	// Parse rate-limit headers
	parseHeader := func(h string) int {
		v := resp.Header.Get(h)
		if v == "" {
			return -1
		}
		n := 0
		fmt.Sscanf(v, "%d", &n)
		return n
	}

	result["quotaSupported"] = true
	result["statusCode"] = resp.StatusCode

	reqLimit := parseHeader("x-ratelimit-limit-requests")
	reqRemaining := parseHeader("x-ratelimit-remaining-requests")
	tokLimit := parseHeader("x-ratelimit-limit-tokens")
	tokRemaining := parseHeader("x-ratelimit-remaining-tokens")

	if reqLimit >= 0 {
		result["requestsLimit"] = reqLimit
		result["requestsRemaining"] = reqRemaining
		if reqLimit > 0 {
			result["requestsPct"] = reqRemaining * 100 / reqLimit
		}
	}
	if tokLimit >= 0 {
		result["tokensLimit"] = tokLimit
		result["tokensRemaining"] = tokRemaining
		if tokLimit > 0 {
			result["tokensPct"] = tokRemaining * 100 / tokLimit
		}
	}

	// Also capture reset times
	if v := resp.Header.Get("x-ratelimit-reset-requests"); v != "" {
		result["resetRequests"] = v
	}
	if v := resp.Header.Get("x-ratelimit-reset-tokens"); v != "" {
		result["resetTokens"] = v
	}

	if reqLimit < 0 && tokLimit < 0 {
		result["note"] = "No rate-limit headers returned by upstream"
	}

	c.JSON(200, result)
}

// === MiniMax Quota Check ===

// miniMaxQuotaResponse represents the response from MiniMax coding_plan/remains API.
type miniMaxQuotaResponse struct {
	Code         int    `json:"code"`
	Msg          string `json:"msg"`
	Data         any    `json:"data"`
	BaseResp     any    `json:"base_resp"`
	TotalRemains int    `json:"total_remains"`
	Used         int    `json:"used"`
	Limit        int    `json:"limit"`
	Remains      int    `json:"remains"`
	ResetTime    int64  `json:"reset_time"`
	PlanType     string `json:"plan_type"`
}

func handleMiniMaxQuota(c *gin.Context, conn *domain.ProviderConnection, result gin.H) {
	apiKey := conn.APIKey
	if apiKey == "" {
		c.JSON(400, gin.H{"error": "No API key configured for MiniMax"})
		return
	}

	// Use GET method with browser-like headers (matches test.go)
	quotaURL := "https://api.minimax.io/v1/api/openplatform/coding_plan/remains"

	req, err := http.NewRequest("GET", quotaURL, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create request: " + err.Error()})
		return
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
		result["quotaSupported"] = false
		result["message"] = "Quota API unavailable. Chat may still work."
		result["buckets"] = []interface{}{}
		c.JSON(200, result)
		return
	}
	defer resp.Body.Close()

	// Handle gzip
	var bodyReader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, gzipErr := gzip.NewReader(resp.Body)
		if gzipErr == nil {
			defer gr.Close()
			bodyReader = gr
		}
	}

	bodyBytes, _ := io.ReadAll(bodyReader)

	// Check if response is HTML (Cloudflare block)
	if len(bodyBytes) > 0 && bodyBytes[0] == '<' {
		result["quotaSupported"] = false
		result["message"] = "Quota API blocked by Cloudflare. Chat may still work."
		result["buckets"] = []interface{}{}
		c.JSON(200, result)
		return
	}

	// Parse response
	var data map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		result["quotaSupported"] = false
		result["message"] = "Unable to parse quota response"
		c.JSON(200, result)
		return
	}

	// Check for error code
	if code, ok := data["code"].(float64); ok && code != 0 {
		msg, _ := data["msg"].(string)
		result["quotaSupported"] = false
		result["message"] = fmt.Sprintf("API error: %s", msg)
		c.JSON(200, result)
		return
	}

	// Parse model_remains array
	modelRemains, ok := data["model_remains"].([]interface{})
	if !ok {
		result["quotaSupported"] = false
		result["message"] = "No model_remains in response"
		c.JSON(200, result)
		return
	}

	// Build buckets
	buckets := []map[string]interface{}{}
	for _, item := range modelRemains {
		mObj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		modelName, _ := mObj["model_name"].(string)
		if modelName == "" {
			continue
		}

		// Interval
		// current_interval_usage_count = remaining count, not used count
		intervalTotal, _ := mObj["current_interval_total_count"].(float64)
		intervalRemaining, _ := mObj["current_interval_usage_count"].(float64)
		intervalUsed := intervalTotal - intervalRemaining

		// Weekly
		weeklyTotal, _ := mObj["current_weekly_total_count"].(float64)
		weeklyRemaining, _ := mObj["current_weekly_usage_count"].(float64)
		weeklyUsed := weeklyTotal - weeklyRemaining

		// Only show MiniMax-M* model
		if modelName == "MiniMax-M*" {
			if intervalTotal > 0 {
				resetAt := ""
				if endTime, ok := mObj["end_time"].(float64); ok && endTime > 0 {
					resetAt = time.UnixMilli(int64(endTime)).UTC().Format(time.RFC3339)
				}
				pctUsed := 0
				if intervalTotal > 0 {
					pctUsed = int(intervalUsed * 100 / intervalTotal)
					if pctUsed > 100 {
						pctUsed = 100
					}
				}
				buckets = append(buckets, map[string]interface{}{
					"name":      "MiniMax-M* (Interval)",
					"used":      int(intervalUsed),
					"limit":     int(intervalTotal),
					"remaining": int(intervalRemaining),
					"pctUsed":   pctUsed,
					"resetsAt":  resetAt,
				})
			}
			if weeklyTotal > 0 {
				resetAt := ""
				if weeklyEnd, ok := mObj["weekly_end_time"].(float64); ok && weeklyEnd > 0 {
					resetAt = time.UnixMilli(int64(weeklyEnd)).UTC().Format(time.RFC3339)
				}
				pctUsed := 0
				if weeklyTotal > 0 {
					pctUsed = int(weeklyUsed * 100 / weeklyTotal)
					if pctUsed > 100 {
						pctUsed = 100
					}
				}
				buckets = append(buckets, map[string]interface{}{
					"name":      "MiniMax-M* (Weekly)",
					"used":      int(weeklyUsed),
					"limit":     int(weeklyTotal),
					"remaining": int(weeklyRemaining),
					"pctUsed":   pctUsed,
					"resetsAt":  resetAt,
				})
			}
		}
	}

	result["quotaSupported"] = true
	result["buckets"] = buckets
	c.JSON(200, result)
}

// === Codex API Probing (OpenAI OAuth) ===

// probeCodexAPIResult is the result of probing the Codex Responses API.
type probeCodexAPIResult struct {
	valid bool
	error string
}

// probeCodexAPI sends a minimal invalid request to the Codex Responses API
// to verify that the OAuth token is still valid.
func probeCodexAPI(accessToken string) probeCodexAPIResult {
	body := `{"model":"gpt-5.3-codex","input":[],"stream":false,"store":false}`
	req, err := http.NewRequest("POST", "https://chatgpt.com/backend-api/codex/responses", strings.NewReader(body))
	if err != nil {
		return probeCodexAPIResult{valid: false, error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("originator", "codex-cli")
	req.Header.Set("User-Agent", "codex-cli/1.0.18 (macOS; arm64)")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return probeCodexAPIResult{valid: false, error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 400 || resp.StatusCode == 200 {
		return probeCodexAPIResult{valid: true}
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return probeCodexAPIResult{valid: false, error: fmt.Sprintf("HTTP %d: Token expired or revoked", resp.StatusCode)}
	}
	return probeCodexAPIResult{valid: false, error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

// checkCodexQuota probes the Codex Responses API to determine quota status.
func checkCodexQuota(accessToken string) map[string]interface{} {
	body := `{"model":"gpt-5.4","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"x"}]}],"stream":false,"store":false}`
	req, err := http.NewRequest("POST", "https://chatgpt.com/backend-api/codex/responses", strings.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("originator", "codex-cli")
	req.Header.Set("User-Agent", "codex-cli/1.0.18 (macOS; arm64)")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	result := map[string]interface{}{}

	if resp.StatusCode == 200 || resp.StatusCode == 400 {
		result["quotaSupported"] = true
		result["quotaAvailable"] = true
		result["note"] = "Quota available — ChatGPT plan active."
		return result
	}

	if resp.StatusCode == 429 {
		var errData struct {
			Error struct {
				Type         string  `json:"type"`
				Message      string  `json:"message"`
				PlanType     string  `json:"plan_type"`
				ResetsAt     float64 `json:"resets_at"`
				ResetsInSecs float64 `json:"resets_in_seconds"`
			} `json:"error"`
		}
		result["quotaSupported"] = true
		result["quotaAvailable"] = false
		result["usageLimitReached"] = true
		if json.Unmarshal(bodyBytes, &errData) == nil && errData.Error.Type != "" {
			result["quotaErrorType"] = errData.Error.Type
			result["quotaErrorMsg"] = errData.Error.Message
			if errData.Error.PlanType != "" {
				result["planType"] = errData.Error.PlanType
			}
			if errData.Error.ResetsAt > 0 {
				resetsTime := time.Unix(int64(errData.Error.ResetsAt), 0)
				result["resetsAt"] = resetsTime.UTC().Format(time.RFC3339)
				result["resetsAtHuman"] = resetsTime.Local().Format("2006-01-02 15:04")
			}
			if errData.Error.ResetsInSecs > 0 {
				result["resetsInSeconds"] = int(errData.Error.ResetsInSecs)
			}
		}
		return result
	}

	return nil
}
