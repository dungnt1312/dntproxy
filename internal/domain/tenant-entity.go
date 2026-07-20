package domain

import (
	"fmt"
	"strings"
	"unicode"
)

// TenantStatusActive / TenantStatusDisabled are the allowed values for Tenant.Status.
const (
	TenantStatusActive   = "active"
	TenantStatusDisabled = "disabled"
)

// Tenant is a customer/workspace that owns a set of connections, combos,
// aliases, API keys, and logs. The Slug is the public identifier embedded
// in API keys (e.g. "acme" → "sk-dnt-acme-...").
//
// A Tenant is the OWNER of resources, not a tenant-scoped resource itself,
// so it does not carry its own TenantID field.
type Tenant struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`               // unique, lowercase alphanumeric + hyphen, used as API key prefix
	Name      string `json:"name"`               // human-friendly display name
	Status    string `json:"status"`             // "active" | "disabled"
	Notes     string `json:"notes,omitempty"`    // free-form admin notes
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// NormalizeTenantSlug lowercases and strips characters that are invalid in a
// slug (or in an API key prefix). Returns the cleaned slug.
func NormalizeTenantSlug(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	var b strings.Builder
	lastHyphen := false
	for _, r := range slug {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if r == '-' && b.Len() > 0 && !lastHyphen {
			b.WriteRune('-')
			lastHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// ValidateTenantSlug returns an error if the slug is not a valid tenant slug:
// 2-32 chars, lowercase alphanumeric and hyphens only, must start and end
// with an alphanumeric character. Reserved words ("global", "admin", "all")
// are rejected to avoid collisions with the legacy/admin namespace.
func ValidateTenantSlug(slug string) error {
	n := len(slug)
	if n < 2 || n > 32 {
		return fmt.Errorf("tenant slug must be 2-32 characters long (got %d)", n)
	}
	for i, r := range slug {
		isAlnum := unicode.IsLetter(r) || unicode.IsDigit(r)
		isHyphen := r == '-'
		if !isAlnum && !isHyphen {
			return fmt.Errorf("tenant slug contains invalid character %q at position %d (only lowercase letters, digits, and hyphens allowed)", r, i)
		}
		if unicode.IsUpper(r) {
			return fmt.Errorf("tenant slug must be lowercase (got %q at position %d)", r, i)
		}
	}
	if slug[0] == '-' || slug[n-1] == '-' {
		return fmt.Errorf("tenant slug must not start or end with a hyphen")
	}
	switch slug {
	case "global", "admin", "all", "default":
		return fmt.Errorf("tenant slug %q is reserved", slug)
	}
	return nil
}

// FindTenantBySlug returns a pointer to the tenant with the given slug, or
// nil if none matches. The lookup is a linear scan (tenant counts are small).
func FindTenantBySlug(tenants []Tenant, slug string) *Tenant {
	if slug == "" {
		return nil
	}
	for i := range tenants {
		if tenants[i].Slug == slug {
			return &tenants[i]
		}
	}
	return nil
}

// FindTenantByID returns a pointer to the tenant with the given ID, or nil.
func FindTenantByID(tenants []Tenant, id string) *Tenant {
	if id == "" {
		return nil
	}
	for i := range tenants {
		if tenants[i].ID == id {
			return &tenants[i]
		}
	}
	return nil
}

// IsTenantSlugTaken returns true if any tenant already uses the slug.
func IsTenantSlugTaken(tenants []Tenant, slug string, excludeID string) bool {
	t := FindTenantBySlug(tenants, slug)
	return t != nil && t.ID != excludeID
}
