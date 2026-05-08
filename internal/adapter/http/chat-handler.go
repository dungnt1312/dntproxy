package http

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/compressor"
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

func chatHandler(chatService port.ChatService, store port.CredentialStore, comp *compressor.Compressor) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New().String()

		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxChatBodySize+1))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Failed to read body"}})
			return
		}
		if len(body) > maxChatBodySize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"message": "Request body exceeds 10MB limit"}})
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

		// Compress tool result content before forwarding
		body, stats := comp.Compress(body)
		if stats.CompressedBytes > 0 && stats.CompressedBytes < stats.OriginalBytes {
			log.Printf("[COMPRESS] orig=%d comp=%d saved=%d tokens",
				stats.OriginalBytes, stats.CompressedBytes, stats.TokensSaved)
		}

		log.Printf("[CHAT] POST /v1/chat/completions | model=%s", partial.Model)

		result := chatService.HandleChat(body, partial.Model, requestID)

		if result.Stream != nil {
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")
			c.Status(http.StatusOK)
			c.Writer.Flush()

			var bytesReceived int64
			done := make(chan struct{})
			defer close(done)
			chunks, errs := streamChunks(done, result.Stream)
			timeout := time.NewTimer(streamReadTimeout)
			defer timeout.Stop()
			for {
				select {
				case chunk, ok := <-chunks:
					if !ok {
						result.Stream.Close()
						return
					}
					if len(chunk) > 0 {
						if _, writeErr := c.Writer.Write(chunk); writeErr != nil {
							result.Stream.Close()
							return
						}
						c.Writer.Flush()
						bytesReceived += int64(len(chunk))
					}
					if !timeout.Stop() {
						select {
						case <-timeout.C:
						default:
						}
					}
					timeout.Reset(streamReadTimeout)
				case readErr, ok := <-errs:
					if ok && readErr != nil && readErr != io.EOF {
						log.Printf("[CHAT] Stream error: model=%s bytes=%d err=%s", partial.Model, bytesReceived, readErr)
					}
					result.Stream.Close()
					return
				case <-timeout.C:
					log.Printf("[CHAT] Stream timeout after %v | model=%s bytes=%d", streamReadTimeout, partial.Model, bytesReceived)
					result.Stream.Close()
					return
				case <-c.Request.Context().Done():
					result.Stream.Close()
					return
				}
			}
		}

		c.JSON(result.StatusCode, gin.H{
			"error": gin.H{"message": result.Error},
		})
	}
}

func streamChunks(done <-chan struct{}, r io.Reader) (<-chan []byte, <-chan error) {
	chunks := make(chan []byte)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		buf := make([]byte, 4096)
		for {
			select {
			case <-done:
				return
			default:
			}

			n, err := r.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				select {
				case chunks <- chunk:
				case <-done:
					return
				}
			}
			if err != nil {
				select {
				case errs <- err:
				case <-done:
				}
				return
			}
		}
	}()

	return chunks, errs
}
