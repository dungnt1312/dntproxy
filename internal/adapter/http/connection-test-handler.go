package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/kiro"
	openai "github.com/dungnt/dntproxy/internal/adapter/openai"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/logger"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// === Test Connection ===

func apiTestConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		// Load a snapshot of the connection for testing (read-only)
		conn, err := store.GetConnectionByID(id)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if conn == nil {
			c.JSON(404, gin.H{"error": "Connection not found"})
			return
		}

		refreshSvc := auth.NewTokenRefreshService(store)
		if refreshSvc.NeedsRefresh(conn) {
			refreshed, err := refreshSvc.Refresh(conn)
			if err != nil {
				c.JSON(200, gin.H{"status": "error", "message": "Token refresh failed: " + err.Error()})
				return
			}
			*conn = *refreshed
			_ = store.UpdateConnection(conn)
		}

		email := ""
		if conn.AccessToken != "" {
			email = auth.ExtractEmailFromJWT(conn.AccessToken)
		}

		// Helper to atomically update connection test status
		updateTestStatus := func(status, lastErr string) {
			_ = store.Update(func(cfg *domain.AppConfig) {
				for i := range cfg.ProviderConnections {
					if cfg.ProviderConnections[i].ID == id {
						c := &cfg.ProviderConnections[i]
						c.TestStatus = status
						if status == "active" {
							c.LastError = ""
							c.LastErrorAt = ""
						} else {
							c.LastError = lastErr
							c.LastErrorAt = time.Now().UTC().Format(time.RFC3339)
						}
						break
					}
				}
			})
		}

		// For OpenAI OAuth connections, probe the Codex Responses API
		if conn.Provider == "openai" && conn.AuthType == "oauth" && conn.AccessToken != "" {
			probeResult := probeCodexAPI(conn.AccessToken)
			if !probeResult.valid {
				// Try refresh if we have a refresh token
				if conn.RefreshToken != "" {
					refreshed, refErr := refreshSvc.Refresh(conn)
					if refErr == nil {
						*conn = *refreshed
						_ = store.UpdateConnection(conn)
						probeResult = probeCodexAPI(conn.AccessToken)
					}
				}
			}
			if probeResult.valid {
				updateTestStatus("active", "")
				c.JSON(200, gin.H{
					"status":    "ok",
					"hasToken":  true,
					"expiresAt": conn.ExpiresAt,
					"email":     email,
				})
			} else {
				c.JSON(200, gin.H{
					"status":  "error",
					"message": "OpenAI OAuth token invalid: " + probeResult.error,
				})
			}
			return
		}

		// For API key providers (GLM, MiniMax, Qwen API key, etc.), send a test request
		if conn.APIKey != "" {
			testResult := testProviderAPI(conn)
			if testResult.OK {
				updateTestStatus("active", "")
				c.JSON(200, gin.H{
					"status":    "ok",
					"hasApiKey": true,
					"response":  testResult.Response,
				})
			} else {
				updateTestStatus("error", testResult.Error)
				c.JSON(200, gin.H{
					"status":  "error",
					"message": testResult.Error,
				})
			}
			return
		}

		// For OAuth providers with access token, send a test request
		if conn.AccessToken != "" {
			testResult := testProviderAPI(conn)
			if testResult.OK {
				updateTestStatus("active", "")
				c.JSON(200, gin.H{
					"status":    "ok",
					"hasToken":  true,
					"expiresAt": conn.ExpiresAt,
					"email":     email,
					"response":  testResult.Response,
				})
			} else {
				updateTestStatus("error", testResult.Error)
				c.JSON(200, gin.H{
					"status":  "error",
					"message": testResult.Error,
				})
			}
			return
		}
	}
}

// testProviderResult holds the result of a provider API test.
type testProviderResult struct {
	OK       bool
	Response string
	Error    string
}

