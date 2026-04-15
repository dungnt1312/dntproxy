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
	RegisterAPIRoutes(r, store, providers)

	// Auth flow endpoints (Builder ID, IDC, Social Login, Fetch Models)
	RegisterAuthRoutes(r, store)

	// Tunnel endpoints
	if tunnelMgr != nil {
		settings, _ := store.GetSettings()
		listenPort := settings.Port
		if listenPort == 0 {
			listenPort = 20199
		}
		RegisterTunnelRoutes(r, tunnelMgr, listenPort)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Serve static UI files (production: ui/dist/)
	serveStaticUI(r)

	return r
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

	// Serve static assets
	r.Static("/assets", filepath.Join(distDir, "assets"))

	// SPA fallback: serve index.html for all non-API, non-asset routes
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Don't serve UI for API/v1 routes
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") || path == "/health" {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}

		// Try to serve the exact file first
		filePath := filepath.Join(distDir, path)
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			c.File(filePath)
			return
		}

		// SPA fallback — serve index.html
		c.File(filepath.Join(distDir, "index.html"))
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

		key := extractAPIKey(c.Request)
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
