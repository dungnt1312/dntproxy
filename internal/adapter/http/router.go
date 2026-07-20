package http

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/compressor"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/dungnt/dntproxy/internal/version"
	"github.com/dungnt/dntproxy/ui"
	"github.com/gin-gonic/gin"
)

// serverPortKey is the context key for storing actual server port
const serverPortKey = "server_port"

// telegramBotKey is the context key for the telegram bot instance
const telegramBotKey = "telegram_bot"

// tenantIDKey is the context key for the resolved tenant ID (multi-tenancy).
const tenantIDKey = "tenant_id"

// apiKeyObjKey is the context key for the resolved APIKey object (multi-tenancy).
const apiKeyObjKey = "api_key_obj"

// Package-level references for late-bound components
var (
	globalTelegramBot interface{}
	globalServerPort  int
)

// NewRouter creates and configures the Gin router.
func NewRouter(store port.CredentialStore, providers port.ProviderRegistry, imageProviders port.ImageProviderRegistry, tunnelMgr port.TunnelManager) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(requestLogger())

	// Initialize runtime log-bodies flag from persisted settings
	if s, err := store.GetSettings(); err == nil && s != nil {
		shared.SetLogBodiesEnabled(s.LogBodies)
	}

	chatService := service.NewChatService(store, providers)
	modelAccess := service.NewModelAccessService(store)

	// Build compressor — re-reads settings at most once per second
	comp := compressor.NewWithLoader(func() compressor.Options {
		s, err := store.GetSettings()
		if err != nil || s == nil {
			return compressor.Options{}
		}
		s.Compression.Normalize()
		return compressor.Options{
			Enabled:          s.Compression.Enabled,
			MinContentLength: s.Compression.MinContentLength,
			LogSavings:       s.Compression.LogSavings,
		}
	})

	// OpenAI-compatible endpoints (with optional API key check)
	v1 := r.Group("/v1")
	v1.Use(apiKeyMiddleware(store))
	{
		v1.POST("/chat/completions", chatHandler(chatService, store, comp))
		v1.POST("/messages", messagesHandler(chatService, store, comp))
		v1.POST("/images/generations", imageGenerationsHandler(store, imageProviders))
		v1.POST("/images/edits", imageEditsHandler(store, imageProviders))
		v1.GET("/models", modelsHandler(modelAccess, store, imageProviders))
	}

	// Dashboard API endpoints (tunnel, OAuth enrollment, fetch-models registered
	// inside on the /api group under dashboardKeyMiddleware)
	RegisterAPIRoutes(r, store, providers, imageProviders, chatService.ClearComboRotation, tunnelMgr)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version.Version})
	})

	// Serve static UI files (production: ui/dist/)
	serveStaticUI(r)

	return r
}

// SetServerPort stores the actual server port for late-bound access.
func SetServerPort(r *gin.Engine, port int) {
	globalServerPort = port
}

// SetTelegramBot stores the telegram bot reference for handler access.
func SetTelegramBot(r *gin.Engine, bot interface{}, alerter interface{}) {
	globalTelegramBot = bot
}

// GetServerPort retrieves the actual server port.
func GetServerPort(c *gin.Context) int {
	if globalServerPort > 0 {
		return globalServerPort
	}
	return 20199 // fallback
}

