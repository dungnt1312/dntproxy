package http

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
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
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
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

// serveStaticUI serves the built frontend from ui/dist/ if it exists.
func serveStaticUI(r *gin.Engine) {
	// Try to find ui/dist relative to executable or cwd
	candidates := []string{"ui/dist", "../ui/dist"}

	// Also check relative to executable path
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "ui", "dist"))
	}

	var distDir string
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			distDir = dir
			break
		}
	}

	if distDir == "" {
		// No built UI — serve a simple JSON at root
		r.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"name":    "dntproxy",
				"version": "0.1.0",
				"status":  "running",
				"ui":      "not built — run 'cd ui && bun run build'",
			})
		})
		return
	}

	log.Printf("[dntproxy] Serving UI from %s", distDir)

	// Serve static assets under /dashboard
	r.Static("/dashboard/assets", filepath.Join(distDir, "assets"))

	// Serve index.html at /dashboard root
	r.GET("/dashboard", func(c *gin.Context) {
		c.File(filepath.Join(distDir, "index.html"))
	})
	r.GET("/dashboard/", func(c *gin.Context) {
		c.File(filepath.Join(distDir, "index.html"))
	})

	// SPA fallback: serve index.html for all /dashboard/* routes
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Don't serve UI for API/v1 routes
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") || path == "/health" {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}

		// Serve UI for /dashboard/* routes
		if strings.HasPrefix(path, "/dashboard/") {
			// Try to serve the exact file first (remove /dashboard prefix)
			relativePath := strings.TrimPrefix(path, "/dashboard")
			filePath := filepath.Join(distDir, relativePath)
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				c.File(filePath)
				return
			}

			// SPA fallback — serve index.html
			c.File(filepath.Join(distDir, "index.html"))
			return
		}

		// Root path - show API info
		if path == "/" {
			c.JSON(http.StatusOK, gin.H{
				"name":      "dntproxy",
				"version":   "0.1.0",
				"status":    "running",
				"dashboard": "/dashboard",
			})
			return
		}

		// Other routes - 404
		c.JSON(404, gin.H{"error": "Not found"})
	})
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
