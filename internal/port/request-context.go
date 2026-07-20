package port

import "github.com/dungnt/dntproxy/internal/domain"

// RequestContext carries tenant, authentication, and observability data
// through the request processing pipeline. This is the primary mechanism
// for multi-tenancy (SaaS) support.
//
// Legacy single-tenant deployments use TenantID = "".
type RequestContext struct {
	// RequestID is a unique identifier for this request (UUID).
	RequestID string

	// TenantID identifies the customer/tenant. Empty string means legacy
	// single-tenant mode (backward compatible).
	TenantID string

	// APIKey is the resolved APIKey record (contains TenantID, policies, etc).
	APIKey *domain.APIKey

	// Policy contains allowed connections/models derived from the API key.
	Policy *APIKeyPolicy

	// Metadata holds optional per-request data (compression stats, etc).
	Metadata RequestMetadata
}

// NewRequestContext creates a new context with defaults.
func NewRequestContext(requestID, tenantID string) *RequestContext {
	return &RequestContext{
		RequestID: requestID,
		TenantID:  tenantID,
	}
}

// IsMultiTenant returns true if this context represents a specific tenant.
func (c *RequestContext) IsMultiTenant() bool {
	return c != nil && domain.IsLegacyTenant(c.TenantID)
}

// WithAPIKey returns a copy of the context with the API key attached.
func (c *RequestContext) WithAPIKey(key *domain.APIKey) *RequestContext {
	if c == nil {
		return nil
	}
	clone := *c
	clone.APIKey = key
	if key != nil && key.TenantID != "" {
		clone.TenantID = key.TenantID
	}
	return &clone
}

// ToMetadata converts the context into RequestMetadata for variadic propagation.
func (c *RequestContext) ToMetadata() RequestMetadata {
	if c == nil {
		return RequestMetadata{}
	}
	return RequestMetadata{
		Compression: c.Metadata.Compression,
		TenantID:    c.TenantID,
	}
}
