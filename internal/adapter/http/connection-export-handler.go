package http

import (
	"fmt"
	"log"
	"time"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service/backup"
	"github.com/gin-gonic/gin"
)

// === Export Single Connection ===

func apiExportConnection(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if _, ok := requireTenantOwnsConnection(c, store, id); !ok {
			return
		}

		data, err := backup.ExportConnection(store, id)
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}

		filename := fmt.Sprintf("dntproxy-connection-%s-%s.json",
			data.Connection.Name,
			time.Now().Format("20060102-150405"))
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.JSON(200, data)
	}
}

// === Export Multiple Connections ===

func apiExportConnections(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ConnectionIDs []string `json:"connectionIds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		// Verify the caller owns every requested connection.
		for _, id := range req.ConnectionIDs {
			if _, ok := requireTenantOwnsConnection(c, store, id); !ok {
				return
			}
		}

		data, err := backup.ExportConnections(store, req.ConnectionIDs)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		filename := fmt.Sprintf("dntproxy-connections-%s.json", time.Now().Format("20060102-150405"))
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.JSON(200, data)
	}
}

// === Import Single Connection ===

func apiImportConnectionFromFile(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var req struct {
			backup.ConnectionExportData
			Mode string `json:"mode"` // "add", "replace", "merge"
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid connection data: " + err.Error()})
			return
		}

		// Default mode is "add"
		mode := backup.ImportModeAdd
		if req.Mode != "" {
			mode = backup.ImportConnectionMode(req.Mode)
		}

		result, err := backup.ImportConnection(store, &req.ConnectionExportData, mode)
		if err != nil {
			c.JSON(400, gin.H{"error": "Import failed: " + err.Error()})
			return
		}

		log.Printf("[CONNECTION] Imported connection: %s (mode: %s, imported: %d, updated: %d, skipped: %d)",
			req.Connection.ID, mode, result.Imported, result.Updated, result.Skipped)

		c.JSON(200, gin.H{
			"ok":       true,
			"imported": result.Imported,
			"updated":  result.Updated,
			"skipped":  result.Skipped,
			"errors":   result.Errors,
		})
	}
}

// === Import Multiple Connections ===

func apiImportConnectionsFromFile(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		var req struct {
			backup.BackupData
			Mode string `json:"mode"` // "add", "replace", "merge"
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid backup data: " + err.Error()})
			return
		}

		// Default mode is "add"
		mode := backup.ImportModeAdd
		if req.Mode != "" {
			mode = backup.ImportConnectionMode(req.Mode)
		}

		result, err := backup.ImportConnections(store, &req.BackupData, mode)
		if err != nil {
			c.JSON(400, gin.H{"error": "Import failed: " + err.Error()})
			return
		}

		log.Printf("[CONNECTION] Imported %d connections (mode: %s, updated: %d, skipped: %d)",
			result.Imported, mode, result.Updated, result.Skipped)

		c.JSON(200, gin.H{
			"ok":       true,
			"imported": result.Imported,
			"updated":  result.Updated,
			"skipped":  result.Skipped,
			"errors":   result.Errors,
		})
	}
}
