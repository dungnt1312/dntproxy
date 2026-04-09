package logger

import (
	"log"
	"sync"
	"time"
)

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Provider  string `json:"provider"`
	Message   string `json:"message"`
}

type Logger struct {
	mu          sync.RWMutex
	logs        []LogEntry
	maxSize     int
	subs        chan []LogEntry
	subMu       sync.RWMutex
	subscribers map[chan []LogEntry]bool
}

var (
	defaultLogger *Logger
	once          sync.Once
)

func Get() *Logger {
	once.Do(func() {
		defaultLogger = &Logger{
			maxSize:     1000,
			logs:        make([]LogEntry, 0, 1000),
			subscribers: make(map[chan []LogEntry]bool),
		}
	})
	return defaultLogger
}

func (l *Logger) Add(provider, level, message string) {
	entry := LogEntry{
		Timestamp: time.Now().Format("15:04:05.000"),
		Level:     level,
		Provider:  provider,
		Message:   message,
	}

	log.Printf("[%s] %s | %s", provider, level, message)

	l.mu.Lock()
	l.logs = append(l.logs, entry)
	if len(l.logs) > l.maxSize {
		l.logs = l.logs[len(l.logs)-l.maxSize:]
	}
	logsCopy := make([]LogEntry, len(l.logs))
	copy(logsCopy, l.logs)
	l.mu.Unlock()

	l.subMu.RLock()
	for ch := range l.subscribers {
		select {
		case ch <- logsCopy:
		default:
		}
	}
	l.subMu.RUnlock()
}

func (l *Logger) GetAll() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]LogEntry, len(l.logs))
	copy(result, l.logs)
	return result
}

func (l *Logger) Subscribe() chan []LogEntry {
	l.subMu.Lock()
	defer l.subMu.Unlock()
	ch := make(chan []LogEntry, 10)
	l.subscribers[ch] = true
	return ch
}

func (l *Logger) Unsubscribe(ch chan []LogEntry) {
	l.subMu.Lock()
	defer l.subMu.Unlock()
	delete(l.subscribers, ch)
	close(ch)
}

func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = l.logs[:0]
}

func (l *Logger) AddKiro(format string, args ...interface{}) {
	l.Add("KIRO", "INFO", formatArgs(format, args))
}

func (l *Logger) AddOpenAI(format string, args ...interface{}) {
	l.Add("OPENAI", "INFO", formatArgs(format, args))
}

func (l *Logger) AddError(provider, format string, args ...interface{}) {
	l.Add(provider, "ERROR", formatArgs(format, args))
}

func formatArgs(format string, args []interface{}) string {
	if len(args) == 0 {
		return format
	}
	result := format
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			result += " " + v
		case int:
			result += " " + string(rune('0'+v))
		case error:
			result += " " + v.Error()
		default:
			result += " "
		}
	}
	return result
}
