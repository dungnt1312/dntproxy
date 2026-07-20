package http

import (
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requireAdmin is defined in auth-guard.go (shared across handlers).

// tenantView is the API response shape for a tenant with aggregated stats.
type tenantView struct {
	domain.Tenant
	Connections int `json:"connections"`
	Combos      int `json:"combos"`
	Keys        int `json:"keys"`
}

// === Tenants (admin-only) ===

func apiListTenants(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		cfg, err := store.Load()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		views := make([]tenantView, 0, len(cfg.Tenants))
		for _, t := range cfg.Tenants {
			views = append(views, buildTenantView(cfg, t))
		}
		c.JSON(200, views)
	}
}

func apiCreateTenant(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var req struct {
			Slug  string `json:"slug"`
			Name  string `json:"name"`
			Notes string `json:"notes"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}
		slug := domain.NormalizeTenantSlug(req.Slug)
		if slug == "" {
			c.JSON(400, gin.H{"error": "slug is required"})
			return
		}
		if err := domain.ValidateTenantSlug(slug); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = slug
		}

		var created *domain.Tenant
		var defaultKey *domain.APIKey
		var defaultKeyValue string

		if err := store.Update(func(cfg *domain.AppConfig) {
			if cfg.Tenants == nil {
				cfg.Tenants = []domain.Tenant{}
			}
			if domain.IsTenantSlugTaken(cfg.Tenants, slug, "") {
				return
			}
			now := time.Now().UTC().Format(time.RFC3339)
			t := domain.Tenant{
				ID:        uuid.New().String(),
				Slug:      slug,
				Name:      name,
				Status:    domain.TenantStatusActive,
				Notes:     strings.TrimSpace(req.Notes),
				CreatedAt: now,
				UpdatedAt: now,
			}
			cfg.Tenants = append(cfg.Tenants, t)
			created = &t

			// Auto-create a default dashboard key for the new tenant.
			defaultKeyValue = GenerateAPIKey(slug)
			k := domain.APIKey{
				ID:              uuid.New().String(),
				Name:            name + " (default)",
				Key:             defaultKeyValue,
				IsActive:        true,
				DashboardAccess: true,
				CreatedAt:       now,
				TenantID:        slug,
			}
			cfg.APIKeys = append(cfg.APIKeys, k)
			defaultKey = &k
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if created == nil {
			c.JSON(409, gin.H{"error": "Tenant slug already exists: " + slug})
			return
		}
		logAction("tenant created: %s (%s), default key: %s", created.Slug, created.ID, defaultKey.ID)
		c.JSON(200, gin.H{
			"tenant": created,
			"defaultKey": gin.H{
				"id":   defaultKey.ID,
				"name": defaultKey.Name,
				"key":  defaultKeyValue,
			},
		})
	}
}

func apiUpdateTenant(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		id := c.Param("id")
		var req struct {
			Name   *string `json:"name,omitempty"`
			Notes  *string `json:"notes,omitempty"`
			Status *string `json:"status,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}

		found := false
		var updated *domain.Tenant
		if err := store.Update(func(cfg *domain.AppConfig) {
			t := domain.FindTenantByID(cfg.Tenants, id)
			if t == nil {
				return
			}
			found = true
			if req.Name != nil {
				if name := strings.TrimSpace(*req.Name); name != "" {
					t.Name = name
				}
			}
			if req.Notes != nil {
				t.Notes = strings.TrimSpace(*req.Notes)
			}
			if req.Status != nil {
				switch strings.ToLower(strings.TrimSpace(*req.Status)) {
				case domain.TenantStatusActive:
					t.Status = domain.TenantStatusActive
				case domain.TenantStatusDisabled:
					t.Status = domain.TenantStatusDisabled
				default:
					return
				}
			}
			t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			updated = t
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found || updated == nil {
			c.JSON(404, gin.H{"error": "Tenant not found"})
			return
		}
		// Status change must take effect immediately — bypass the 5s cache.
		if req.Status != nil {
			invalidateTenantDisableCache(updated.Slug)
		}
		logAction("tenant updated: %s (status=%s)", updated.Slug, updated.Status)
		c.JSON(200, gin.H{"ok": true})
	}
}

func apiDeleteTenant(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		id := c.Param("id")
		// cascade=true also deletes the tenant's connections, combos, and keys.
		cascade := strings.EqualFold(c.Query("cascade"), "true")

		found := false
		deletedSlug := ""
		if err := store.Update(func(cfg *domain.AppConfig) {
			idx := -1
			for i := range cfg.Tenants {
				if cfg.Tenants[i].ID == id {
					idx = i
					deletedSlug = cfg.Tenants[i].Slug
					break
				}
			}
			if idx < 0 {
				return
			}
			found = true
			cfg.Tenants = append(cfg.Tenants[:idx], cfg.Tenants[idx+1:]...)
			if cascade && deletedSlug != "" {
				cfg.ProviderConnections = filterConnsNotTenant(cfg.ProviderConnections, deletedSlug)
				cfg.Combos = filterCombosNotTenant(cfg.Combos, deletedSlug)
				cfg.APIKeys = filterKeysNotTenant(cfg.APIKeys, deletedSlug)
			}
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(404, gin.H{"error": "Tenant not found"})
			return
		}
		logAction("tenant deleted: %s (cascade=%v)", deletedSlug, cascade)
		c.JSON(200, gin.H{"ok": true})
	}
}

// apiGenerateTenantKey creates an API key pinned to the tenant's slug.
// The key is returned once (full key string only visible in this response).
func apiGenerateTenantKey(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		id := c.Param("id")
		var req struct {
			Name                 string   `json:"name"`
			DashboardAccess      *bool    `json:"dashboardAccess"`
			AllowedConnectionIDs []string `json:"allowedConnectionIds"`
			AllowedModels        []string `json:"allowedModels"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
			c.JSON(400, gin.H{"error": "name is required"})
			return
		}
		req.AllowedConnectionIDs = uniqueNonEmpty(req.AllowedConnectionIDs)
		req.AllowedModels = uniqueNonEmpty(req.AllowedModels)

		// Resolve the tenant by ID to get its slug and verify status.
		tenants, err := store.GetTenants()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		t := domain.FindTenantByID(tenants, id)
		if t == nil {
			c.JSON(404, gin.H{"error": "Tenant not found"})
			return
		}
		if t.Status == domain.TenantStatusDisabled {
			c.JSON(400, gin.H{"error": "Cannot generate key for disabled tenant"})
			return
		}

		dashboardAccess := false
		if req.DashboardAccess != nil {
			dashboardAccess = *req.DashboardAccess
		}
		key := GenerateAPIKey(t.Slug)
		apiKey := domain.APIKey{
			ID:                   uuid.New().String(),
			Name:                 strings.TrimSpace(req.Name),
			Key:                  key,
			IsActive:             true,
			DashboardAccess:      dashboardAccess,
			CreatedAt:            time.Now().UTC().Format(time.RFC3339),
			AllowedConnectionIDs: req.AllowedConnectionIDs,
			AllowedModels:        req.AllowedModels,
			TenantID:             t.Slug,
		}
		if err := store.Update(func(cfg *domain.AppConfig) {
			cfg.APIKeys = append(cfg.APIKeys, apiKey)
		}); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		logAction("tenant key generated: %s for tenant %s", apiKey.ID, t.Slug)
		c.JSON(200, gin.H{
			"id":       apiKey.ID,
			"name":     apiKey.Name,
			"key":      key,
			"tenantId": apiKey.TenantID,
		})
	}
}

// buildTenantView aggregates per-tenant resource counts from the full config.
func buildTenantView(cfg *domain.AppConfig, t domain.Tenant) tenantView {
	view := tenantView{Tenant: t}
	for _, conn := range cfg.ProviderConnections {
		if conn.TenantID == t.Slug {
			view.Connections++
		}
	}
	for _, combo := range cfg.Combos {
		if combo.TenantID == t.Slug {
			view.Combos++
		}
	}
	for _, k := range cfg.APIKeys {
		if k.TenantID == t.Slug {
			view.Keys++
		}
	}
	return view
}

// filterConnsNotTenant returns connections NOT owned by the given tenant slug.
func filterConnsNotTenant(conns []domain.ProviderConnection, slug string) []domain.ProviderConnection {
	result := make([]domain.ProviderConnection, 0, len(conns))
	for _, c := range conns {
		if c.TenantID != slug {
			result = append(result, c)
		}
	}
	return result
}

func filterCombosNotTenant(combos []domain.Combo, slug string) []domain.Combo {
	result := make([]domain.Combo, 0, len(combos))
	for _, c := range combos {
		if c.TenantID != slug {
			result = append(result, c)
		}
	}
	return result
}

func filterKeysNotTenant(keys []domain.APIKey, slug string) []domain.APIKey {
	result := make([]domain.APIKey, 0, len(keys))
	for _, k := range keys {
		if k.TenantID != slug {
			result = append(result, k)
		}
	}
	return result
}
