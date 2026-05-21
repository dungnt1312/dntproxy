package http

import (
	"net/http"

	"github.com/dungnt/dntproxy/internal/service"
	"github.com/gin-gonic/gin"
)

// OpenAI-compatible model list response.
type modelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func modelsHandler(modelAccess *service.ModelAccessService) gin.HandlerFunc {
	return func(c *gin.Context) {
		policy := extractAPIKeyPolicy(c)
		pool, err := modelAccess.BuildPool(policy)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build model pool"})
			return
		}

		var models []modelObject

		for _, m := range pool.Models {
			ownedBy := m.DisplayProvider
			if ownedBy == "" {
				ownedBy = m.Provider
			}
			models = append(models, newModelObject(m.QualifiedID, ownedBy))
		}
		for _, combo := range pool.Combos {
			models = append(models, newModelObject(combo.Name, "combo"))
		}
		for _, alias := range pool.Aliases {
			models = append(models, newModelObject(alias.Name, "alias"))
		}

		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   models,
		})
	}
}

func newModelObject(id string, ownedBy string) modelObject {
	return modelObject{
		ID:      id,
		Object:  "model",
		Created: 1700000000,
		OwnedBy: ownedBy,
	}
}