// serveStaticUI serves the built frontend. Priority: filesystem > embedded.
func serveStaticUI(r *gin.Engine) {
	var uiFS http.FileSystem

	// 1. Try filesystem (for development)
	candidates := []string{"ui/dist", "../ui/dist"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "ui", "dist"))
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			log.Printf("[dntproxy] Serving UI from filesystem: %s", dir)
			uiFS = http.Dir(dir)
			break
		}
	}

	// 2. Fallback to embedded FS
	if uiFS == nil {
		sub, err := fs.Sub(ui.DistFS, "dist")
		if err != nil {
			log.Printf("[dntproxy] No embedded UI available: %v", err)
			serveNoUI(r)
			return
		}
		// Check if embedded FS has content
		if entries, err := fs.ReadDir(sub.(fs.ReadDirFS), "."); err != nil || len(entries) == 0 {
			log.Printf("[dntproxy] Embedded UI is empty")
			serveNoUI(r)
			return
		}
		log.Printf("[dntproxy] Serving UI from embedded binary")
		uiFS = http.FS(sub)
	}

	// Serve static assets under /dashboard
	r.StaticFS("/dashboard/assets", newPrefixFS(uiFS, "assets"))

	// Serve index.html at /dashboard root
	serveIndex := func(c *gin.Context) {
		f, err := uiFS.Open("index.html")
		if err != nil {
			c.JSON(500, gin.H{"error": "UI index.html not found"})
			return
		}
		defer f.Close()
		stat, _ := f.Stat()
		http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), f.(readSeeker))
	}
	r.GET("/dashboard", serveIndex)
	r.GET("/dashboard/", serveIndex)

	// Redirect root to dashboard
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/dashboard/")
	})

	// SPA fallback
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") || path == "/health" {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}

		if strings.HasPrefix(path, "/dashboard/") {
			relativePath := strings.TrimPrefix(path, "/dashboard/")
			if f, err := uiFS.Open(relativePath); err == nil {
				stat, _ := f.Stat()
				if !stat.IsDir() {
					http.ServeContent(c.Writer, c.Request, relativePath, stat.ModTime(), f.(readSeeker))
					f.Close()
					return
				}
				f.Close()
			}
			serveIndex(c)
			return
		}

		c.JSON(404, gin.H{"error": "Not found"})
	})
}

func serveNoUI(r *gin.Engine) {
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "dntproxy",
			"version": version.Version,
			"status":  "running",
			"ui":      "not available",
		})
	})
}

// readSeeker combines io.ReadSeeker for http.ServeContent
type readSeeker interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}

// prefixFS wraps http.FileSystem to serve from a subdirectory
type prefixFS struct {
	fs     http.FileSystem
	prefix string
}

func newPrefixFS(fsys http.FileSystem, prefix string) http.FileSystem {
	return &prefixFS{fs: fsys, prefix: prefix}
}

func (p *prefixFS) Open(name string) (http.File, error) {
	return p.fs.Open(p.prefix + "/" + strings.TrimPrefix(name, "/"))
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		// Allow localhost origins (any port) and Cloudflare tunnel origins
		if origin != "" && (strings.HasPrefix(origin, "http://localhost") ||
			strings.HasPrefix(origin, "http://127.0.0.1") ||
			strings.HasPrefix(origin, "http://[::1]") ||
			strings.HasSuffix(origin, ".trycloudflare.com")) {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "*")
		c.Header("Access-Control-Expose-Headers", "X-DNTProxy-Auth-Error")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[%s] %s", c.Request.Method, c.Request.URL.Path)
		c.Next()
	}
}

func apiKeyMiddleware(store port.CredentialStore) gin.HandlerFunc {
	return apiKeyMiddlewareWithDashboard(store, false)
}

func dashboardKeyMiddleware(store port.CredentialStore) gin.HandlerFunc {
	return apiKeyMiddlewareWithDashboard(store, true)
}

