package http

import (
	"fmt"
	"log"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service/backup"
	"github.com/gin-gonic/gin"
)

// === Export ===

func apiExportBackup(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		var req struct {
			Version             string                      `json:"version"`
			ExportedAt          string                      `json:"exportedAt"`
			ProviderConnections []domain.ProviderConnection `json:"providerConnections"`
			Combos              []domain.Combo              `json:"combos"`
			ModelAliases        domain.AliasMap             `json:"modelAliases"`
			APIKeys             []domain.APIKey             `json:"apiKeys"`
			Settings            domain.Settings             `json:"settings"`
			ModelRegistry       *domain.ModelRegistry       `json:"modelRegistry"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid backup data: " + err.Error()})
			return
		}

		backupData := backup.BackupData{
			Version:             req.Version,
			ExportedAt:          req.ExportedAt,
			ProviderConnections: req.ProviderConnections,
			Combos:              req.Combos,
			ModelAliases:        req.ModelAliases,
			APIKeys:             req.APIKeys,
			Settings:            req.Settings,
			ModelRegistry:       req.ModelRegistry,
		}

		result, err := backup.Import(store, &backupData)
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
