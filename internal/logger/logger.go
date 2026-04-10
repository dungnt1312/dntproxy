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
	subMu       sync.RWMutex
	subscribers map[chan []domain.LogEntry]bool
}

var (
	defaultLogger *Logger
	once          sync.Once
)

func Get() *Logger {
	once.Do(func() {
		defaultLogger = &Logger{
			maxSize:     1000,
			logs:        make([]domain.LogEntry, 0, 1000),
			subscribers: make(map[chan []domain.LogEntry]bool),
		}
	})
	return defaultLogger
}

// Init attaches persistent storage to the process-wide logger.
func Init(store port.LogStore) {
	Get().store = store
}

func (l *Logger) Add(provider, level, message string) {
	l.AddEntry(domain.LogEntry{
		Provider:  provider,
		Level:     level,
		Direction: "event",
		Message:   message,
	})
}

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
	if entry.Currency == "" {
		entry.Currency = "USD"
	}

	log.Printf("[%s] %s | %s", entry.Provider, entry.Level, entry.Message)

	if l.store != nil {
		if err := l.store.Insert(context.Background(), &entry); err != nil {
			log.Printf("[LOG] Failed to persist log: %s", err)
		}
	}

	l.mu.Lock()
	l.logs = append(l.logs, entry)
	if len(l.logs) > l.maxSize {
		l.logs = l.logs[len(l.logs)-l.maxSize:]
	}
	logsCopy := make([]domain.LogEntry, len(l.logs))
	copy(logsCopy, l.logs)
	l.mu.Unlock()

	l.broadcast(logsCopy)
}

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

func (l *Logger) GetAll() []domain.LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]domain.LogEntry, len(l.logs))
	copy(result, l.logs)
	return result
}

func (l *Logger) Subscribe() chan []domain.LogEntry {
	l.subMu.Lock()
	defer l.subMu.Unlock()
	ch := make(chan []domain.LogEntry, 10)
	l.subscribers[ch] = true
	return ch
}

func (l *Logger) Unsubscribe(ch chan []domain.LogEntry) {
	l.subMu.Lock()
	defer l.subMu.Unlock()
	delete(l.subscribers, ch)
	close(ch)
}

func (l *Logger) Clear() {
	if l.store != nil {
		if err := l.store.Clear(context.Background()); err != nil {
			log.Printf("[LOG] Failed to clear persisted logs: %s", err)
		}
	}
	l.mu.Lock()
	l.logs = l.logs[:0]
	logsCopy := make([]domain.LogEntry, 0)
	l.mu.Unlock()
	l.broadcast(logsCopy)
}

func (l *Logger) broadcast(logsCopy []domain.LogEntry) {
	l.subMu.RLock()
	for ch := range l.subscribers {
		select {
		case ch <- logsCopy:
		default:
		}
	}
	l.subMu.RUnlock()
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
