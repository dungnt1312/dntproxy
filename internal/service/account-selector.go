package service

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// AccountSelector manages multi-account selection with weighted random and cooldown.
type AccountSelector struct {
	store          port.CredentialStore
	tokenRefresh   *auth.TokenRefreshService
	rotationStates sync.Map // strategy key -> *int32
}

type AccountSelectionErrorKind string

const (
	SelectionErrNoActiveCredentials AccountSelectionErrorKind = "no_active_credentials"
	SelectionErrUnsupportedModel    AccountSelectionErrorKind = "unsupported_model"
	SelectionErrNoAllowedConnection AccountSelectionErrorKind = "no_allowed_connection"
	SelectionErrRateLimited         AccountSelectionErrorKind = "rate_limited"
	SelectionErrModelLocked         AccountSelectionErrorKind = "model_locked"
	SelectionErrUnavailable         AccountSelectionErrorKind = "unavailable"

	// DefaultWeight is the weight assigned to connections with zero/unset weight.
	DefaultWeight = 100

	ConnectionStrategyWeightedRandom   = "weighted-random"
	ConnectionStrategyPriorityFallback = "priority-fallback"
	ConnectionStrategyRoundRobin       = "round-robin"
	ConnectionStrategyFillFirst        = "fill-first"
)

type AccountSelectionError struct {
	Kind     AccountSelectionErrorKind
	Provider string
	Model    string
}

func (e *AccountSelectionError) Error() string {
	switch e.Kind {
	case SelectionErrNoActiveCredentials:
		return fmt.Sprintf("no active credentials for provider: %s", e.Provider)
	case SelectionErrUnsupportedModel:
		return fmt.Sprintf("no accounts support model %q for provider: %s", e.Model, e.Provider)
	case SelectionErrNoAllowedConnection:
		return fmt.Sprintf("no allowed connections for provider: %s", e.Provider)
	case SelectionErrRateLimited:
		return fmt.Sprintf("all accounts rate limited for provider: %s", e.Provider)
	case SelectionErrModelLocked:
		return fmt.Sprintf("all accounts locked for model %q on provider: %s", e.Model, e.Provider)
	default:
		return fmt.Sprintf("no available accounts for provider: %s", e.Provider)
	}
}

func IsAccountSelectionError(err error, kind AccountSelectionErrorKind) bool {
	var selErr *AccountSelectionError
	if !errors.As(err, &selErr) {
		return false
	}
	return selErr.Kind == kind
}

// NewAccountSelector creates a new AccountSelector.
func NewAccountSelector(store port.CredentialStore) *AccountSelector {
	return &AccountSelector{
		store:        store,
		tokenRefresh: auth.NewTokenRefreshService(store),
	}
}

// SelectCredentialsForModel selects account based on model string (with optional @connectionId).
// If model string contains @connectionId, it will pin to that specific connection.
// Otherwise, it uses the existing weighted random selection logic.
//
// tenantID filters connections to those owned by the tenant (empty = legacy).
func (s *AccountSelector) SelectCredentialsForModel(
	modelStr string,
	excludeIDs map[string]bool,
	allowedConnectionIDs []string,
	tenantID string,
) (*domain.Credentials, error) {
	parsed, err := ParseModelString(modelStr)
	if err != nil {
		return nil, fmt.Errorf("parse model: %w", err)
	}

	// If pinned to specific connection
	if parsed.ConnectionID != "" {
		// Pass only the model name (without @connectionId) to selectPinnedConnection
		return s.selectPinnedConnection(parsed.Provider, parsed.Model, parsed.ConnectionID, excludeIDs, tenantID)
	}

	// Otherwise use existing weighted random logic
	return s.SelectCredentials(parsed.Provider, excludeIDs, parsed.Model, allowedConnectionIDs, tenantID)
}

