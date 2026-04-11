package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

const tokenExpiryBuffer = 5 * time.Minute

// TokenRefreshService handles automatic token refresh for Kiro connections.
type TokenRefreshService struct {
	store port.CredentialStore
}

// NewTokenRefreshService creates a new TokenRefreshService.
func NewTokenRefreshService(store port.CredentialStore) *TokenRefreshService {
	return &TokenRefreshService{store: store}
}

// NeedsRefresh checks if a connection's token is expiring soon.
func (s *TokenRefreshService) NeedsRefresh(conn *domain.ProviderConnection) bool {
	if conn.ExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, conn.ExpiresAt)
	if err != nil {
		return false
	}
	return time.Until(expiresAt) < tokenExpiryBuffer
}

// Refresh performs token refresh for a connection (Kiro or OpenAI).
func (s *TokenRefreshService) Refresh(conn *domain.ProviderConnection) (*domain.ProviderConnection, error) {
	if conn.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	// OpenAI OAuth refresh
	if conn.Provider == "openai" {
		log.Printf("[TOKEN] Refreshing OpenAI token for %s", conn.Name)
		return s.refreshOpenAI(conn)
	}

	// Qwen OAuth refresh
	if conn.Provider == "qwen" {
		log.Printf("[TOKEN] Refreshing Qwen token for %s", conn.Name)
		return s.refreshQwen(conn)
	}

	// Kiro refresh
	authMethod := getStringFromMap(conn.ProviderSpecificData, "authMethod")
	clientID := getStringFromMap(conn.ProviderSpecificData, "clientId")
	clientSecret := getStringFromMap(conn.ProviderSpecificData, "clientSecret")
	region := getStringFromMap(conn.ProviderSpecificData, "region")
	if region == "" {
		region = "us-east-1"
	}

	var result *TokenResult
	var err error

	// AWS SSO OIDC refresh (Builder ID or IDC)
	if clientID != "" && clientSecret != "" {
		log.Printf("[TOKEN] Refreshing via SSO OIDC (method=%s)", authMethod)
		result, err = RefreshTokenSSO(conn.RefreshToken, clientID, clientSecret, region)
	} else {
		// Social auth refresh (Google/GitHub)
		log.Printf("[TOKEN] Refreshing via social auth (method=%s)", authMethod)
		result, err = RefreshTokenSocial(conn.RefreshToken)
	}

	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	// Update connection
	conn.AccessToken = result.AccessToken
	if result.RefreshToken != "" {
		conn.RefreshToken = result.RefreshToken
	}
	conn.ExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	conn.ExpiresIn = result.ExpiresIn

	// Update profileArn if returned
	if result.ProfileArn != "" {
		if conn.ProviderSpecificData == nil {
			conn.ProviderSpecificData = make(map[string]interface{})
		}
		conn.ProviderSpecificData["profileArn"] = result.ProfileArn
	}

	conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	return conn, nil
}

// refreshOpenAI refreshes OpenAI OAuth token.
func (s *TokenRefreshService) refreshOpenAI(conn *domain.ProviderConnection) (*domain.ProviderConnection, error) {
	tokens, err := RefreshOpenAIToken(conn.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("openai refresh failed: %w", err)
	}

	conn.AccessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		conn.RefreshToken = tokens.RefreshToken
	}

	expiresIn := tokens.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}
	conn.ExpiresIn = expiresIn
	conn.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339)
	conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	return conn, nil
}

// refreshQwen refreshes Qwen OAuth token.
func (s *TokenRefreshService) refreshQwen(conn *domain.ProviderConnection) (*domain.ProviderConnection, error) {
	tokens, err := RefreshQwenToken(conn.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("qwen refresh failed: %w", err)
	}

	conn.AccessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		conn.RefreshToken = tokens.RefreshToken
	}

	expiresIn := tokens.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}
	conn.ExpiresIn = expiresIn
	conn.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339)
	conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	return conn, nil
}

// CheckAndRefresh checks if token needs refresh and refreshes if needed.
// Returns updated credentials.
func (s *TokenRefreshService) CheckAndRefresh(conn *domain.ProviderConnection) (*domain.ProviderConnection, error) {
	if !s.NeedsRefresh(conn) {
		return conn, nil
	}

	log.Printf("[TOKEN] Token expiring soon for %s, refreshing...", conn.Name)

	updated, err := s.Refresh(conn)
	if err != nil {
		return conn, fmt.Errorf("auto-refresh failed: %w", err)
	}

	// Persist to DB
	if err := s.store.UpdateConnection(updated); err != nil {
		log.Printf("[TOKEN] Failed to persist refreshed token: %s", err)
	} else {
		log.Printf("[TOKEN] Token refreshed and persisted for %s", conn.Name)
	}

	return updated, nil
}

// ExtractEmailFromJWT extracts email from a JWT access token.
func ExtractEmailFromJWT(accessToken string) string {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return ""
	}

	// Add padding if needed
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		// Try standard encoding
		decoded, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return ""
		}
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}

	// Try email, preferred_username, sub
	for _, key := range []string{"email", "preferred_username", "sub"} {
		if v, ok := claims[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