func apiKeyMiddlewareWithDashboard(store port.CredentialStore, requireDashboard bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Dashboard routes (/api/*) always require auth.
		// API routes (/v1/*) respect the RequireAPIKey setting for backward compatibility.
		if !requireDashboard {
			cfg, err := store.Load()
			if err == nil && cfg != nil && !cfg.Settings.RequireAPIKey {
				c.Next()
				return
			}
		}

		// Exempt endpoints needed for UI auth bootstrap
		path := c.Request.URL.Path
		method := c.Request.Method
		if method == "GET" && path == "/api/settings" {
			c.Next()
			return
		}
		if method == "POST" && path == "/api/auth/validate-key" {
			c.Next()
			return
		}
		if method == "GET" && path == "/api/auth/session" {
			c.Next()
			return
		}

		key := extractAPIKey(c.Request)
		if key == "" {
			// Fallback: accept key from query param (for EventSource/SSE which can't set headers)
			key = c.Query("key")
		}
		if key == "" {
			c.Header("X-DNTProxy-Auth-Error", "true")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "Missing API key"},
			})
			return
		}

		apiKey, valid := store.GetAPIKeyByValue(key)
		if !valid || apiKey == nil {
			c.Header("X-DNTProxy-Auth-Error", "true")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "Invalid API key"},
			})
			return
		}

		// Dashboard routes require DashboardAccess flag
		if requireDashboard && !apiKey.DashboardAccess {
			c.Header("X-DNTProxy-Auth-Error", "true")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{"message": "This API key does not have dashboard access"},
			})
			return
		}

		// Tenant-disable check: if the key belongs to a tenant that an admin
		// has disabled, reject every request with that key. Admin/legacy keys
		// (empty tenantID) always pass. Uses a short cache to avoid hitting the
		// store on every request.
		if isTenantDisabledCached(store, apiKey.TenantID) {
			c.Header("X-DNTProxy-Auth-Error", "true")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{"message": "Tenant is disabled. Contact your administrator."},
			})
			return
		}

		// Inject API key identity, tenant, and policy into context for downstream handlers
		c.Set("apiKeyID", apiKey.ID)
		c.Set(tenantIDKey, apiKey.TenantID)
		c.Set(apiKeyObjKey, apiKey)
		if len(apiKey.AllowedConnectionIDs) > 0 {
			c.Set("apiKeyAllowedConnectionIDs", apiKey.AllowedConnectionIDs)
		}
		if len(apiKey.AllowedModels) > 0 {
			c.Set("apiKeyAllowedModels", apiKey.AllowedModels)
		}

		c.Next()
	}
}

// GetTenantID returns the tenant ID resolved for the current request.
// Returns "" in legacy single-tenant mode.
func GetTenantID(c *gin.Context) string {
	if v, ok := c.Get(tenantIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetAPIKey returns the APIKey object resolved for the current request.
// Returns nil if no API key was provided (e.g. RequireAPIKey=false).
func GetAPIKey(c *gin.Context) *domain.APIKey {
	if v, ok := c.Get(apiKeyObjKey); ok {
		if k, ok := v.(*domain.APIKey); ok {
			return k
		}
	}
	return nil
}

func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// extractAPIKeyPolicy reads API key policy from Gin context (set by apiKeyMiddleware).
// Returns nil if no restrictions apply.
func extractAPIKeyPolicy(c *gin.Context) *port.APIKeyPolicy {
	var connIDs []string
	var models []string

	if v, ok := c.Get("apiKeyAllowedConnectionIDs"); ok {
		connIDs, _ = v.([]string)
	}
	if v, ok := c.Get("apiKeyAllowedModels"); ok {
		models, _ = v.([]string)
	}

	if len(connIDs) == 0 && len(models) == 0 {
		return nil
	}
	return &port.APIKeyPolicy{
		AllowedConnectionIDs: connIDs,
		AllowedModels:        models,
	}
}

// tenantDisableCache is a short-TTL cache of tenant disable status, keyed by
// tenant slug. It avoids hitting the credential store on every authenticated
// request. Entries expire after 5 seconds; the cache is also safe to use
// concurrently.
var tenantDisableCache = struct {
	sync.RWMutex
	m map[string]tenantDisableEntry
}{
	m: make(map[string]tenantDisableEntry),
}

type tenantDisableEntry struct {
	disabled  bool
	expiresAt time.Time
}

// isTenantDisabledCached reports whether the tenant identified by tenantID is
// disabled, using a 5-second in-memory cache to amortize store lookups.
// Admin/legacy keys (empty tenantID) always return false.
func isTenantDisabledCached(store port.CredentialStore, tenantID string) bool {
	if domain.IsLegacyTenant(tenantID) {
		return false
	}

	// Fast path: read from cache.
	tenantDisableCache.RLock()
	if entry, ok := tenantDisableCache.m[tenantID]; ok && time.Now().Before(entry.expiresAt) {
		tenantDisableCache.RUnlock()
		return entry.disabled
	}
	tenantDisableCache.RUnlock()

	// Slow path: ask the store.
	tenants, err := store.GetTenants()
	disabled := err == nil && domain.IsTenantDisabled(tenants, tenantID)

	tenantDisableCache.Lock()
	tenantDisableCache.m[tenantID] = tenantDisableEntry{
		disabled:  disabled,
		expiresAt: time.Now().Add(5 * time.Second),
	}
	tenantDisableCache.Unlock()
	return disabled
}

// invalidateTenantDisableCache clears the cached disable status for a tenant.
// Called after admin updates a tenant's status so that the change takes effect
// immediately rather than after the 5-second TTL.
func invalidateTenantDisableCache(tenantID string) {
	tenantDisableCache.Lock()
	delete(tenantDisableCache.m, tenantID)
	tenantDisableCache.Unlock()
}