// selectPinnedConnection returns a specific connection by ID.
// It still respects rate limits and model locks, but bypasses weighted random selection.
func (s *AccountSelector) selectPinnedConnection(
	provider string,
	model string,
	connectionID string,
	excludeIDs map[string]bool,
	tenantID string,
) (*domain.Credentials, error) {
	// Skip if already excluded (failed in this request)
	if excludeIDs != nil && excludeIDs[connectionID] {
		return nil, &AccountSelectionError{
			Kind:     SelectionErrUnavailable,
			Provider: provider,
			Model:    model,
		}
	}

	connections, err := s.getActiveConnectionsForTenant(provider, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get connections: %w", err)
	}

	for _, conn := range connections {
		if conn.ID != connectionID {
			continue
		}

		// Check if supports model (model should NOT contain @connectionId here)
		// Exception: OpenAI OAuth connections accept any ChatGPT model slug
		isOpenAIOAuth := conn.Provider == "openai" && conn.AuthType == "oauth"
		if !isOpenAIOAuth && !conn.SupportsModel(model) {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrUnsupportedModel,
				Provider: provider,
				Model:    model,
			}
		}

		// Check if rate limited
		if domain.IsAccountUnavailable(conn.RateLimitedUntil) {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrRateLimited,
				Provider: provider,
				Model:    model,
			}
		}

		// Check if model locked
		if domain.IsModelLockActive(conn.ModelLocks, model) {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrModelLocked,
				Provider: provider,
				Model:    model,
			}
		}

		fresh, err := s.ensureFreshOAuth(&conn)
		if err != nil {
			return nil, err
		}

		return shared.ConnectionToCredentials(fresh), nil
	}

	return nil, &AccountSelectionError{
		Kind:     SelectionErrNoActiveCredentials,
		Provider: provider,
		Model:    model,
	}
}

// SelectCredentials returns a weighted-random available connection for a provider+model,
// filtered by excludeIDs and optional allowedConnectionIDs restriction.
//
// tenantID filters connections to those owned by the tenant (empty = legacy).
func (s *AccountSelector) SelectCredentials(
	provider string,
	excludeIDs map[string]bool,
	model string,
	allowedConnectionIDs []string,
	tenantIDs ...string,
) (*domain.Credentials, error) {
	// Keep the pre-multi-tenant call shape compatible for internal callers and
	// tests while applying a tenant filter when supplied.
	tenantID := ""
	if len(tenantIDs) > 0 {
		tenantID = tenantIDs[0]
	}
	connections, err := s.getActiveConnectionsForTenant(provider, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get connections: %w", err)
	}

	if len(connections) == 0 {
		return nil, &AccountSelectionError{
			Kind:     SelectionErrNoActiveCredentials,
			Provider: provider,
			Model:    model,
		}
	}

	// Filter to only available connections
	var available []domain.ProviderConnection
	allowedByPolicyCount := 0
	supportedCount := 0
	rateLimitedSupportedCount := 0
	lockedSupportedCount := 0
	nonExcludedCount := 0

	for _, conn := range connections {
		// Skip excluded connections (already failed in this request)
		if excludeIDs != nil && excludeIDs[conn.ID] {
			continue
		}
		nonExcludedCount++

		// Skip connections not in allowedConnectionIDs (combo restriction)
		if len(allowedConnectionIDs) > 0 {
			allowed := false
			for _, id := range allowedConnectionIDs {
				if conn.ID == id {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}
		allowedByPolicyCount++

		// Skip connections that don't support this model.
		// Exception: OpenAI OAuth connections (ChatGPT tokens) use the Codex Responses API
		// which accepts any ChatGPT model slug — the upstream validates the model.
		isOpenAIOAuth := conn.Provider == "openai" && conn.AuthType == "oauth"
		if !isOpenAIOAuth && !conn.SupportsModel(model) {
			continue
		}
		supportedCount++

		// Skip rate-limited connections
		if domain.IsAccountUnavailable(conn.RateLimitedUntil) {
			rateLimitedSupportedCount++
			continue
		}

		// Skip model-locked connections
		if domain.IsModelLockActive(conn.ModelLocks, model) {
			lockedSupportedCount++
			continue
		}

		available = append(available, conn)
	}

	if len(available) == 0 {
		// Return appropriate error based on what filtered them out
		if nonExcludedCount == 0 {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrUnavailable,
				Provider: provider,
				Model:    model,
			}
		}
		if supportedCount == 0 {
			if len(allowedConnectionIDs) > 0 && allowedByPolicyCount == 0 {
				return nil, &AccountSelectionError{
					Kind:     SelectionErrNoAllowedConnection,
					Provider: provider,
					Model:    model,
				}
			}
			return nil, &AccountSelectionError{
				Kind:     SelectionErrUnsupportedModel,
				Provider: provider,
				Model:    model,
			}
		}
		if rateLimitedSupportedCount == supportedCount {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrRateLimited,
				Provider: provider,
				Model:    model,
			}
		}
		if lockedSupportedCount == supportedCount {
			return nil, &AccountSelectionError{
				Kind:     SelectionErrModelLocked,
				Provider: provider,
				Model:    model,
			}
		}
		return nil, &AccountSelectionError{
			Kind:     SelectionErrUnavailable,
			Provider: provider,
			Model:    model,
		}
	}

	selected := s.selectConnection(available, provider, model, allowedConnectionIDs)

	updated, err := s.ensureFreshOAuth(selected)
	if err != nil {
		return nil, err
	}
	selected = updated

	return shared.ConnectionToCredentials(selected), nil
}

