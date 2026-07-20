package domain

import "testing"

func TestIsLegacyTenant(t *testing.T) {
	cases := []struct {
		tenant string
		want   bool
	}{
		{"", true},
		{"global", true},
		{"tenant-1", false},
		{"acme-corp", false},
	}
	for _, tc := range cases {
		if got := IsLegacyTenant(tc.tenant); got != tc.want {
			t.Errorf("IsLegacyTenant(%q) = %v, want %v", tc.tenant, got, tc.want)
		}
	}
}

func TestSameTenant(t *testing.T) {
	cases := []struct {
		name           string
		resource, req  string
		want           bool
	}{
		{"legacy requester sees everything", "tenant-1", "", true},
		{"legacy requester sees legacy resource", "", "", true},
		{"tenant sees own resource", "tenant-1", "tenant-1", true},
		{"tenant blocked from other tenant", "tenant-2", "tenant-1", false},
		{"tenant blocked from legacy resource", "", "tenant-1", false},
		{"global requester sees everything", "tenant-1", "global", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SameTenant(tc.resource, tc.req); got != tc.want {
				t.Errorf("SameTenant(resource=%q, req=%q) = %v, want %v",
					tc.resource, tc.req, got, tc.want)
			}
		})
	}
}

func TestFilterConnectionsByTenant(t *testing.T) {
	conns := []ProviderConnection{
		{ID: "c1", TenantID: ""},
		{ID: "c2", TenantID: "acme"},
		{ID: "c3", TenantID: "globex"},
	}

	// Legacy: sees all
	if got := FilterConnectionsByTenant(conns, ""); len(got) != 3 {
		t.Errorf("legacy tenant: got %d, want 3", len(got))
	}

	// acme: sees only c2
	got := FilterConnectionsByTenant(conns, "acme")
	if len(got) != 1 || got[0].ID != "c2" {
		t.Errorf("acme tenant: got %+v, want [c2]", got)
	}
}

func TestFilterConfigByTenant(t *testing.T) {
	cfg := &AppConfig{
		ProviderConnections: []ProviderConnection{
			{ID: "c1", TenantID: ""},
			{ID: "c2", TenantID: "acme"},
		},
		Combos: []Combo{
			{Name: "shared", TenantID: ""},
			{Name: "acme-combo", TenantID: "acme"},
		},
		APIKeys: []APIKey{
			{ID: "k1", TenantID: "acme"},
		},
	}

	// Legacy: no filtering
	filtered := FilterConfigByTenant(cfg, "")
	if len(filtered.ProviderConnections) != 2 || len(filtered.Combos) != 2 {
		t.Errorf("legacy: expected unfiltered, got conns=%d combos=%d",
			len(filtered.ProviderConnections), len(filtered.Combos))
	}

	// acme tenant
	filtered = FilterConfigByTenant(cfg, "acme")
	if len(filtered.ProviderConnections) != 1 || filtered.ProviderConnections[0].ID != "c2" {
		t.Errorf("acme: expected [c2], got %+v", filtered.ProviderConnections)
	}
	if len(filtered.Combos) != 1 || filtered.Combos[0].Name != "acme-combo" {
		t.Errorf("acme: expected [acme-combo], got %+v", filtered.Combos)
	}
	if len(filtered.APIKeys) != 1 {
		t.Errorf("acme: expected 1 key, got %d", len(filtered.APIKeys))
	}
}

func TestFilterConfigByTenantNilSafe(t *testing.T) {
	if got := FilterConfigByTenant(nil, "acme"); got != nil {
		t.Errorf("nil cfg: expected nil, got %+v", got)
	}
}

