package http

import (
	"fmt"
	"net/http"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/version"
	"github.com/gin-gonic/gin"
)

// RegisterInfoRoute adds GET /api/info — a single endpoint returning
// server identity, resolved URLs, and tunnel state so the UI never
// has to compute these itself.
func RegisterInfoRoute(api *gin.RouterGroup, store port.CredentialStore, tunnelMgr port.TunnelManager) {
	api.GET("/info", func(c *gin.Context) {
		serverPort := GetServerPort(c)
		localURL := fmt.Sprintf("http://127.0.0.1:%d/v1", serverPort)

		tunnelURL := ""
		tunnelRunning := false
		if tunnelMgr != nil {
			st := tunnelMgr.Status()
			tunnelRunning = st.Running
			if st.PublicURL != "" {
				tunnelURL = st.PublicURL + "/v1"
			} else if st.TunnelURL != "" {
				tunnelURL = st.TunnelURL + "/v1"
			}
		}

		// Best URL for CLI tools: tunnel if available, else local
		baseURL := localURL
		if tunnelURL != "" {
			baseURL = tunnelURL
		}

		c.JSON(http.StatusOK, gin.H{
			"version":       version.Version,
			"port":          serverPort,
			"localUrl":      localURL,
			"tunnelUrl":     tunnelURL,
			"tunnelRunning": tunnelRunning,
			"baseUrl":       baseURL,
		})
	})
}
