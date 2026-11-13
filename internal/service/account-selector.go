package service

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/logger"
	"github.com/dungnt/dntproxy/internal/port"
)

// oauthTokenRefresher is the subset of auth.TokenRefreshService the selector
// needs. Defined as an interface so tests can stub refresh outcomes.
type oauthTokenRefresher interface {
	ForceRefresh(conn *domain.ProviderConnection) (*domain.ProviderConnection, error)
	ShouldProactivelyRefresh(conn *domain.ProviderConnection) bool
}

// AccountSelector manages multi-account selection with weighted random and cooldown.
type AccountSelector struct {
	store          port.CredentialStore
	tokenRefresh   oauthTokenRefresher
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
	// RetryAfter is the earliest moment a rejected connection becomes usable
	// again (RFC3339), when the rejection is only a cooldown. It lets the caller
	// answer with a meaningful Retry-After instead of an open-ended 429.
	RetryAfter string
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

// oauthRefreshError wraps a failed OAuth token refresh so selection code can
// distinguish a dead refresh token (upstream rejects it — the account needs
// re-auth and should be sidelined for a long window) from a transient refresh
// failure (network blip — retry another account now, cool briefly).
type oauthRefreshError struct {
	ConnectionID string
	Dead         bool
	Err          error
}

func (e *oauthRefreshError) Error() string {
	return fmt.Sprintf("oauth refresh failed for connection %s: %v", e.ConnectionID, e.Err)
}

func (e *oauthRefreshError) Unwrap() error { return e.Err }

// isDeadRefreshToken reports whether a refresh failure means the refresh token
// itself was rejected (needs re-auth) rather than a transient network error.
func isDeadRefreshToken(err error) bool {
	if err == nil {
		return false
	}
	return domain.IsAuthRevokedErrorText(err.Error())
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
				Kind:       SelectionErrRateLimited,
				Provider:   provider,
				Model:      model,
				RetryAfter: conn.RateLimitedUntil,
			}
		}

		// Check if model locked
		if lockUntil := domain.ModelLockExpiry(conn.ModelLocks, model); lockUntil != "" {
			return nil, &AccountSelectionError{
				Kind:       SelectionErrModelLocked,
				Provider:   provider,
				Model:      model,
				RetryAfter: lockUntil,
			}
		}

		fresh, err := s.ensureFreshOAuth(&conn)
		if err != nil {
			// A pinned connection whose token cannot be refreshed must not be
			// silently retried on every request. Sideline it so the operator
			// sees the failure and re-auths; the request itself still fails
			// fast (pins never spill over to another connection).
			var refreshErr *oauthRefreshError
			if errors.As(err, &refreshErr) {
				status := http.StatusServiceUnavailable
				msg := refreshErr.Err.Error()
				if refreshErr.Dead {
					status = http.StatusUnauthorized
				}
				if markErr := s.MarkUnavailable(conn.ID, status, "token refresh failed: "+msg, model); markErr != nil && logger.IsDevMode() {
					logger.Get().AddError("PROXY", "Failed marking account unavailable: %s", markErr)
				}
				return nil, &AccountSelectionError{
					Kind:     SelectionErrUnavailable,
					Provider: provider,
					Model:    model,
				}
			}
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
	// earliestRetry tracks when a rejected connection becomes usable again, so a
	// fully cooling pool can answer with a real Retry-After instead of an
	// open-ended "come back later".
	earliestRetry := ""
	earliestRetryAt := time.Time{}
	noteRetry := func(until string) {
		if until == "" {
			return
		}
		t, err := time.Parse(time.RFC3339, until)
		if err != nil {
			// Corrupt timestamps count as unavailable elsewhere; keep the raw
			// value so the caller at least sees a hint.
			if earliestRetry == "" {
				earliestRetry = until
			}
			return
		}
		if earliestRetryAt.IsZero() || t.Before(earliestRetryAt) {
			earliestRetry = until
			earliestRetryAt = t
		}
	}

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
			noteRetry(conn.RateLimitedUntil)
			continue
		}

		// Skip model-locked connections
		if lockUntil := domain.ModelLockExpiry(conn.ModelLocks, model); lockUntil != "" {
			lockedSupportedCount++
			noteRetry(lockUntil)
			continue
		}

		available = append(available, conn)
	}

	// If no candidate can serve this request, return the classification error
	// describing which filter eliminated the pool.
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
				Kind:       SelectionErrRateLimited,
				Provider:   provider,
				Model:      model,
				RetryAfter: earliestRetry,
			}
		}
		if lockedSupportedCount == supportedCount {
			return nil, &AccountSelectionError{
				Kind:       SelectionErrModelLocked,
				Provider:   provider,
				Model:      model,
				RetryAfter: earliestRetry,
			}
		}
		return nil, &AccountSelectionError{
			Kind:     SelectionErrUnavailable,
			Provider: provider,
			Model:    model,
		}
	}

	// Pick candidates in strategy order. When a candidate's OAuth token needs
	// refreshing and the refresh fails (dead refresh token or transient), the
	// connection is sidelined with an appropriate cooldown and the next
	// candidate is tried — a single stale credential must not abort the whole
	// request while healthy accounts remain.
	pool := available
	for len(pool) > 0 {
		selected := s.selectConnection(pool, provider, model, allowedConnectionIDs)
		if selected == nil {
			break
		}

		fresh, err := s.ensureFreshOAuth(selected)
		if err == nil {
			return shared.ConnectionToCredentials(fresh), nil
		}

		// A failed refresh means this connection cannot be used right now:
		// record the reason and remove it from the pool, then try the next
		// candidate. Dead refresh tokens (upstream rejects them) get the long
		// auth-revoked cooldown; transient failures a short one. Proactive
		// refresh failures are non-fatal (current token is used) and are not
		// oauthRefreshError, so they never reach this branch.
		var refreshErr *oauthRefreshError
		if errors.As(err, &refreshErr) {
			status := http.StatusServiceUnavailable
			msg := refreshErr.Err.Error()
			if refreshErr.Dead {
				status = http.StatusUnauthorized
			}
			if markErr := s.MarkUnavailable(selected.ID, status, "token refresh failed: "+msg, model); markErr != nil && logger.IsDevMode() {
				logger.Get().AddError("PROXY", "Failed marking account unavailable: %s", markErr)
			}
			pool = removeConnection(pool, selected.ID)
			continue
		}

		return nil, err
	}

	// Every candidate was tried and its refresh failed. Report unavailable so
	// the caller can surface a meaningful error instead of a raw refresh error.
	return nil, &AccountSelectionError{
		Kind:     SelectionErrUnavailable,
		Provider: provider,
		Model:    model,
	}
}

