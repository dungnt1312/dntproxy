package http

import (
	"net/http"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterCLIToolRoutes(api *gin.RouterGroup, store port.CredentialStore) {
	svc := service.NewCLIToolsService(store)

	api.GET("/cli-tools/configs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"tools": svc.Statuses()})
	})

	api.POST("/cli-tools/configs/preview", func(c *gin.Context) {
		var req domain.CLIToolsConfigRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		resp, err := svc.Preview(req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	})

	api.POST("/cli-tools/configs/apply", func(c *gin.Context) {
		var req domain.CLIToolsConfigRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		resp, err := svc.Apply(req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	})

	api.POST("/cli-tools/configs/restore", func(c *gin.Context) {
		var req domain.CLIToolsRestoreRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"results": svc.Restore(req)})
	})
}
