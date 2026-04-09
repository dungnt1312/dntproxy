package http

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/gin-gonic/gin"
)

func chatHandler(chatService *service.ChatService, store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check API key if required
		settings, _ := store.GetSettings()
		if settings != nil && settings.RequireAPIKey {
			key := extractAPIKey(c.Request)
			if key == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"message": "Missing API key"}})
				return
			}
			if !store.ValidateAPIKey(key) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"message": "Invalid API key"}})
				return
			}
		}

		// Read body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Failed to read body"}})
			return
		}

		// Extract model from raw body
		var partial struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(body, &partial); err != nil || partial.Model == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Missing model"}})
			return
		}

		log.Printf("[CHAT] POST /v1/chat/completions | model=%s", partial.Model)

		result := chatService.HandleChat(body, partial.Model)

		if result.Stream != nil {
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")
			c.Status(http.StatusOK)
			c.Writer.Flush()

			buf := make([]byte, 4096)
			for {
				n, readErr := result.Stream.Read(buf)
				if n > 0 {
					c.Writer.Write(buf[:n])
					c.Writer.Flush()
				}
				if readErr != nil {
					if readErr != io.EOF {
						log.Printf("[CHAT] Stream error: %s", readErr)
					}
					break
				}
			}
			result.Stream.Close()
			return
		}

		c.JSON(result.StatusCode, gin.H{
			"error": gin.H{"message": result.Error},
		})
	}
}
