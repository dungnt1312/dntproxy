package http

import (
	"net/http"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/gin-gonic/gin"
)

// RegisterToolsRoutes adds /api/tools/* endpoints.
func RegisterToolsRoutes(api *gin.RouterGroup, store port.CredentialStore) {
	jsonDB, ok := store.(*storage.JsonDB)
	if !ok {
		return
	}

	svc := service.NewToolsService(jsonDB)

	api.GET("/tools", apiListTools(svc))
	api.GET("/tools/status", apiToolsStatus(svc))
	api.POST("/tools/:id/configure", apiConfigureTool(svc))
	api.POST("/tools/:id/reset", apiResetTool(svc))
	api.POST("/tools/configure-all", apiConfigureAllTools(svc))
	api.POST("/tools/reset-all", apiResetAllTools(svc))
}

func apiListTools(svc *service.ToolsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		statuses, err := svc.ListTools()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, statuses)
	}
}

func apiToolsStatus(svc *service.ToolsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		statuses, err := svc.ListTools()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Filter to only configured tools
		var configured []service.ToolStatus
		for _, s := range statuses {
			if s.Configured {
				configured = append(configured, s)
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"configured": configured,
			"total":      len(statuses),
			"count":      len(configured),
		})
	}
}

func apiConfigureTool(svc *service.ToolsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := domain.ToolID(c.Param("id"))
		def := domain.GetToolDefinition(id)
		if def == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown tool: " + string(id)})
			return
		}

		if err := svc.Configure(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		status, _ := svc.GetStatus(id)
		c.JSON(http.StatusOK, gin.H{
			"message": def.Name + " configured successfully",
			"status":  status,
		})
	}
}

func apiResetTool(svc *service.ToolsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := domain.ToolID(c.Param("id"))
		def := domain.GetToolDefinition(id)
		if def == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown tool: " + string(id)})
			return
		}

		if err := svc.Reset(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		status, _ := svc.GetStatus(id)
		c.JSON(http.StatusOK, gin.H{
			"message": def.Name + " reset to defaults",
			"status":  status,
		})
	}
}

func apiConfigureAllTools(svc *service.ToolsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		statuses, err := svc.ListTools()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var results []gin.H
		for _, s := range statuses {
			if !s.Installed {
				continue
			}
			if err := svc.Configure(s.ID); err != nil {
				results = append(results, gin.H{"id": s.ID, "name": s.Name, "success": false, "error": err.Error()})
			} else {
				updated, _ := svc.GetStatus(s.ID)
				results = append(results, gin.H{"id": s.ID, "name": s.Name, "success": true, "status": updated})
			}
		}

		c.JSON(http.StatusOK, gin.H{"results": results, "count": len(results)})
	}
}

func apiResetAllTools(svc *service.ToolsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		statuses, err := svc.ListTools()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var results []gin.H
		for _, s := range statuses {
			if !s.Configured {
				continue
			}
			if err := svc.Reset(s.ID); err != nil {
				results = append(results, gin.H{"id": s.ID, "name": s.Name, "success": false, "error": err.Error()})
			} else {
				results = append(results, gin.H{"id": s.ID, "name": s.Name, "success": true})
			}
		}

		c.JSON(http.StatusOK, gin.H{"results": results, "count": len(results)})
	}
}
