package service

import (
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// NormalizeModelPolicyString normalizes a model string to "provider/model[@connectionId]".
// Resolves provider aliases and strips duplicate provider prefixes.
func NormalizeModelPolicyString(model string) string {
	parsed, err := ParseModelString(model)
	if err != nil {
		return model
	}
	// Strip duplicate prefix: provider="glm", model="glm/glm-5.1" → "glm-5.1"
	if strings.HasPrefix(parsed.Model, parsed.Provider+"/") {
		parsed.Model = strings.TrimPrefix(parsed.Model, parsed.Provider+"/")
	}
	normalized := parsed.Provider + "/" + parsed.Model
	if parsed.ConnectionID != "" {
		normalized += "@" + parsed.ConnectionID
	}
	return normalized
}

// StripConnectionPin removes the @connectionId suffix from a model string.
func StripConnectionPin(model string) string {
	if atIdx := strings.Index(model, "@"); atIdx >= 0 {
		return model[:atIdx]
	}
	return model
}

// ModelPolicyMatch checks if a model string matches an allowed-model policy entry.
// Both sides are normalized before comparison. If the allowed entry has no @pin,
// the model's @pin is ignored for matching purposes.
func ModelPolicyMatch(model string, allowed string) bool {
	if model == allowed {
		return true
	}
	normalizedModel := NormalizeModelPolicyString(model)
	normalizedAllowed := NormalizeModelPolicyString(allowed)
	if strings.Contains(normalizedAllowed, "@") {
		return normalizedModel == normalizedAllowed
	}
	return StripConnectionPin(normalizedModel) == normalizedAllowed
}

// ConnectionAllowed checks if a connection ID is permitted by the policy.
// Returns true if policy is nil, has no connection restrictions, or the ID is in the allowlist.
func ConnectionAllowed(connectionID string, policy *port.APIKeyPolicy) bool {
	if policy == nil || len(policy.AllowedConnectionIDs) == 0 || connectionID == "" {
		return true
	}
	for _, id := range policy.AllowedConnectionIDs {
		if connectionID == id {
			return true
		}
	}
	return false
}

// IntersectConnectionIDs returns the intersection of two connection ID slices.
// If either is nil/empty, the other is returned (no restriction from that side).
func IntersectConnectionIDs(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	set := make(map[string]bool, len(a))
	for _, id := range a {
		set[id] = true
	}
	var result []string
	for _, id := range b {
		if set[id] {
			result = append(result, id)
		}
	}
	return result
}

// ModelAllowedByPolicy checks if a model is permitted by the API key's model allowlist.
// Returns true if policy is nil or has no model restrictions.
func ModelAllowedByPolicy(qualifiedModel string, policy *port.APIKeyPolicy) bool {
	if policy == nil || len(policy.AllowedModels) == 0 {
		return true
	}
	for _, allowed := range policy.AllowedModels {
		if ModelPolicyMatch(qualifiedModel, allowed) {
			return true
		}
	}
	return false
}

// modelNameAllowedByPolicy checks if a plain name (alias, combo) is in the policy allowlist.
// This is a simple string match — no normalization.
func modelNameAllowedByPolicy(name string, policy *port.APIKeyPolicy) bool {
	if policy == nil || len(policy.AllowedModels) == 0 {
		return true
	}
	for _, allowed := range policy.AllowedModels {
		if allowed == name {
			return true
		}
	}
	return false
}

// resolveProviderAliasForPolicy resolves a provider alias to canonical ID.
func resolveProviderAliasForPolicy(aliasOrID string) string {
	if id, ok := domain.ProviderAliasToID[aliasOrID]; ok {
		return id
	}
	return aliasOrID
}
