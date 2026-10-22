package http

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// fetchKiroConnectionModels answers POST /connections/:id/fetch-models for a
// Kiro connection with the account's live catalog.
//
// The response keeps the shape the dashboard already reads — {models: [...]} —
// and adds optional fields it ignores. It deliberately never answers with the
// upstream's 401/403: the dashboard treats any 401 as "dashboard session
// expired" and logs the operator out, so an upstream rejection is reported as
// 502 with the reason in the body instead.
//
// When the live fetch fails the call degrades to the configured defaults with a
// note (exactly what the endpoint returned for Kiro before), so the "Detect
// models" button never dead-ends.
func fetchKiroConnectionModels(c *gin.Context, store port.CredentialStore, conn *domain.ProviderConnection) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	svc := auth.NewTokenRefreshService(store)
	work := *conn // FetchKiroModelsWithRefresh may swap in refreshed credentials
	cat, err := auth.FetchKiroModelsWithRefresh(ctx, svc, &work)
	if err != nil {
		respondKiroModelsFallback(c, store, conn, err)
		return
	}
	if len(cat.Models) == 0 {
		respondKiroModelsFallback(c, store, conn, errors.New("Kiro returned an empty model list"))
		return
	}

	ids := cat.IDs()
	sort.Strings(ids)
	details := make([]auth.KiroModelInfo, len(cat.Models))
	copy(details, cat.Models)
	sort.Slice(details, func(i, j int) bool { return details[i].ID < details[j].ID })

	resp := gin.H{
		"provider": "kiro",
		"name":     conn.Name,
		"models":   ids,
		"source":   "live",
		"surface":  cat.Surface,
		"details":  details,
	}
	if cat.DefaultModel != "" {
		resp["defaultModel"] = cat.DefaultModel
	}
	c.JSON(200, resp)
}

func respondKiroModelsFallback(c *gin.Context, store port.CredentialStore, conn *domain.ProviderConnection, cause error) {
	var settings *domain.Settings
	if cfg, err := store.Load(); err == nil && cfg != nil {
		settings = &cfg.Settings
	}
	cfg := domain.GetProviderConfig(conn.Provider)
	c.JSON(200, gin.H{
		"provider": conn.Provider,
		"name":     conn.Name,
		"models":   domain.GetDefaultConnectionModels(settings, conn.Provider),
		"source":   "provider-config",
		"note":     "Live fetch failed (" + auth.UserMessage(cause) + "); showing " + cfg.Name + " defaults",
	})
}