func TestValidateTenantSlug(t *testing.T) {
	cases := []struct {
		name    string
		slug    string
		wantErr bool
	}{
		{"valid simple", "acme", false},
		{"valid with hyphen", "acme-corp", false},
		{"valid alphanumeric mix", "team42", false},
		{"valid max length 32", "a123456789012345678901234567890a", false},
		{"too short", "a", true},
		{"too long", "a123456789012345678901234567890abc", true},
		{"empty", "", true},
		{"uppercase rejected", "ACME", true},
		{"leading hyphen", "-acme", true},
		{"trailing hyphen", "acme-", true},
		{"special chars", "acme_corp", true},
		{"space", "acme corp", true},
		{"reserved global", "global", true},
		{"reserved admin", "admin", true},
		{"reserved all", "all", true},
		{"reserved default", "default", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTenantSlug(tc.slug)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateTenantSlug(%q) err = %v, wantErr = %v", tc.slug, err, tc.wantErr)
			}
		})
	}
}

func TestNormalizeTenantSlug(t *testing.T) {
	// NormalizeTenantSlug lowercases, trims, keeps existing hyphens, and drops
	// every other non-alphanumeric character (no separator insertion).
	cases := []struct{ in, want string }{
		{"  ACME  ", "acme"},
		{"ACME-Corp", "acme-corp"},      // existing hyphen preserved
		{"ACME_Corp", "acmecorp"},       // underscore dropped
		{"ACME.Corp", "acmecorp"},       // dot dropped
		{"ACME Corp!", "acmecorp"},      // space + ! dropped
		{"team 42", "team42"},           // space dropped
		{"---weird---", "weird"},        // leading/trailing hyphens trimmed
		{"UPPER-CASE", "upper-case"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeTenantSlug(tc.in); got != tc.want {
			t.Errorf("NormalizeTenantSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFindTenantBySlug(t *testing.T) {
	tenants := []Tenant{
		{ID: "1", Slug: "acme"},
		{ID: "2", Slug: "globex"},
	}
	if got := FindTenantBySlug(tenants, "acme"); got == nil || got.ID != "1" {
		t.Errorf("FindTenantBySlug(acme) = %+v, want id=1", got)
	}
	if got := FindTenantBySlug(tenants, "missing"); got != nil {
		t.Errorf("FindTenantBySlug(missing) = %+v, want nil", got)
	}
	if got := FindTenantBySlug(tenants, ""); got != nil {
		t.Errorf("FindTenantBySlug('') = %+v, want nil", got)
	}
}

func TestIsTenantSlugTaken(t *testing.T) {
	tenants := []Tenant{
		{ID: "1", Slug: "acme"},
	}
	if !IsTenantSlugTaken(tenants, "acme", "") {
		t.Error("IsTenantSlugTaken(acme, exclude='') should be true")
	}
	if IsTenantSlugTaken(tenants, "acme", "1") {
		t.Error("IsTenantSlugTaken(acme, exclude='1') should be false (same tenant)")
	}
	if IsTenantSlugTaken(tenants, "globex", "") {
		t.Error("IsTenantSlugTaken(globex) should be false")
	}
}

func TestIsTenantDisabled(t *testing.T) {
	tenants := []Tenant{
		{Slug: "acme", Status: TenantStatusActive},
		{Slug: "globex", Status: TenantStatusDisabled},
	}
	cases := []struct {
		name     string
		tenantID string
		want     bool
	}{
		{"active tenant", "acme", false},
		{"disabled tenant", "globex", true},
		{"unknown tenant (legacy fallback)", "unknown", false},
		{"legacy admin key", "", false},
		{"global admin key", "global", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTenantDisabled(tenants, tc.tenantID); got != tc.want {
				t.Errorf("IsTenantDisabled(%q) = %v, want %v", tc.tenantID, got, tc.want)
			}
		})
	}
}

func TestFilterConfigByTenantHidesTenantsRegistry(t *testing.T) {
	cfg := &AppConfig{
		Tenants: []Tenant{{Slug: "acme"}, {Slug: "globex"}},
	}
	filtered := FilterConfigByTenant(cfg, "acme")
	if filtered.Tenants != nil {
		t.Errorf("tenant-filtered config should hide the Tenants registry, got %+v", filtered.Tenants)
	}
	// Legacy mode keeps it.
	filtered = FilterConfigByTenant(cfg, "")
	if filtered.Tenants == nil || len(filtered.Tenants) != 2 {
		t.Errorf("legacy mode should keep Tenants registry, got %+v", filtered.Tenants)
	}
}
