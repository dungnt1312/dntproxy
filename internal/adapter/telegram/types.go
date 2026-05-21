package telegram

import (
	"strings"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/port"
)

// AlertState tracks deduplication state for a single alert key.
type AlertState struct {
	LastSent time.Time
	Count    int
}

// DedupStore manages alert deduplication with 30-minute suppression.
type DedupStore struct {
	mu     sync.Mutex
	states map[string]*AlertState
}

const alertDedupWindow = 30 * time.Minute

// NewDedupStore creates a new deduplication store.
func NewDedupStore() *DedupStore {
	return &DedupStore{states: make(map[string]*AlertState)}
}

// ShouldSend checks if an alert should be sent (not suppressed).
// Returns true if the alert has not been sent in the last 30 minutes.
func (d *DedupStore) ShouldSend(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, exists := d.states[key]
	if !exists {
		d.states[key] = &AlertState{LastSent: time.Now(), Count: 1}
		return true
	}

	if time.Since(state.LastSent) >= alertDedupWindow {
		state.LastSent = time.Now()
		state.Count++
		return true
	}
	return false
}

// SuppressionWindow returns the deduplication window used for repeated alerts.
func SuppressionWindow() time.Duration {
	return alertDedupWindow
}

// Clear removes dedup state for a connection (used on recovery).
func (d *DedupStore) Clear(connectionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	prefix := connectionID + ":"
	for key := range d.states {
		if strings.HasPrefix(key, prefix) {
			delete(d.states, key)
		}
	}
}

// HasEntries returns true if there are any dedup entries for a connection.
func (d *DedupStore) HasEntries(connectionID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	prefix := connectionID + ":"
	for key := range d.states {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// DedupKey builds the deduplication key for an alert.
func DedupKey(alert port.Alert) string {
	switch alert.Type {
	case port.AlertAllDown:
		return alert.Provider + ":all_down"
	case port.AlertComboExhausted:
		return alert.Combo + ":exhausted"
	default:
		return alert.ConnectionID + ":" + string(alert.Type)
	}
}
