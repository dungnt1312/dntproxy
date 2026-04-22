package http

import (
	"fmt"
	"net/http"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// RegisterTunnelRoutes registers tunnel API endpoints.
func RegisterTunnelRoutes(r *gin.Engine, tunnelMgr port.TunnelManager, store port.CredentialStore) {
	r.POST("/api/tunnel/enable", func(c *gin.Context) {
		enableTunnel(c, tunnelMgr, store)
	})
	r.POST("/api/tunnel/disable", func(c *gin.Context) {
		disableTunnel(c, tunnelMgr)
	})
	r.GET("/api/tunnel/status", func(c *gin.Context) {
		getTunnelStatus(c, tunnelMgr)
	})
}

func enableTunnel(c *gin.Context, tunnelMgr port.TunnelManager, store port.CredentialStore) {
	// Check if already running
	status := tunnelMgr.Status()
	if status.Running {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Tunnel already running",
			"url":     status.PublicURL,
		})
		return
	}

	// Get actual server port from context (set by main.go)
	localPort := GetServerPort(c)

	// Start tunnel in background
	go func() {
		if err := tunnelMgr.Enable(localPort); err != nil {
			// Error is stored in TunnelStatus.LastError via service
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "Starting tunnel...",
	})
}

func disableTunnel(c *gin.Context, tunnelMgr port.TunnelManager) {
	if err := tunnelMgr.Disable(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to stop tunnel: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Tunnel stopped",
	})
}

func getTunnelStatus(c *gin.Context, tunnelMgr port.TunnelManager) {
	status := tunnelMgr.Status()
	c.JSON(http.StatusOK, gin.H{
		"enabled":   status.Enabled,
		"running":   status.Running,
		"provider":  status.Provider,
		"tunnelUrl": status.TunnelURL,
		"shortId":   status.ShortID,
		"publicUrl": status.PublicURL,
		"lastError": status.LastError,
		"starting":  status.Starting,
	})
}