func (s *AccountSelector) selectConnection(available []domain.ProviderConnection, provider string, model string, allowedConnectionIDs []string) *domain.ProviderConnection {
	strategy := s.connectionStrategyFor(provider)
	pool := available
	switch strategy {
	case ConnectionStrategyPriorityFallback, ConnectionStrategyFillFirst:
		pool = filterByLowestPriority(available)
	}
	switch strategy {
	case ConnectionStrategyPriorityFallback:
		return priorityFallbackSelect(pool)
	case ConnectionStrategyRoundRobin:
		return s.roundRobinSelect(pool, provider, model, allowedConnectionIDs)
	case ConnectionStrategyFillFirst:
		return fillFirstSelect(pool)
	default:
		return weightedRandomSelect(pool)
	}
}

func (s *AccountSelector) connectionStrategyFor(provider string) string {
	settings, err := s.store.GetSettings()
	if err != nil || settings == nil {
		return ConnectionStrategyWeightedRandom
	}
	if m := settings.ConnectionStrategies; m != nil {
		if v, ok := m[provider]; ok && v != "" {
			return v
		}
	}
	return s.connectionStrategy()
}

func (s *AccountSelector) connectionStrategy() string {
	settings, err := s.store.GetSettings()
	if err != nil || settings == nil || settings.ConnectionStrategy == "" {
		return ConnectionStrategyWeightedRandom
	}
	switch settings.ConnectionStrategy {
	case ConnectionStrategyWeightedRandom, ConnectionStrategyPriorityFallback, ConnectionStrategyRoundRobin, ConnectionStrategyFillFirst:
		return settings.ConnectionStrategy
	default:
		return ConnectionStrategyWeightedRandom
	}
}

func priorityFallbackSelect(available []domain.ProviderConnection) *domain.ProviderConnection {
	if len(available) == 1 {
		return &available[0]
	}
	sorted := append([]domain.ProviderConnection(nil), available...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return normalizedWeight(sorted[i].Weight) > normalizedWeight(sorted[j].Weight)
	})
	return &sorted[0]
}

func (s *AccountSelector) roundRobinSelect(available []domain.ProviderConnection, provider string, model string, allowedConnectionIDs []string) *domain.ProviderConnection {
	if len(available) == 1 {
		return &available[0]
	}
	sorted := append([]domain.ProviderConnection(nil), available...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return normalizedWeight(sorted[i].Weight) > normalizedWeight(sorted[j].Weight)
	})

	key := connectionRotationKey(provider, model, allowedConnectionIDs)
	val, _ := s.rotationStates.LoadOrStore(key, new(int32))
	counter := val.(*int32)
	offset := int(atomic.LoadInt32(counter)) % len(sorted)
	if offset < 0 {
		offset += len(sorted)
	}
	return &sorted[offset]
}

func (s *AccountSelector) AdvanceConnectionRotation(provider string, model string, allowedConnectionIDs []string) {
	if s.connectionStrategyFor(provider) != ConnectionStrategyRoundRobin {
		return
	}
	key := connectionRotationKey(provider, model, allowedConnectionIDs)
	val, ok := s.rotationStates.Load(key)
	if !ok {
		return
	}
	counter := val.(*int32)
	atomic.AddInt32(counter, 1)
}