// testProviderAPI sends a minimal test request to the provider's API.
func testProviderAPI(conn *domain.ProviderConnection) testProviderResult {
	cfg := domain.GetProviderConfig(conn.Provider)

	// Kiro uses AWS EventStream, not HTTP chat completions
	if cfg.Format == domain.FormatAWSKiro {
		// For Kiro, just check that we have a valid token
		if conn.AccessToken == "" {
			return testProviderResult{OK: false, Error: "No access token available"}
		}
		return testProviderResult{OK: true, Response: "Token present (full test requires actual API call)"}
	}

	baseURL := conn.BaseURL
	if baseURL == "" {
		baseURL = cfg.DefaultBaseURL
	}
	baseURL = domain.StripVersionSuffix(baseURL)
	chatPath := cfg.ChatPath
	if chatPath == "" {
		chatPath = "/v1/chat/completions"
	}
	url := baseURL + chatPath

	// Use first supported model for test, fallback to a generic one
	testModel := "test"
	if len(conn.SupportedModels) > 0 {
		testModel = conn.SupportedModels[0]
	} else if len(cfg.DefaultModels) > 0 {
		testModel = cfg.DefaultModels[0]
	}

	body := map[string]interface{}{
		"model":      testModel,
		"messages":   []map[string]string{{"role": "user", "content": "Hi"}},
		"max_tokens": 1,
		"stream":     false,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return testProviderResult{OK: false, Error: fmt.Sprintf("create request: %s", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	if conn.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+conn.APIKey)
	} else if conn.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return testProviderResult{OK: false, Error: fmt.Sprintf("request failed: %s", err)}
	}
	defer resp.Body.Close()

	// 200 means auth + connectivity OK
	if resp.StatusCode == 200 {
		return testProviderResult{OK: true, Response: fmt.Sprintf("HTTP 200 OK")}
	}

	// 400 with "test" model often means auth OK but model name wrong = success
	if resp.StatusCode == 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		bodyStr := string(respBody)
		// If error mentions "model" but not "auth" or "key", auth is likely OK
		if strings.Contains(strings.ToLower(bodyStr), "model") &&
			!strings.Contains(strings.ToLower(bodyStr), "api key") &&
			!strings.Contains(strings.ToLower(bodyStr), "authentication") &&
			!strings.Contains(strings.ToLower(bodyStr), "unauthorized") {
			return testProviderResult{OK: true, Response: fmt.Sprintf("Auth OK (model test not found)")}
		}
		return testProviderResult{OK: false, Error: fmt.Sprintf("HTTP 400: %s", bodyStr)}
	}

	// 401/403 = auth failure
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return testProviderResult{OK: false, Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))}
	}

	// Other status codes
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return testProviderResult{OK: false, Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))}
}

// === Test Model ===

func apiTestModel(store port.CredentialStore, providers port.ProviderRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Model string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
			c.JSON(400, gin.H{"error": "model is required"})
			return
		}

		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
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

		// Strip provider prefix from model if present (e.g., "glm/glm-4.6" -> "glm-4.6")
		modelName := req.Model
		if strings.Contains(modelName, "/") {
			parts := strings.SplitN(modelName, "/", 2)
			modelName = parts[1]
		}

		refreshSvc := auth.NewTokenRefreshService(store)
		if refreshSvc.NeedsRefresh(conn) {
			refreshed, err := refreshSvc.Refresh(conn)
			if err != nil {
				c.JSON(200, gin.H{"status": "error", "message": "Token refresh failed: " + err.Error()})
				return
			}
			*conn = *refreshed
			store.Save(cfg)
		}

		creds := shared.ConnectionToCredentials(conn)

		// Build the test body based on the connection type.
		// OpenAI OAuth → Codex Responses API: does NOT support max_tokens.
		var testBody map[string]interface{}
		if conn.Provider == "openai" && conn.AuthType == "oauth" {
			testBody = map[string]interface{}{
				"model": modelName,
				"messages": []map[string]string{
					{"role": "user", "content": "Hi"},
				},
				"stream": false,
			}
		} else {
			testBody = map[string]interface{}{
				"model": modelName,
				"messages": []map[string]string{
					{"role": "user", "content": "Hi"},
				},
				"stream":     false,
				"max_tokens": 5,
			}
		}
		bodyBytes, _ := json.Marshal(testBody)

		// Try provider registry first, fallback to direct instantiation
		provider := conn.Provider
		executor := providers.GetExecutor(provider)
		if executor == nil {
			// Fallback for providers not yet registered
			switch provider {
			case "kiro":
				executor = kiro.NewExecutor()
			case "openai", "openai-compatible":
				executor = openai.NewExecutor()
			default:
				c.JSON(400, gin.H{"status": "error", "message": "Unsupported provider: " + provider})
				return
			}
		}

		reqlog := logger.NewRequestLog(uuid.New().String())
		stream, statusCode, execErr := executor.Execute(modelName, bodyBytes, creds, reqlog)
		if stream != nil {
			stream.Close()
		}

		if execErr != nil {
			c.JSON(200, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("HTTP %d: %s", statusCode, execErr.Error()),
				"code":    statusCode,
			})
			return
		}

		if statusCode != 200 {
			c.JSON(200, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("Upstream returned HTTP %d", statusCode),
				"code":    statusCode,
			})
			return
		}

		c.JSON(200, gin.H{
			"status": "ok",
			"model":  modelName,
		})
	}
}
