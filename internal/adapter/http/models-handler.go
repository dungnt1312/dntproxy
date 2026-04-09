package http

import (
	"net/http"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// OpenAI-compatible model list response.
type modelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func modelsHandler(store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var models []modelObject

		// Add Kiro models
		kiroModels := []string{
			"claude-sonnet-4.5",
			"claude-haiku-4.5",
			"deepseek-3.2",
			"deepseek-3.1",
			"qwen3-coder-next",
		}
		for _, m := range kiroModels {
			models = append(models, modelObject{
				ID:      "kr/" + m,
				Object:  "model",
				Created: 1700000000,
				OwnedBy: "kiro",
			})
		}

		// Add OpenAI models
		openaiModels := []string{
			"gpt-4.1",
			"gpt-4.1-mini",
			"gpt-4.1-nano",
			"gpt-4o",
			"gpt-4o-mini",
			"o3",
			"o3-mini",
			"o4-mini",
		}
		for _, m := range openaiModels {
			models = append(models, modelObject{
				ID:      "oai/" + m,
				Object:  "model",
				Created: 1700000000,
				OwnedBy: "openai",
			})
		}

		// Add combos as models
		combos, err := store.GetCombos()
		if err == nil {
			for _, combo := range combos {
				models = append(models, modelObject{
					ID:      combo.Name,
					Object:  "model",
					Created: 1700000000,
					OwnedBy: "combo",
				})
			}
		}

		// Add aliases as models
		aliases, err := store.GetModelAliases()
		if err == nil {
			for alias := range aliases {
				models = append(models, modelObject{
					ID:      alias,
					Object:  "model",
					Created: 1700000000,
					OwnedBy: "alias",
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   models,
		})
	}
}