func connectionRotationKey(provider string, model string, allowedConnectionIDs []string) string {
	ids := append([]string(nil), allowedConnectionIDs...)
	sort.Strings(ids)
	if len(ids) == 0 {
		return provider + "/" + model + "|all"
	}
	return provider + "/" + model + "|" + strings.Join(ids, ",")
}

// filterByLowestPriority returns connections from the lowest priority group only.
// Lower Priority value = higher precedence. When all remaining connections are
// in the same priority, this is a pass-through. When there are multiple priority
// levels, only the lowest-numbered connections survive — higher-priority-number
// (lower precedence) connections are only used as fallback after all lower-priority
// connections have been excluded, rate-limited, or locked.
func filterByLowestPriority(conns []domain.ProviderConnection) []domain.ProviderConnection {
	if len(conns) <= 1 {
		return conns
	}
	minPriority := conns[0].Priority
	for _, c := range conns[1:] {
		if c.Priority < minPriority {
			minPriority = c.Priority
		}
	}
	filtered := make([]domain.ProviderConnection, 0, len(conns))
	for _, c := range conns {
		if c.Priority == minPriority {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// fillFirstSelect picks the single best candidate deterministically.
// Sort order: weight DESC → ID ASC for stability.
// This strategy is "sticky" — it always returns the same connection from the
// available pool (same priority level after pre-filter), maximizing provider-side
// cache hit rate.
func fillFirstSelect(available []domain.ProviderConnection) *domain.ProviderConnection {
	if len(available) == 1 {
		return &available[0]
	}
	sorted := append([]domain.ProviderConnection(nil), available...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if normalizedWeight(sorted[i].Weight) != normalizedWeight(sorted[j].Weight) {
			return normalizedWeight(sorted[i].Weight) > normalizedWeight(sorted[j].Weight)
		}
		return sorted[i].ID < sorted[j].ID
	})
	return &sorted[0]
}

// weightedRandomSelect picks a connection from the available list using weighted random.
// Weight represents probability: higher weight = more likely to be selected.
// Connections with weight <= 0 are treated as DefaultWeight (100).
func weightedRandomSelect(available []domain.ProviderConnection) *domain.ProviderConnection {
	if len(available) == 1 {
		return &available[0]
	}

	totalWeight := 0
	for _, conn := range available {
		totalWeight += normalizedWeight(conn.Weight)
	}

	r := rand.Intn(totalWeight)
	cumulative := 0
	for i := range available {
		w := normalizedWeight(available[i].Weight)
		cumulative += w
		if r < cumulative {
			return &available[i]
		}
	}

	// Fallback (should never reach here)
	return &available[0]
}

func normalizedWeight(weight int) int {
	if weight <= 0 {
		return DefaultWeight
	}
	return weight
}

func connectionDisableCooling(conn *domain.ProviderConnection) bool {
	if conn == nil || conn.ProviderSpecificData == nil {
		return false
	}
	v, ok := conn.ProviderSpecificData["disableCooling"].(bool)
	return ok && v
}

func clampCooldownMs(ms int, maxSeconds int) int {
	if maxSeconds <= 0 || ms <= 0 {
		return ms
	}
	return min(ms, maxSeconds*1000)
}

// MarkUnavailable marks a connection as unavailable with cooldown.
func (s *AccountSelector) MarkUnavailable(connectionID string, status int, errorText string, model string) error {
	return s.store.Update(func(cfg *domain.AppConfig) {
		var conn *domain.ProviderConnection
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == connectionID {
				conn = &cfg.ProviderConnections[i]
				break
			}
		}
		if conn == nil {
			return
		}

		settings := cfg.Settings
		if !settings.CooldownOn() || connectionDisableCooling(conn) {
			conn.LastError = errorText
			conn.LastErrorAt = time.Now().UTC().Format(time.RFC3339)
			return
		}

		fb := domain.CheckFallbackError(status, errorText, conn.BackoffLevel)
		if !fb.ShouldFallback {
			return
		}
		if domain.ClassifyUpstream(status, errorText) == domain.UpstreamTransient && settings.TransientCooldownSeconds > 0 {
			fb.CooldownMs = settings.TransientCooldownSeconds * 1000
			// An explicit operator override is an account-level circuit breaker.
			fb.ModelOnly = false
		}
		fb.CooldownMs = clampCooldownMs(fb.CooldownMs, settings.MaxCooldownSeconds)

		if fb.CooldownMs > 0 {
			until := domain.CooldownUntil(fb.CooldownMs)
			if !fb.ModelOnly {
				conn.RateLimitedUntil = until
				conn.BackoffLevel = fb.NewBackoffLevel
			}
			conn.LastError = errorText
			conn.LastErrorAt = time.Now().UTC().Format(time.RFC3339)

			if settings.ModelLockOn() && model != "" {
				if conn.ModelLocks == nil {
					conn.ModelLocks = make(map[string]string)
				}
				conn.ModelLocks[model] = until
			}
		}
	})
}

// ClearError resets a connection's error state after a successful request.
func (s *AccountSelector) ClearError(connectionID string, model string) error {
	return s.store.Update(func(cfg *domain.AppConfig) {
		var conn *domain.ProviderConnection
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == connectionID {
				conn = &cfg.ProviderConnections[i]
				break
			}
		}
		if conn == nil {
			return
		}

		conn.RateLimitedUntil = ""
		conn.BackoffLevel = 0
		conn.LastError = ""
		conn.LastErrorAt = ""

		if model != "" && conn.ModelLocks != nil {
			delete(conn.ModelLocks, model)

			// Clean up expired model locks (including corrupt timestamps)
			now := time.Now()
			for k, expiry := range conn.ModelLocks {
				t, err := time.Parse(time.RFC3339, expiry)
				if err != nil || t.Before(now) {
					delete(conn.ModelLocks, k)
				}
			}
		}
	})
}

