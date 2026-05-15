package http

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/compressor"
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

		// Extract API key policy from context (set by apiKeyMiddleware)
		policy := extractAPIKeyPolicy(c)

		result := chatService.HandleChat(body, partial.Model, requestID, policy, compressionMetadata(stats))

		if result.Stream != nil {
			// Only aggregate when client EXPLICITLY sends "stream": false.
			// When stream field is absent (nil), default to streaming SSE —
			// most SDK clients (e.g. Vercel AI SDK) handle aggregation client-side
			// and expect streaming even for generateText() calls.
			if partial.Stream != nil && !*partial.Stream {
				defer result.Stream.Close()
				completion, err := aggregateChatCompletion(result.Stream, partial.Model, requestID)
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

// --- Non-streaming aggregation types ---

type chatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []chatCompletionChoice `json:"choices"`
	Usage   *chatCompletionUsage   `json:"usage,omitempty"`
}

type chatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      chatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason,omitempty"`
}

type chatCompletionMessage struct {
	Role      string              `json:"role"`
	Content   string              `json:"content"`
	ToolCalls []chatToolCallEntry `json:"tool_calls,omitempty"`
}

type chatToolCallEntry struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function chatToolCallFn `json:"function"`
}

type chatToolCallFn struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamingChatChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id,omitempty"`
				Type     string `json:"type,omitempty"`
				Function struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *chatCompletionUsage `json:"usage,omitempty"`
}

// aggregateChatCompletion reads an OpenAI-compatible SSE stream and assembles
// a single non-streaming chat completion response.
func aggregateChatCompletion(stream io.Reader, requestedModel, requestID string) (chatCompletionResponse, error) {
	response := chatCompletionResponse{
		ID:      "chatcmpl-" + requestID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []chatCompletionChoice{{Index: 0, Message: chatCompletionMessage{Role: "assistant"}}},
	}

	// Use a map of index → pointer into a separate slice to avoid dangling pointers on append.
	choiceList := []*chatCompletionChoice{&response.Choices[0]}
	choiceMap := map[int]*chatCompletionChoice{0: choiceList[0]}

	scanner := bufio.NewScanner(stream)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxChatBodySize)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var chunk streamingChatChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return chatCompletionResponse{}, err
		}
		if chunk.ID != "" {
			response.ID = chunk.ID
		}
		if chunk.Created != 0 {
			response.Created = chunk.Created
		}
		if chunk.Model != "" {
			response.Model = chunk.Model
		}
		if chunk.Usage != nil {
			response.Usage = chunk.Usage
		}

		for _, chunkChoice := range chunk.Choices {
			choice := choiceMap[chunkChoice.Index]
			if choice == nil {
				newChoice := &chatCompletionChoice{
					Index:   chunkChoice.Index,
					Message: chatCompletionMessage{Role: "assistant"},
				}
				choiceList = append(choiceList, newChoice)
				choiceMap[chunkChoice.Index] = newChoice
			}
			choice = choiceMap[chunkChoice.Index]

			if chunkChoice.Delta.Role != "" {
				choice.Message.Role = chunkChoice.Delta.Role
			}
			if chunkChoice.Delta.Content != "" {
				choice.Message.Content += chunkChoice.Delta.Content
			}

			// Aggregate tool calls
			for _, tc := range chunkChoice.Delta.ToolCalls {
				if tc.Function.Name != "" {
					// New tool call
					choice.Message.ToolCalls = append(choice.Message.ToolCalls, chatToolCallEntry{
						ID:   tc.ID,
						Type: tc.Type,
						Function: chatToolCallFn{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				} else if len(choice.Message.ToolCalls) > 0 {
					// Append arguments to last tool call
					last := &choice.Message.ToolCalls[len(choice.Message.ToolCalls)-1]
					last.Function.Arguments += tc.Function.Arguments
				}
			}

			if chunkChoice.FinishReason != nil {
				choice.FinishReason = *chunkChoice.FinishReason
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return chatCompletionResponse{}, err
	}

	// Rebuild response.Choices from choiceList (avoids dangling pointer issue)
	response.Choices = make([]chatCompletionChoice, len(choiceList))
	for i, cp := range choiceList {
		response.Choices[i] = *cp
	}

	return response, nil
}
