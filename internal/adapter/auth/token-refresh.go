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
	"golang.org/x/sync/singleflight"
)

const tokenExpiryBuffer = 5 * time.Minute

// TokenRefreshService handles automatic token refresh for Kiro connections.
type TokenRefreshService struct {
	store port.CredentialStore
	sfg   singleflight.Group
}

// NewTokenRefreshService creates a new TokenRefreshService.
func NewTokenRefreshService(store port.CredentialStore) *TokenRefreshService {
	return &TokenRefreshService{store: store}
}

// NeedsRefresh checks if a connection's token is expiring soon.
func (s *TokenRefreshService) NeedsRefresh(conn *domain.ProviderConnection) bool {
	return s.shouldRefresh(conn, false)
}

// ShouldProactivelyRefresh is used by the background OAuth refresh loop (includes already-expired tokens).
func (s *TokenRefreshService) ShouldProactivelyRefresh(conn *domain.ProviderConnection) bool {
	return s.shouldRefresh(conn, true)
}

func (s *TokenRefreshService) shouldRefresh(conn *domain.ProviderConnection, includeExpired bool) bool {
	if conn == nil || strings.TrimSpace(conn.RefreshToken) == "" {
		return false
	}
	if conn.ExpiresAt == "" {
		return includeExpired
	}
	expiresAt, ok := ParseExpiresAt(conn.ExpiresAt)
	if !ok {
		return includeExpired
	}
	if includeExpired && !expiresAt.After(time.Now().UTC()) {
		return true
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

	// xAI OAuth refresh
	if conn.Provider == "xai" {
		log.Printf("[TOKEN] Refreshing xAI token for %s", conn.Name)
		return s.refreshXAI(conn)
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

// refreshXAI refreshes xAI OAuth token.
func (s *TokenRefreshService) refreshXAI(conn *domain.ProviderConnection) (*domain.ProviderConnection, error) {
	tokenEndpoint := getStringFromMap(conn.ProviderSpecificData, "tokenEndpoint")
	tokens, err := RefreshXAIToken(conn.RefreshToken, tokenEndpoint)
	if err != nil {
		return nil, fmt.Errorf("xai refresh failed: %w", err)
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

	if conn.ProviderSpecificData == nil {
		conn.ProviderSpecificData = make(map[string]interface{})
	}
	if tokens.IDToken != "" {
		conn.ProviderSpecificData["idToken"] = tokens.IDToken
	}
	if tokens.Subject != "" {
		conn.ProviderSpecificData["subject"] = tokens.Subject
	}
	if tokenEndpoint != "" {
		conn.ProviderSpecificData["tokenEndpoint"] = tokenEndpoint
	}
	if tokens.Email != "" {
		conn.Email = tokens.Email
	}

	return conn, nil
}

// CheckAndRefresh checks if token needs refresh and refreshes if needed.
// Uses singleflight to deduplicate concurrent refreshes for the same connection.
func (s *TokenRefreshService) CheckAndRefresh(conn *domain.ProviderConnection) (*domain.ProviderConnection, error) {
	return s.checkAndRefresh(conn, false)
}

// ForceRefresh refreshes OAuth credentials even when expiry is unknown or not within the buffer.
func (s *TokenRefreshService) ForceRefresh(conn *domain.ProviderConnection) (*domain.ProviderConnection, error) {
	return s.checkAndRefresh(conn, true)
}

func (s *TokenRefreshService) checkAndRefresh(conn *domain.ProviderConnection, force bool) (*domain.ProviderConnection, error) {
	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}
	if !force && !s.NeedsRefresh(conn) {
		return conn, nil
	}
	if force && strings.TrimSpace(conn.RefreshToken) == "" {
		return nil, fmt.Errorf("no refresh token available")
	}
	if !force && !s.ShouldProactivelyRefresh(conn) {
		return conn, nil
	}

	log.Printf("[TOKEN] Token expiring soon for %s, refreshing...", conn.Name)

	result, err, _ := s.sfg.Do(conn.ID, func() (interface{}, error) {
		latest, loadErr := s.store.GetConnectionByID(conn.ID)
		if loadErr != nil || latest == nil {
			latest = conn
		}
		if !force && !s.ShouldProactivelyRefresh(latest) {
			return latest, nil
		}

		updated, refreshErr := s.Refresh(latest)
		if refreshErr != nil {
			return latest, fmt.Errorf("auto-refresh failed: %w", refreshErr)
		}

		if err := s.store.UpdateConnection(updated); err != nil {
			log.Printf("[TOKEN] Failed to persist refreshed token: %s", err)
		} else {
			log.Printf("[TOKEN] Token refreshed and persisted for %s", updated.Name)
		}

		return updated, nil
	})

	if err != nil {
		return conn, err
	}
	return result.(*domain.ProviderConnection), nil
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
