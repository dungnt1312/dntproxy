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

		// Verify ownership before making any upstream calls with the connection's credentials.
		conn, ok := requireTenantOwnsConnection(c, store, id)
		if !ok {
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
	if cfg.Format == domain.FormatImageAPI {
		return testImageProviderAPI(conn, cfg)
	}

	baseURL := conn.BaseURL
	if baseURL == "" {
		baseURL = cfg.DefaultBaseURL
	}
	chatPath := cfg.ChatPath
	if chatPath == "" {
		chatPath = "/v1/chat/completions"
	}
	url := providerTestURL(conn, cfg)

	// Use first supported model for test, fallback to a generic one
	testModel := "test"
	if len(conn.SupportedModels) > 0 {
		testModel = conn.SupportedModels[0]
	} else if len(cfg.DefaultModels) > 0 {
		testModel = cfg.DefaultModels[0]
	}

	var body map[string]interface{}
	if cfg.ChatPath == "/responses" {
		body = map[string]interface{}{
			"model":             testModel,
			"input":             []map[string]string{{"role": "user", "content": "Hi"}},
			"max_output_tokens": 1,
			"stream":            false,
		}
	} else {
		body = map[string]interface{}{
			"model":      testModel,
			"messages":   []map[string]string{{"role": "user", "content": "Hi"}},
			"max_tokens": 1,
			"stream":     false,
		}
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

// testImageProviderAPI performs a non-generative auth/connectivity probe so
// testing an image-only connection never creates a billable image.
func testImageProviderAPI(conn *domain.ProviderConnection, cfg domain.ProviderConfig) testProviderResult {
	baseURL := strings.TrimRight(strings.TrimSpace(conn.BaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(cfg.DefaultBaseURL, "/")
	}
	probeURL := baseURL + "/models"
	req, err := http.NewRequest(http.MethodGet, probeURL, nil)
	if err != nil {
		return testProviderResult{OK: false, Error: fmt.Sprintf("create request: %s", err)}
	}
	apiKey := conn.APIKey
	if apiKey == "" {
		apiKey = conn.AccessToken
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return testProviderResult{OK: false, Error: fmt.Sprintf("request failed: %s", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return testProviderResult{OK: true, Response: fmt.Sprintf("HTTP %d OK (non-generative probe)", resp.StatusCode)}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return testProviderResult{OK: false, Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))}
	}
	return testProviderResult{OK: false, Error: fmt.Sprintf("Non-generative probe returned HTTP %d: %s", resp.StatusCode, string(body))}
}

func providerTestURL(conn *domain.ProviderConnection, cfg domain.ProviderConfig) string {
	baseURL := conn.BaseURL
	if baseURL == "" {
		baseURL = cfg.DefaultBaseURL
	}
	chatPath := cfg.ChatPath
	if chatPath == "" {
		chatPath = "/v1/chat/completions"
	}
	if conn.Provider != "xai" {
		baseURL = domain.StripVersionSuffix(baseURL)
	}
	return baseURL + chatPath
}

// === Test Model ===

func apiTestModel(store port.CredentialStore, providers port.ProviderRegistry, registries ...port.ImageProviderRegistry) gin.HandlerFunc {
	var imageProviders port.ImageProviderRegistry
	if len(registries) > 0 {
		imageProviders = registries[0]
	}
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Model string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
			c.JSON(400, gin.H{"error": "model is required"})
			return
		}

		// Verify ownership before making any upstream calls with the connection's credentials.
		conn, ok := requireTenantOwnsConnection(c, store, id)
		if !ok {
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
			_ = store.UpdateConnection(conn)
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
			if imageProviders != nil {
				if imageProvider := imageProviders.GetImageProvider(provider); imageProvider != nil {
					capabilities := imageProvider.Capabilities(modelName)
					if capabilities.Generate || capabilities.Edit {
						result := testImageProviderAPI(conn, domain.GetProviderConfig(provider))
						if result.OK {
							c.JSON(http.StatusOK, gin.H{"status": "ok", "model": modelName, "response": result.Response})
						} else {
							c.JSON(http.StatusOK, gin.H{"status": "error", "message": result.Error})
						}
						return
					}
				}
			}
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
