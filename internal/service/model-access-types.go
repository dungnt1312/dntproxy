package service

// ModelRef represents a model available through the effective pool.
type ModelRef struct {
	// QualifiedID is "provider/model" (normalized).
	QualifiedID string
	// Provider is the canonical provider ID.
	Provider string
	// Model is the model name without provider prefix.
	Model string
	// DisplayProvider is the prefix shown to users (routePrefix for custom connections).
	DisplayProvider string
	// ConnectionIDs lists connections that can serve this model.
	ConnectionIDs []string
}

// AliasRef represents an alias whose target is available in the pool.
type AliasRef struct {
	// Name is the alias key (e.g. "sonnet").
	Name string
	// Target is the resolved "provider/model" string.
	Target string
}

// ComboRef represents a combo with only its effective (allowed) members.
type ComboRef struct {
	// Name is the combo name.
	Name string
	// EffectiveModels are the qualified model strings that passed policy.
	EffectiveModels []string
	// ConnectionIDs are the combo-level connection restrictions (already intersected with policy).
	ConnectionIDs []string
}

// RouteAttempt describes a single model+connection attempt for chat execution.
type RouteAttempt struct {
	// QualifiedModel is "provider/model" or "provider/model@connectionId".
	QualifiedModel string
	// Provider is the canonical provider ID.
	Provider string
	// Model is the model name.
	Model string
	// PinnedConnectionID is set when model string includes @connectionId.
	PinnedConnectionID string
	// AllowedConnectionIDs restricts which connections can serve this attempt.
	// Empty means unrestricted (within the effective pool).
	AllowedConnectionIDs []string
}

// RoutePlan describes the full execution plan for a chat request.
type RoutePlan struct {
	// RequestedModel is the original model string from the request.
	RequestedModel string
	// IsCombo is true if the request resolved to a combo.
	IsCombo bool
	// ComboName is the combo name (empty if not a combo).
	ComboName string
	// Attempts is the ordered list of route attempts to try.
	Attempts []RouteAttempt
	// DeniedByPolicy is true when models exist but are blocked by API key restrictions.
	DeniedByPolicy bool
}

// EffectiveModelPool is the computed pool for a given API key policy.
type EffectiveModelPool struct {
	// Models are the direct models available.
	Models []ModelRef
	// Aliases are available aliases whose targets are routable.
	Aliases []AliasRef
	// Combos are combos with at least one effective member.
	Combos []ComboRef
}
