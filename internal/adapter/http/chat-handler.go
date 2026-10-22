package http

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/compressor"
	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
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

		// Extract model and stream flag from raw body
		var partial struct {
			Model  string `json:"model"`
			Stream *bool  `json:"stream"`
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

		log.Printf("[CHAT] POST /v1/chat/completions | model=%s | stream=%v", partial.Model, partial.Stream)

		// Extract API key policy and tenant from context (set by apiKeyMiddleware).
		policy := extractAPIKeyPolicy(c)
		meta := compressionMetadata(stats)
		meta.TenantID = GetTenantID(c)
		meta.Context = c.Request.Context()
		meta.SessionKey = sessionHeader(c)
		if v, ok := c.Get("apiKeyID"); ok {
			if id, ok := v.(string); ok {
				meta.APIKeyID = id
			}
		}

		result := chatService.HandleChat(body, partial.Model, requestID, policy, meta)

		if result.Stream != nil {
			// Only aggregate when client EXPLICITLY sends "stream": false.
			// When stream field is absent (nil), default to streaming SSE —
			// most SDK clients (e.g. Vercel AI SDK) handle aggregation client-side
			// and expect streaming even for generateText() calls.
			if partial.Stream != nil && !*partial.Stream {
				defer result.Stream.Close()
				completion, err := shared.AggregateChatCompletion(result.Stream, partial.Model, requestID)
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": err.Error()}})
					return
				}
				c.JSON(http.StatusOK, completion)
				return
			}

			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")
			c.Status(http.StatusOK)
			c.Writer.Flush()

			var bytesReceived int64
			done := make(chan struct{})
			defer close(done)
			events := streamChunks(done, result.Stream)
			timeout := time.NewTimer(streamReadTimeout)
			defer timeout.Stop()
			// Keep the connection visibly alive while the upstream is silent
			// (e.g. a model thinking before its first token).
			hb := startSSEHeartbeat(c.Writer, sseKeepAliveInterval)
			defer hb.Stop()
			writeSSEEnd := func(msg string) {
				hb.Stop()
				if msg != "" {
					_, _ = c.Writer.Write([]byte("data: {\"error\":{\"message\":\"" + msg + "\"}}\n\n"))
				}
				_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
				c.Writer.Flush()
			}
			for {
				select {
				case ev, ok := <-events:
					if !ok {
						result.Stream.Close()
						return
					}
					if ev.err != nil && ev.err != io.EOF {
						log.Printf("[CHAT] Stream error: model=%s bytes=%d err=%s", partial.Model, bytesReceived, ev.err)
						writeSSEEnd("stream interrupted")
						result.Stream.Close()
						return
					}
					if len(ev.data) > 0 {
						hb.Lock()
						_, writeErr := c.Writer.Write(ev.data)
						if writeErr == nil {
							c.Writer.Flush()
						}
						hb.Unlock()
						if writeErr != nil {
							result.Stream.Close()
							return
						}
						bytesReceived += int64(len(ev.data))
					}
					if ev.err == io.EOF {
						result.Stream.Close()
						return
					}
					if !timeout.Stop() {
						select {
						case <-timeout.C:
						default:
						}
					}
					timeout.Reset(streamReadTimeout)
				case <-timeout.C:
					log.Printf("[CHAT] Stream timeout after %v | model=%s bytes=%d", streamReadTimeout, partial.Model, bytesReceived)
					writeSSEEnd("stream timeout")
					result.Stream.Close()
					return
				case <-c.Request.Context().Done():
					writeSSEEnd("client disconnected")
					result.Stream.Close()
					return
				}
			}
		}

		writeRetryAfterHeader(c, result.RetryAfter)
		c.JSON(result.StatusCode, gin.H{
			"error": gin.H{"message": result.Error},
		})
	}
}

// writeRetryAfterHeader converts an RFC3339 cooldown timestamp into an HTTP
// Retry-After delta-seconds header. A past or unparseable value is skipped.
func writeRetryAfterHeader(c *gin.Context, retryAfter string) {
	if retryAfter == "" {
		return
	}
	t, err := time.Parse(time.RFC3339, retryAfter)
	if err != nil {
		return
	}
	secs := int(time.Until(t).Seconds())
	if secs <= 0 {
		return
	}
	c.Header("Retry-After", strconv.Itoa(secs))
}

// sessionHeader returns the first non-empty client session header.
func sessionHeader(c *gin.Context) string {
	if v := strings.TrimSpace(c.GetHeader("X-Session-Id")); v != "" {
		return v
	}
	return strings.TrimSpace(c.GetHeader("X-Dntproxy-Session"))
}

func compressionMetadata(stats compressor.Stats) port.RequestMetadata {
	if !stats.LogSavings || stats.CompressedBytes <= 0 || stats.CompressedBytes >= stats.OriginalBytes {
		return port.RequestMetadata{}
	}
	savedBytes := stats.OriginalBytes - stats.CompressedBytes
	ratio := 0.0
	if stats.OriginalBytes > 0 {
		ratio = float64(stats.CompressedBytes) / float64(stats.OriginalBytes)
	}
	return port.RequestMetadata{
		Compression: &domain.CompressionLogMetadata{
			OriginalBytes:       stats.OriginalBytes,
			CompressedBytes:     stats.CompressedBytes,
			SavedBytes:          savedBytes,
			TokensSavedEstimate: stats.TokensSaved,
			Ratio:               ratio,
			Detections:          stats.Detections,
			Skipped:             stats.Skipped,
		},
	}
}

type streamEvent struct {
	data []byte
	err  error
}

func streamChunks(done <-chan struct{}, r io.Reader) <-chan streamEvent {
	out := make(chan streamEvent, 1)

	go func() {
		defer close(out)
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
				case out <- streamEvent{data: chunk}:
				case <-done:
					return
				}
			}
			if err != nil {
				if err != io.EOF || n == 0 {
					select {
					case out <- streamEvent{err: err}:
					case <-done:
					}
				}
				return
			}
		}
	}()

	return out
}
