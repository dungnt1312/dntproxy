package port

import "github.com/dungnt/dntproxy/internal/domain"

// QuotaBucket represents a single quota bucket (e.g. daily messages, weekly tokens, credits).
type QuotaBucket struct {
	Name      string `json:"name"`      // e.g. "daily_messages", "weekly_tokens", "credits"
	Used      int    `json:"used"`      // amount used
	Limit     int    `json:"limit"`     // total limit (0 = unlimited/unknown)
	Remaining int    `json:"remaining"` // remaining amount (-1 = unknown)
	ResetsAt  string `json:"resetsAt"`  // ISO timestamp when quota resets (empty = unknown)
	PctUsed   int    `json:"pctUsed"`   // percentage used (0-100)
}

// QuotaResult holds the result of a quota/usage check.
// Each provider returns different structures, so we use a flexible approach.
type QuotaResult struct {
	Provider string         `json:"provider"`
	HasData  bool           `json:"hasData"` // true if any quota info available
	Buckets  []QuotaBucket  `json:"buckets"` // named quota buckets (daily, weekly, credits, etc.)
	Extras   map[string]any `json:"extras"`  // provider-specific extras (planType, token expiry, etc.)
}

// QuotaChecker checks remaining quota for a provider connection.
type QuotaChecker interface {
	// CheckQuota returns quota info for the given connection.
	// Returns nil if the provider doesn't support quota checks.
	CheckQuota(conn *domain.ProviderConnection) (*QuotaResult, error)
}
