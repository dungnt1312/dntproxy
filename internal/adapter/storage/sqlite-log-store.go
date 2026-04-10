package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	_ "modernc.org/sqlite"
)

const logRetentionDays = 30

// SQLiteLogStore persists structured request logs in a local SQLite database.
type SQLiteLogStore struct {
	db *sql.DB
}

// NewSQLiteLogStore opens the log database and applies lightweight migrations.
func NewSQLiteLogStore(path string) (*SQLiteLogStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create log db dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open log db: %w", err)
	}

	store := &SQLiteLogStore{db: db}
	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -logRetentionDays).UnixMilli()
	if err := store.PurgeOlderThan(context.Background(), cutoff); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// Close releases the SQLite database handle.
func (s *SQLiteLogStore) Close() error {
	return s.db.Close()
}

// Insert stores a log entry and calculates estimated cost when usage exists.
func (s *SQLiteLogStore) Insert(ctx context.Context, entry *domain.LogEntry) error {
	price, err := s.PriceFor(ctx, entry.Provider, entry.Model)
	if err == nil && price != nil && entry.TotalTokens > 0 {
		entry.CostInput = float64(entry.InputTokens) / 1_000_000 * price.InputPer1M
		entry.CostOutput = float64(entry.OutputTokens) / 1_000_000 * price.OutputPer1M
		entry.CostTotal = entry.CostInput + entry.CostOutput
		entry.Currency = price.Currency
	}
	if entry.Currency == "" {
		entry.Currency = "USD"
	}

	_, err = s.db.ExecContext(ctx, `INSERT INTO request_logs
		(id, timestamp_ms, timestamp, level, provider, direction, method, path, status_code, duration_ms,
		 connection_id, connection_name, model, request_id, message, error, body_size, input_tokens,
		 output_tokens, total_tokens, usage_source, cost_input, cost_output, cost_total, currency, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.TimestampMs, entry.Timestamp, entry.Level, entry.Provider, entry.Direction,
		entry.Method, entry.Path, entry.StatusCode, entry.DurationMs, entry.ConnectionID,
		entry.ConnectionName, entry.Model, entry.RequestID, entry.Message, entry.Error,
		entry.BodySize, entry.InputTokens, entry.OutputTokens, entry.TotalTokens, entry.UsageSource,
		entry.CostInput, entry.CostOutput, entry.CostTotal, entry.Currency, entry.MetadataJSON)
	if err != nil {
		return fmt.Errorf("insert log: %w", err)
	}
	return nil
}

// Clear deletes all logs while keeping price profiles.
func (s *SQLiteLogStore) Clear(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM request_logs`)
	return err
}

// PurgeOlderThan removes logs older than the retention cutoff.
func (s *SQLiteLogStore) PurgeOlderThan(ctx context.Context, cutoffMs int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM request_logs WHERE timestamp_ms < ?`, cutoffMs)
	return err
}
