package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/logger"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxChatBodySize = 10 * 1024 * 1024

// streamReadTimeout is the maximum time to wait between stream reads.
// Configurable via DNTPROXY_STREAM_TIMEOUT_MS env var (default: 5 minutes).
var streamReadTimeout = func() time.Duration {
	if ms, err := strconv.Atoi(os.Getenv("DNTPROXY_STREAM_TIMEOUT_MS")); err == nil && ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return 5 * time.Minute
}()

func chatHandler(chatService port.ChatService, store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.New().String()

		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxChatBodySize))
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

		result := chatService.HandleChat(body, partial.Model, requestID)
		logger.Get().AddEntry(domain.LogEntry{
			Level:      statusLevel(result.StatusCode),
			Provider:   "CLIENT",
			Direction:  "inbound",
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			StatusCode: result.StatusCode,
			DurationMs: time.Since(start).Milliseconds(),
			Model:      partial.Model,
			RequestID:  requestID,
			Message:    "Client chat completion request",
			Error:      result.Error,
			BodySize:   len(body),
		})

		if result.Stream != nil {
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")
			c.Status(http.StatusOK)
			c.Writer.Flush()

			buf := make([]byte, 4096)
			for {
				n, readErr := readWithTimeout(result.Stream, buf, streamReadTimeout)
				if n > 0 {
					if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
						break
					}
					c.Writer.Flush()
				}
				if readErr != nil {
					if readErr != io.EOF {
						log.Printf("[CHAT] Stream error: %s", readErr)
					}
					break
				}
				select {
				case <-c.Request.Context().Done():
					result.Stream.Close()
					return
				default:
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

func statusLevel(status int) string {
	if status >= 400 {
		return "ERROR"
	}
	return "INFO"
}

// readWithTimeout reads from a stream with a timeout.
// If no data is received within the timeout, it returns an error.
func readWithTimeout(r io.Reader, buf []byte, timeout time.Duration) (int, error) {
	type readResult struct {
		n   int
		err error
	}

	ch := make(chan readResult, 1)
	go func() {
		n, err := r.Read(buf)
		ch <- readResult{n, err}
	}()

	select {
	case res := <-ch:
		return res.n, res.err
	case <-time.After(timeout):
		return 0, fmt.Errorf("stream read timeout after %v", timeout)
	}
}
