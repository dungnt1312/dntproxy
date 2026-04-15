package logger

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/google/uuid"
)

type Logger struct {
	mu          sync.RWMutex
	logs        []domain.LogEntry
	maxSize     int
	store       port.LogStore
	writer      *AsyncWriter
	subMu       sync.RWMutex
	subscribers map[chan *domain.LogEntry]bool
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Get returns the process-wide logger.
func Get() *Logger {
	once.Do(func() {
		defaultLogger = &Logger{
			maxSize:     1000,
			logs:        make([]domain.LogEntry, 0, 1000),
			subscribers: make(map[chan *domain.LogEntry]bool),
		}
	})
	return defaultLogger
}

// Init attaches persistent storage to the process-wide logger and starts the async writer.
func Init(store port.LogStore) {
	l := Get()
	l.store = store
	l.writer = NewAsyncWriter(store)
	l.writer.Start(context.Background())
}

// Stop flushes pending async logs to storage and stops the writer.
func (l *Logger) Stop() {
	if l.writer != nil {
		l.writer.Stop()
	}
}

// Add logs a general event (backward compatibility for non-request logs).
func (l *Logger) Add(provider, level, message string) {
	l.AddEntry(domain.LogEntry{
		Provider:  provider,
		Level:     level,
		Direction: "event",
		Message:   message,
	})
}

// AddEntry directly injects a structured log entry into the stream.
// Used primarily by non-proxy routines (e.g. background tasks, tunnel logic).
func (l *Logger) AddEntry(entry domain.LogEntry) {
	now := time.Now()
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.TimestampMs == 0 {
		entry.TimestampMs = now.UnixMilli()
	}
	if entry.Timestamp == "" {
		entry.Timestamp = now.Format(time.RFC3339Nano)
	}
	if entry.Level == "" {
		entry.Level = "INFO"
	}
	if entry.Provider == "" {
		entry.Provider = "APP"
	}
	if entry.Direction == "" {
		entry.Direction = "event"
	}

	// For legacy compatibility, print to console if it's an event or error
	// Request logs are handled exclusively by reqlog.go printing.
	if entry.Direction == "event" || entry.Direction == "usage" || entry.Level == "ERROR" {
		log.Printf("[%s] %s | %s", entry.Provider, entry.Level, entry.Message)
	}

	// Async queue insert
	if l.writer != nil {
		l.writer.Enqueue(&entry)
	}

	// Maintain in-memory buffer (optional, for first-load fallback if DB missing)
	l.mu.Lock()
	l.logs = append(l.logs, entry)
	if len(l.logs) > l.maxSize {
		l.logs = l.logs[len(l.logs)-l.maxSize:]
	}
	l.mu.Unlock()

	// Delta broadcast
	l.broadcast(&entry)
}

// AddUsage logs usage directly (backward compatibility).
func (l *Logger) AddUsage(provider, requestID, connectionID, connectionName, model string, inputTokens, outputTokens int, source string) {
	total := inputTokens + outputTokens
	if total <= 0 {
		return
	}
	l.AddEntry(domain.LogEntry{
		Level:          "INFO",
		Provider:       provider,
		Direction:      "usage",
		ConnectionID:   connectionID,
		ConnectionName: connectionName,
		Model:          model,
		RequestID:      requestID,
		Message:        fmt.Sprintf("Usage captured: input=%d output=%d total=%d source=%s", inputTokens, outputTokens, total, source),
		InputTokens:    inputTokens,
		OutputTokens:   outputTokens,
		TotalTokens:    total,
		UsageSource:    source,
	})
}

// GetAll returns a snapshot of the in-memory buffer.
func (l *Logger) GetAll() []domain.LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]domain.LogEntry, len(l.logs))
	copy(result, l.logs)
	return result
}

// Subscribe returns a channel that emits new log entries as they arrive.
func (l *Logger) Subscribe() chan *domain.LogEntry {
	l.subMu.Lock()
	defer l.subMu.Unlock()
	ch := make(chan *domain.LogEntry, 100)
	l.subscribers[ch] = true
	return ch
}

// Unsubscribe removes a channel from the broadcast list.
func (l *Logger) Unsubscribe(ch chan *domain.LogEntry) {
	l.subMu.Lock()
	defer l.subMu.Unlock()
	delete(l.subscribers, ch)
	close(ch)
}

// Clear resets the log buffer and truncates the SQLite table.
func (l *Logger) Clear() {
	if l.store != nil {
		if err := l.store.Clear(context.Background()); err != nil {
			log.Printf("[LOG] Failed to clear persisted logs: %s", err)
		}
	}
	l.mu.Lock()
	l.logs = l.logs[:0]
	l.mu.Unlock()
}

func (l *Logger) broadcast(entry *domain.LogEntry) {
	l.subMu.RLock()
	for ch := range l.subscribers {
		select {
		case ch <- entry:
		default:
			// slow consumer, skip
		}
	}
	l.subMu.RUnlock()
}

// InvalidatePriceCache forces the async writer to reload its price snapshot.
func (l *Logger) InvalidatePriceCache() {
	if l.writer != nil {
		l.writer.InvalidatePriceCache()
	}
}

// AddKiro logs a Kiro-specific message.
func (l *Logger) AddKiro(format string, args ...interface{}) {
	l.Add("KIRO", "INFO", fmt.Sprintf(format, args...))
}

// AddOpenAI logs an OpenAI-specific message.
func (l *Logger) AddOpenAI(format string, args ...interface{}) {
	l.Add("OPENAI", "INFO", fmt.Sprintf(format, args...))
}

// AddError logs an error message for a provider.
func (l *Logger) AddError(provider, format string, args ...interface{}) {
	l.Add(provider, "ERROR", fmt.Sprintf(format, args...))
}
