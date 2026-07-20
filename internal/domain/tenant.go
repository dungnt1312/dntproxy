package domain

// Tenant filtering helpers for multi-tenancy (SaaS) support.
//
// Backward compatibility:
//   - TenantID == "" or "global" means legacy single-tenant mode.
//   - In legacy mode, ALL resources are visible (no filtering).
//   - When a specific TenantID is requested, only resources owned by that
//     tenant are visible. Resources with empty TenantID are NOT visible
//     to non-legacy tenants (strict isolation).
//   - Resources with a specific TenantID are NOT visible to other tenants.

// DefaultTenantID is the empty/legacy tenant.
const DefaultTenantID = ""

// GlobalTenantID is a reserved alias treated the same as DefaultTenantID.
const GlobalTenantID = "global"

// IsLegacyTenant returns true when the tenant represents legacy
// single-tenant mode (no filtering should be applied).
func IsLegacyTenant(tenantID string) bool {
	return tenantID == "" || tenantID == GlobalTenantID
}

// SameTenant returns true if a resource with the given owner tenant
// should be visible to the requester tenant.
//
// Visibility rules:
//   - Legacy requester (tenantID="") sees everything.
//   - Specific tenant sees only resources with matching TenantID.
//   - Resources with a specific TenantID are only visible to that tenant.
func SameTenant(resourceTenantID, requesterTenantID string) bool {
	if IsLegacyTenant(requesterTenantID) {
		return true
	}
	return resourceTenantID == requesterTenantID
}

// FilterConnectionsByTenant returns connections visible to the tenant.
func FilterConnectionsByTenant(conns []ProviderConnection, tenantID string) []ProviderConnection {
	if IsLegacyTenant(tenantID) {
		return conns
	}
	result := make([]ProviderConnection, 0, len(conns))
	for _, c := range conns {
		if c.TenantID == tenantID {
			result = append(result, c)
		}
	}
	return result
}

// FilterCombosByTenant returns combos visible to the tenant.
func FilterCombosByTenant(combos []Combo, tenantID string) []Combo {
	if IsLegacyTenant(tenantID) {
		return combos
	}
	result := make([]Combo, 0, len(combos))
	for _, c := range combos {
		if c.TenantID == tenantID {
			result = append(result, c)
		}
	}
	return result
}

// FilterAPIKeysByTenant returns API keys visible to the tenant.
// API keys with empty TenantID are admin keys — NOT exposed to tenants.
func FilterAPIKeysByTenant(keys []APIKey, tenantID string) []APIKey {
	if IsLegacyTenant(tenantID) {
		return keys
	}
	result := make([]APIKey, 0, len(keys))
	for _, k := range keys {
		if k.TenantID == tenantID {
			result = append(result, k)
		}
	}
	return result
}

// FilterConfigByTenant returns a shallow-filtered copy of AppConfig
// containing only resources visible to the tenant. Settings and
// ModelRegistry remain global (not tenant-scoped in this version).
func FilterConfigByTenant(cfg *AppConfig, tenantID string) *AppConfig {
	if cfg == nil {
		return nil
	}
	if IsLegacyTenant(tenantID) {
		return cfg
	}
	clone := *cfg
	clone.ProviderConnections = FilterConnectionsByTenant(cfg.ProviderConnections, tenantID)
	clone.Combos = FilterCombosByTenant(cfg.Combos, tenantID)
	clone.APIKeys = FilterAPIKeysByTenant(cfg.APIKeys, tenantID)
	// Tenants (the registry) is admin-only — never exposed to tenant users.
	clone.Tenants = nil
	// Note: ModelAliases is a map; tenant-scoping aliases would require
	// a per-alias TenantID. For now aliases remain shared (global).
	return &clone
}

// IsTenantDisabled reports whether the tenant identified by tenantID (slug)
// exists in the registry and is currently disabled.
//
// Returns false for:
//   - Legacy/global tenantID (admin/legacy keys are never disabled).
//   - Unknown tenantID (no matching Tenant record — treated as active
//     to preserve backward compatibility with keys created before the
//     Tenant registry existed).
func IsTenantDisabled(tenants []Tenant, tenantID string) bool {
	if IsLegacyTenant(tenantID) {
		return false
	}
	t := FindTenantBySlug(tenants, tenantID)
	if t == nil {
		// Unknown tenant — fall back to active (do not lock out pre-registry keys).
		return false
	}
	return t.Status == TenantStatusDisabled
}
