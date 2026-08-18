package http

import (
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestTenantHasLeftoverResources(t *testing.T) {
	cfg := &domain.AppConfig{
		APIKeys: []domain.APIKey{{ID: "k", TenantID: "acme"}},
	}
	if !tenantHasLeftoverResources(cfg, "acme") {
		t.Fatal("expected leftover key")
	}
	if tenantHasLeftoverResources(cfg, "globex") {
		t.Fatal("other tenant should be clean")
	}
	if tenantHasLeftoverResources(&domain.AppConfig{}, "acme") {
		t.Fatal("empty config should be clean")
	}
}
