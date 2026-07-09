package service

import (
	"sync"
	"time"
)

// SessionAffinityStore is an in-memory sticky connection map keyed by AffinityKey.
type SessionAffinityStore struct {
	mu      sync.Mutex
	entries map[string]affinityEntry
}

type affinityEntry struct {
	ConnectionID string
	ExpiresAt    time.Time
}

// NewSessionAffinityStore creates an empty affinity store.
func NewSessionAffinityStore() *SessionAffinityStore {
	return &SessionAffinityStore{
		entries: make(map[string]affinityEntry),
	}
}

// Get returns the sticky connection ID if present and not expired.
func (s *SessionAffinityStore) Get(key string) (connectionID string, ok bool) {
	if key == "" || s == nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found := s.entries[key]
	if !found {
		return "", false
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.entries, key)
		return "", false
	}
	return entry.ConnectionID, true
}

// Put stores a sticky connection with TTL. Empty key or connectionID is a no-op.
func (s *SessionAffinityStore) Put(key, connectionID string, ttl time.Duration) {
	if s == nil || key == "" || connectionID == "" || ttl <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = affinityEntry{
		ConnectionID: connectionID,
		ExpiresAt:    time.Now().Add(ttl),
	}
}

// Delete removes a sticky entry.
func (s *SessionAffinityStore) Delete(key string) {
	if s == nil || key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
}

// AffinityKey builds storage key; empty means affinity off for this request.
// headerSession non-empty wins; else apiKeyID+provider+model; else "".
// Locked formats:
//   - header → "h:" + headerSession
//   - else apiKeyID → "k:" + apiKeyID + "|" + provider + "|" + model
//   - else ""
func AffinityKey(apiKeyID, provider, model, headerSession string) string {
	if headerSession != "" {
		return "h:" + headerSession
	}
	if apiKeyID != "" {
		return "k:" + apiKeyID + "|" + provider + "|" + model
	}
	return ""
}
