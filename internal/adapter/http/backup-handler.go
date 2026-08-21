package http

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service/backup"
	"github.com/gin-gonic/gin"
)

// === Export ===

func apiExportBackup(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		data, err := backup.Export(store)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		filename := fmt.Sprintf("dntproxy-backup-%s.json", time.Now().Format("20060102-150405"))
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.JSON(200, data)
	}
}

// === Import ===

func apiImportBackup(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			return
		}
		body, err := c.GetRawData()
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid backup data: " + err.Error()})
			return
		}

		var probe struct {
			Version string `json:"version"`
			Mode    string `json:"mode"`
		}
		if err := json.Unmarshal(body, &probe); err != nil {
			c.JSON(400, gin.H{"error": "Invalid backup data: " + err.Error()})
			return
		}

		// Version-less backups come from other tools (e.g. 9router): import
		// connections only and never touch combos, aliases, keys or settings.
		if probe.Version == "" {
			converted, err := backup.Parse9RouterBackup(body)
			if err != nil {
				c.JSON(400, gin.H{"error": "9router import failed: " + err.Error()})
				return
			}
			mode := backup.ImportModeMerge
			if probe.Mode != "" {
				mode = backup.ImportConnectionMode(probe.Mode)
			}
			result, err := backup.ImportConnections(store, converted.Data, mode)
			if err != nil {
				c.JSON(400, gin.H{"error": "Import failed: " + err.Error()})
				return
			}
			result.Errors = append(converted.Skipped, result.Errors...)
			log.Printf("[CONNECTION] Imported %d connections from foreign backup (updated: %d, skipped: %d)",
				result.Imported, result.Updated, result.Skipped)
			c.JSON(200, gin.H{
				"ok":       true,
				"imported": result.Imported,
				"updated":  result.Updated,
				"skipped":  result.Skipped,
				"errors":   result.Errors,
			})
			return
		}

		backupData, err := backup.ParseBackup(body)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid backup data: " + err.Error()})
			return
		}

		result, err := backup.Import(store, backupData)
		if err != nil {
			c.JSON(400, gin.H{"error": "Import failed: " + err.Error()})
			return
		}

		log.Printf("[BACKUP] Imported %d items (full override)", result.Imported)

		c.JSON(200, gin.H{
			"ok":       true,
			"imported": result.Imported,
		})
	}
}
