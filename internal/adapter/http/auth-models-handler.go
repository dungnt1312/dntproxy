package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// === Fetch Models from Provider API ===

func apiFetchConnectionModels(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		conn, ok := requireTenantOwnsConnection(c, store, id)
		if !ok {
			return
		}

		if conn.Provider != "openai" && conn.Provider != "openai-compatible" {
			// Fallback: return default models from provider config
			cfg := domain.GetProviderConfig(conn.Provider)
			c.JSON(200, gin.H{
				"provider": conn.Provider,
				"name":     conn.Name,
				"models":   cfg.DefaultModels,
				"source":   "provider-config",
				"note":     fmt.Sprintf("Live fetching not supported for %s, returning defaults", cfg.Name),
			})
			return
		}

		baseURL := conn.BaseURL
		if baseURL == "" && conn.Provider == "openai" {
			if conn.AuthType == "oauth" {
				baseURL = "https://chatgpt.com/backend-api"
			} else {
				baseURL = "https://api.openai.com"
			}
		}
		if baseURL == "" {
			c.JSON(400, gin.H{"error": "No base URL configured for this connection"})
			return
		}
		baseURL = domain.StripVersionSuffix(baseURL)

		var modelsURL string
		// chatgpt.com/backend-api/models instead of /v1/models
		if conn.Provider == "openai" && conn.AuthType == "oauth" {
			modelsURL = baseURL + "/models"
		} else {
			modelsURL = baseURL + "/v1/models"
		}

		req, _ := http.NewRequest("GET", modelsURL, nil)
		if conn.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+conn.APIKey)
		} else if conn.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
		}

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(502, gin.H{"error": "Failed to reach provider: " + err.Error()})
			return
		}

		if (resp.StatusCode == 401 || resp.StatusCode == 403) && conn.Provider == "openai" && conn.RefreshToken != "" {
			resp.Body.Close()
			updatedConn, refErr := refreshOpenAIConnection(conn, store)
			if refErr == nil {
				conn = updatedConn
				req, _ = http.NewRequest("GET", modelsURL, nil)
				req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
				resp, err = client.Do(req)
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

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
			c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Provider returned %d: %s", resp.StatusCode, string(body))})
			return
		}

		var modelsResp map[string]interface{}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		if err := json.Unmarshal(body, &modelsResp); err != nil {
			c.JSON(500, gin.H{"error": "Failed to parse models response"})
			return
		}

		var modelIDs []string
		if data, ok := modelsResp["data"].([]interface{}); ok {
			// standard OpenAI API format
			for _, mInter := range data {
				if m, ok := mInter.(map[string]interface{}); ok {
					if id, ok := m["id"].(string); ok {
						modelIDs = append(modelIDs, id)
					}
				}
			}
		} else if models, ok := modelsResp["models"].([]interface{}); ok {
			// ChatGPT backend API format
			for _, mInter := range models {
				if m, ok := mInter.(map[string]interface{}); ok {
					if slug, ok := m["slug"].(string); ok {
						modelIDs = append(modelIDs, slug)
					}
				}
			}
		}
		sort.Strings(modelIDs)

		c.JSON(200, gin.H{"models": modelIDs})
	}
}
