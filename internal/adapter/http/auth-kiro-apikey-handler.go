package http

import (
	"fmt"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/kiro"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// kiroAPIKeyExpiryDays is the stored horizon for a headless Kiro API key.
// The key never expires and has no refresh token, so a long horizon keeps the
// proactive refresh path (which requires a refresh token anyway) from firing.
const kiroAPIKeyExpiryDays = 365

// authKiroAPIKey enrolls a long-lived Kiro/CodeWhisperer API key.
//
// The key is validated against ListAvailableModels and then stored verbatim as
// a bearer credential. Unlike the OAuth flows there is no token exchange, no
// refresh token, and no profileArn — Kiro resolves the profile from the key's
// own account, and injecting a shared default ARN would make CodeWhisperer
// answer 403 "bearer token invalid".
func authKiroAPIKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			APIKey          string   `json:"apiKey"`
			Region          string   `json:"region,omitempty"`
			Name            string   `json:"name,omitempty"`
			SupportedModels []string `json:"supportedModels,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}

		result, err := auth.ValidateKiroAPIKey(req.APIKey, req.Region)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Routing uses the same default model set as every other Kiro flow. The
		// ids returned by ListAvailableModels are an upstream catalog and do not
		// necessarily match the registry naming, so they are reported back for
		// display but not used to gate routing.
		supportedModels := req.SupportedModels
		if len(supportedModels) == 0 {
			cfg, _ := store.Load()
			var settings *domain.Settings
			if cfg != nil {
				settings = &cfg.Settings
			}
			supportedModels = domain.GetDefaultConnectionModels(settings, "kiro")
		}

		now := time.Now().UTC().Format(time.RFC3339)
		conn := domain.ProviderConnection{
			ID:       uuid.New().String(),
			Provider: "kiro",
			AuthType: "apikey",
			Weight:   100,
			IsActive: true,
			// Stored in both slots: the executor reads APIKey, while the
			// generic connection views expect AuthType=apikey to carry one.
			APIKey:          result.APIKey,
			AccessToken:     result.APIKey,
			ExpiresAt:       time.Now().AddDate(0, 0, kiroAPIKeyExpiryDays).UTC().Format(time.RFC3339),
			ExpiresIn:       kiroAPIKeyExpiryDays * 24 * 3600,
			TestStatus:      "active",
			SupportedModels: supportedModels,
			ProviderSpecificData: map[string]interface{}{
				"authMethod": kiro.AuthMethodAPIKey,
				"provider":   "Kiro API Key",
				"region":     result.Region,
			},
			CreatedAt: now,
			UpdatedAt: now,
			TenantID:  GetTenantID(c),
		}

		if err := store.Update(func(cfg *domain.AppConfig) {
			name := req.Name
			if name == "" {
				count := 0
				for _, x := range cfg.ProviderConnections {
					if x.Provider == "kiro" {
						count++
					}
				}
				name = fmt.Sprintf("Kiro API Key %d", count+1)
			}
			conn.Name = name
			cfg.ProviderConnections = append(cfg.ProviderConnections, conn)
		}); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"id":     conn.ID,
			"name":   conn.Name,
			"region": result.Region,
			"models": result.Models,
		})
	}
}
