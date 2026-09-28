package autologin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/google/uuid"
)

// openAIProfile is the account info decoded from the access token claims.
type openAIProfile struct {
	Email     string
	AccountID string
	PlanType  string
}

// decodeOpenAIProfile extracts email / ChatGPT account / plan from the
// OpenAI access token's namespaced claims.
func decodeOpenAIProfile(accessToken string) openAIProfile {
	claims := jwtPayload(accessToken)
	if claims == nil {
		return openAIProfile{}
	}
	profile := mapClaim(claims, "https://api.openai.com/profile")
	authClaims := mapClaim(claims, "https://api.openai.com/auth")
	email, _ := profile["email"].(string)
	accountID, _ := authClaims["chatgpt_account_id"].(string)
	plan, _ := authClaims["chatgpt_plan_type"].(string)
	return openAIProfile{Email: email, AccountID: accountID, PlanType: plan}
}

func jwtPayload(token string) map[string]interface{} {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil
	}
	return claims
}

func mapClaim(claims map[string]interface{}, key string) map[string]interface{} {
	if v, ok := claims[key].(map[string]interface{}); ok {
		return v
	}
	return map[string]interface{}{}
}

// updateConfig applies fn to the full config atomically (single store lock).
func (j *job) updateConfig(fn func(cfg *domain.AppConfig)) error {
	return j.store.Update(fn)
}

// upsertConnection stores the freshly obtained tokens. An existing openai
// OAuth connection with the same email (same tenant) keeps its identity —
// id, name, weight, supported models — and only the credentials are replaced;
// genuinely new accounts are appended.
func (j *job) upsertConnection(tokens *auth.OpenAITokenResponse, email string, profile openAIProfile, tenantID string) (string, bool, error) {
	expiresIn := tokens.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	expiresAt := now.Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339)

	var connID string
	replaced := false

	err := j.updateConfig(func(cfg *domain.AppConfig) {
		for i := range cfg.ProviderConnections {
			c := &cfg.ProviderConnections[i]
			if c.Provider != "openai" || c.AuthType != "oauth" || c.TenantID != tenantID {
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(c.Email), email) {
				continue
			}
			c.AccessToken = tokens.AccessToken
			c.RefreshToken = tokens.RefreshToken
			c.ExpiresAt = expiresAt
			c.ExpiresIn = expiresIn
			c.IsActive = true
			c.TestStatus = "active"
			c.LastError = ""
			c.LastErrorAt = ""
			c.UpdatedAt = nowStr
			c.ProviderSpecificData = map[string]interface{}{
				"authMethod":       "oauth",
				"idToken":          tokens.IDToken,
				"chatgptAccountId": profile.AccountID,
				"chatgptPlanType":  profile.PlanType,
			}
			connID = c.ID
			replaced = true
			return
		}

		connID = uuid.New().String()
		cfg.ProviderConnections = append(cfg.ProviderConnections, domain.ProviderConnection{
			ID:           connID,
			Provider:     "openai",
			AuthType:     "oauth",
			Name:         email,
			Weight:       100,
			IsActive:     true,
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			ExpiresAt:    expiresAt,
			ExpiresIn:    expiresIn,
			Email:        email,
			TestStatus:   "active",
			ProviderSpecificData: map[string]interface{}{
				"authMethod":       "oauth",
				"idToken":          tokens.IDToken,
				"chatgptAccountId": profile.AccountID,
				"chatgptPlanType":  profile.PlanType,
			},
			CreatedAt: nowStr,
			UpdatedAt: nowStr,
			TenantID:  tenantID,
		})
	})
	if err != nil {
		return "", false, fmt.Errorf("persist connection: %w", err)
	}
	return connID, replaced, nil
}