// getActiveConnectionsForTenant returns active connections for a provider,
// filtered to those owned by tenantID. When tenantID is empty (legacy mode),
// returns all active connections for the provider.
func (s *AccountSelector) getActiveConnectionsForTenant(provider, tenantID string) ([]domain.ProviderConnection, error) {
	conns, err := s.store.GetActiveConnections(provider)
	if err != nil {
		return nil, err
	}
	return domain.FilterConnectionsByTenant(conns, tenantID), nil
}

func (s *AccountSelector) RefreshCredentialsForOAuth(connectionID string) (*domain.Credentials, error) {
	conn, err := s.store.GetConnectionByID(connectionID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("connection not found: %s", connectionID)
	}
	if conn.AuthType != "oauth" || strings.TrimSpace(conn.RefreshToken) == "" {
		return shared.ConnectionToCredentials(conn), nil
	}
	log.Printf("[AUTH] Force-refreshing OAuth token for %s after upstream 401", conn.Name)
	refreshed, err := s.tokenRefresh.ForceRefresh(conn)
	if err != nil {
		return nil, fmt.Errorf("token refresh failed for %s: %w", conn.Name, err)
	}
	return shared.ConnectionToCredentials(refreshed), nil
}

func (s *AccountSelector) ensureFreshOAuth(conn *domain.ProviderConnection) (*domain.ProviderConnection, error) {
	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}
	if conn.AuthType != "oauth" || strings.TrimSpace(conn.RefreshToken) == "" {
		return conn, nil
	}
	expired := auth.IsAccessTokenExpired(conn.ExpiresAt) || strings.TrimSpace(conn.ExpiresAt) == ""
	if expired {
		log.Printf("[AUTH] Refreshing expired OAuth token for %s", conn.Name)
		refreshed, err := s.tokenRefresh.ForceRefresh(conn)
		if err != nil {
			return nil, fmt.Errorf("token refresh failed for %s: %w", conn.Name, err)
		}
		log.Printf("[AUTH] Token refreshed successfully for %s", conn.Name)
		return refreshed, nil
	}
	if s.tokenRefresh.ShouldProactivelyRefresh(conn) {
		log.Printf("[AUTH] Proactively refreshing OAuth token for %s", conn.Name)
		refreshed, err := s.tokenRefresh.ForceRefresh(conn)
		if err != nil {
			log.Printf("[AUTH] Proactive refresh failed for %s, using current token: %v", conn.Name, err)
			return conn, nil
		}
		log.Printf("[AUTH] Token refreshed successfully for %s", conn.Name)
		return refreshed, nil
	}
	return conn, nil
}
