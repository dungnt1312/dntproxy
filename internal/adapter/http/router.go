package http

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/dungnt/dntproxy/internal/version"
	"github.com/dungnt/dntproxy/ui"
	"github.com/gin-gonic/gin"
)

// serverPortKey is the context key for storing actual server port
const serverPortKey = "server_port"

// NewRouter creates and configures the Gin router.
func NewRouter(store port.CredentialStore, providers port.ProviderRegistry, tunnelMgr port.TunnelManager) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(requestLogger())

	chatService := service.NewChatService(store, providers)

	// OpenAI-compatible endpoints (with optional API key check)
	v1 := r.Group("/v1")
	v1.Use(apiKeyMiddleware(store))
	{
		v1.POST("/chat/completions", chatHandler(chatService, store))
		v1.POST("/messages", messagesHandler(chatService, store))
		v1.GET("/models", modelsHandler(store))
	}

	// Dashboard API endpoints
	RegisterAPIRoutes(r, store, providers, chatService.ClearComboRotation)

	// Auth flow endpoints (Builder ID, IDC, Social Login, Fetch Models)
	RegisterAuthRoutes(r, store)

	// Tunnel endpoints
	if tunnelMgr != nil {
		RegisterTunnelRoutes(r, tunnelMgr, store)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version.Version})
	})

	// Serve static UI files (production: ui/dist/)
	serveStaticUI(r)

	return r
}

// SetServerPort stores the actual server port in the router context.
// This should be called from main.go after determining the final port.
func SetServerPort(r *gin.Engine, port int) {
	// Store in a middleware that sets it in every request context
	r.Use(func(c *gin.Context) {
		c.Set(serverPortKey, port)
		c.Next()
	})
}

// GetServerPort retrieves the actual server port from context.
func GetServerPort(c *gin.Context) int {
	if port, exists := c.Get(serverPortKey); exists {
		if p, ok := port.(int); ok {
			return p
		}
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

		if path == "/" {
			c.JSON(http.StatusOK, gin.H{
				"name":      "dntproxy",
				"version":   version.Version,
				"status":    "running",
				"dashboard": "/dashboard",
			})
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
		// Only allow localhost origins (any port)
		if strings.HasPrefix(origin, "http://localhost") ||
			strings.HasPrefix(origin, "http://127.0.0.1") ||
			strings.HasPrefix(origin, "http://[::1]") ||
			origin == "" {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "*")

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

func optionsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	}
}

func apiKeyMiddleware(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := store.GetSettings()
		if err != nil || !settings.RequireAPIKey {
			c.Next()
			return
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

		key := extractAPIKey(c.Request)
		if key == "" {
			// Fallback: accept key from query param (for EventSource/SSE which can't set headers)
			key = c.Query("key")
		}
		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "Missing API key"},
			})
			return
		}

		if !store.ValidateAPIKey(key) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "Invalid API key"},
			})
			return
		}

		c.Next()
	}
}

func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
