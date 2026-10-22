package auth

import (
	"context"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// Refresh policy mirrors kiro-shared's AuthProvider.
const (
	// kiroRefreshBeforeExpiry: REFRESH_BEFORE_EXPIRY_SECONDS (10 min).
	kiroRefreshBeforeExpiry = 10 * time.Minute
	// kiroInvalidationOffset: AUTH_TOKEN_INVALIDATION_OFFSET_SECONDS (3 min) —
	// a token this close to expiry no longer counts as usable.
	kiroInvalidationOffset = 3 * time.Minute
	// REFRESH_BACKOFF_BASE_SECONDS / REFRESH_BACKOFF_MAX_SECONDS.
	kiroBackoffBase = 5 * time.Second
	kiroBackoffMax  = 60 * time.Second

	kiroRefreshTimeout = 60 * time.Second
)

func isKiroConn(conn *domain.ProviderConnection) bool {
	return conn != nil && conn.Provider == "kiro"
}

// refreshBackoff is the per-connection state behind consecutiveRefreshFailures /
// nextRefreshAttemptAt / lastRefreshError.
type refreshBackoff struct {
	failures     int
	nextAt       time.Time
	err          error
	refreshToken string // token the failed attempt used
}

type backoffTracker struct {
	mu sync.Mutex
	m  map[string]*refreshBackoff
}

func newBackoffTracker() *backoffTracker {
	return &backoffTracker{m: make(map[string]*refreshBackoff)}
}

func backoffDelay(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	d := kiroBackoffBase
	for i := 1; i < failures && d < kiroBackoffMax; i++ {
		d *= 2
	}
	if d > kiroBackoffMax {
		d = kiroBackoffMax
	}
	return d
}

// cooldownErr returns the last refresh error while the connection is cooling
// down, so callers are rejected from cache instead of hitting the network.
// A changed refresh token (re-auth, import, another process refreshed) resets
// the state, like handleTokenChanges → resetRefreshBackoff.
func (b *backoffTracker) cooldownErr(id, currentRefreshToken string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.m[id]
	if st == nil {
		return nil
	}
	if st.refreshToken != currentRefreshToken {
		delete(b.m, id)
		return nil
	}
	if st.err != nil && time.Now().Before(st.nextAt) {
		return st.err
	}
	return nil
}

func (b *backoffTracker) fail(id, refreshToken string, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.m[id]
	if st == nil || st.refreshToken != refreshToken {
		st = &refreshBackoff{refreshToken: refreshToken}
		b.m[id] = st
	}
	st.failures++
	st.nextAt = time.Now().Add(backoffDelay(st.failures))
	st.err = err
}

func (b *backoffTracker) reset(id string) {
	b.mu.Lock()
	delete(b.m, id)
	b.mu.Unlock()
}

// kiroBackoff is shared by every TokenRefreshService: handlers construct their
// own service per request, but the cooldown must outlive them.
var kiroBackoff = newBackoffTracker()

// clientRegistrationExpired mirrors isClientRegistrationExpired. A connection
// created before client expiry was tracked has no clientSecretExpiresAt and is
// given the benefit of the doubt (AWS answers with invalid_client if it is gone).
func clientRegistrationExpired(pd map[string]interface{}) bool {
	raw := getStringFromMap(pd, "clientSecretExpiresAt")
	if raw == "" {
		return false
	}
	ts, ok := ParseExpiresAt(raw)
	if !ok {
		return false
	}
	return ts.Before(time.Now().Add(clientRegInvalidationOffset))
}

// refreshKiroConnection dispatches on the credential type exactly like
// AuthProvider.doRefreshToken: IdC → SSO OIDC, social → auth service.
func refreshKiroConnection(conn *domain.ProviderConnection) (*TokenResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), kiroRefreshTimeout)
	defer cancel()

	pd := conn.ProviderSpecificData
	clientID := getStringFromMap(pd, "clientId")
	clientSecret := getStringFromMap(pd, "clientSecret")

	if clientID != "" && clientSecret != "" {
		if clientRegistrationExpired(pd) {
			return nil, newKiroErrUser(KindInvalidIdCAuth,
				"IdC auth: No valid client registration found",
				"Your IAM Identity Center client registration expired. Please sign in again.")
		}
		return refreshTokenSSO(ctx, conn.RefreshToken, clientID, clientSecret, getStringFromMap(pd, "region"))
	}
	return authServiceClient{}.refreshToken(ctx, conn.RefreshToken)
}

// applyKiroRefresh copies a refresh result onto the connection. The stored
// profileArn wins, as SocialAuthProvider.refreshToken carries it over.
func applyKiroRefresh(conn *domain.ProviderConnection, res *TokenResult) {
	conn.AccessToken = res.AccessToken
	if res.RefreshToken != "" {
		conn.RefreshToken = res.RefreshToken
	}
	expiresIn := res.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}
	conn.ExpiresIn = expiresIn
	conn.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339)
	if res.ProfileArn != "" && getStringFromMap(conn.ProviderSpecificData, "profileArn") == "" {
		if conn.ProviderSpecificData == nil {
			conn.ProviderSpecificData = make(map[string]interface{})
		}
		conn.ProviderSpecificData["profileArn"] = res.ProfileArn
	}
	conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// supersededBy reports a connection whose refresh token was replaced while our
// refresh was in flight (another dntproxy worker, a re-import) and whose access
// token is still usable. Mirrors "Token refresh superseded by a concurrent
// refresh in another window; using existing token".
func supersededBy(store port.CredentialStore, id, usedRefreshToken string) *domain.ProviderConnection {
	cur, err := store.GetConnectionByID(id)
	if err != nil || cur == nil || cur.RefreshToken == "" || cur.RefreshToken == usedRefreshToken {
		return nil
	}
	if ts, ok := ParseExpiresAt(cur.ExpiresAt); !ok || ts.Before(time.Now().Add(kiroInvalidationOffset)) {
		return nil
	}
	return cur
}

// persistKiroRefresh writes refreshed tokens only if the stored refresh token
// is still the one this refresh used (writeToken is guarded the same way).
func persistKiroRefresh(store port.CredentialStore, updated *domain.ProviderConnection, usedRefreshToken string) error {
	return store.Update(func(cfg *domain.AppConfig) {
		for i := range cfg.ProviderConnections {
			c := &cfg.ProviderConnections[i]
			if c.ID != updated.ID {
				continue
			}
			if c.RefreshToken != usedRefreshToken {
				return
			}
			c.AccessToken = updated.AccessToken
			c.RefreshToken = updated.RefreshToken
			c.ExpiresAt = updated.ExpiresAt
			c.ExpiresIn = updated.ExpiresIn
			c.UpdatedAt = updated.UpdatedAt
			if arn := getStringFromMap(updated.ProviderSpecificData, "profileArn"); arn != "" {
				if c.ProviderSpecificData == nil {
					c.ProviderSpecificData = make(map[string]interface{})
				}
				c.ProviderSpecificData["profileArn"] = arn
			}
			return
		}
	})
}