// removeConnection returns a new slice without the connection with the given id.
func removeConnection(conns []domain.ProviderConnection, id string) []domain.ProviderConnection {
	out := make([]domain.ProviderConnection, 0, len(conns)-1)
	for _, c := range conns {
		if c.ID != id {
			out = append(out, c)
		}
	}
	return out
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

		// A weak failure must not overwrite a strong one. Once an account is
		// sidelined for an exhausted balance, a suspension, or a revoked grant,
		// a later transient blip (recorded by a request that was already in
		// flight) must not replace that reason with a 2-second model lock — the
		// account would be back in rotation within seconds.
		if domain.IsStickyAccountCooldown(conn.LastError) && domain.IsAccountUnavailable(conn.RateLimitedUntil) {
			if !domain.IsStickyAccountCooldown(errorText) {
				if settings.ModelLockOn() && model != "" && fb.CooldownMs > 0 {
					lockModel(conn, model, domain.CooldownUntil(fb.CooldownMs))
				}
				return
			}
			// Both are account-level: keep whichever runs longer.
			if existing, err := time.Parse(time.RFC3339, conn.RateLimitedUntil); err == nil &&
				existing.After(time.Now().Add(time.Duration(fb.CooldownMs)*time.Millisecond)) {
				if settings.ModelLockOn() && model != "" {
					lockModel(conn, model, conn.RateLimitedUntil)
				}
				return
			}
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
				lockModel(conn, model, until)
			}
		}
	})
}

// lockModel records a per-model lock, creating the map on first use.
func lockModel(conn *domain.ProviderConnection, model, until string) {
	if conn.ModelLocks == nil {
		conn.ModelLocks = make(map[string]string)
	}
	conn.ModelLocks[model] = until
}

// ClearError resets a connection's error state after a successful request.
//
// A success does not prove every previous failure was spurious: with concurrent
// requests, a slow success can land after another request recorded a hard quota
// exhaustion or a suspension. Clearing that account-level cooldown would put an
// exhausted account straight back into rotation, so sticky failures keep their
// cooldown and only the per-model lock of the model that just worked is cleared.
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

		stickyCooldown := domain.IsStickyAccountCooldown(conn.LastError) &&
			domain.IsAccountUnavailable(conn.RateLimitedUntil)

		if !stickyCooldown {
			conn.RateLimitedUntil = ""
			conn.BackoffLevel = 0
			conn.LastError = ""
			conn.LastErrorAt = ""
		}

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
			log.Printf("[AUTH] Token refresh failed for %s: %v", conn.Name, err)
			return nil, &oauthRefreshError{
				ConnectionID: conn.ID,
				Dead:         isDeadRefreshToken(err),
				Err:          err,
			}
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
