package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/google/uuid"
)

func (s *SQLiteLogStore) migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA journal_mode=WAL`,
		`CREATE TABLE IF NOT EXISTS request_logs (
			id TEXT PRIMARY KEY,
			timestamp_ms INTEGER NOT NULL,
			timestamp TEXT NOT NULL,
			level TEXT NOT NULL,
			provider TEXT NOT NULL,
			direction TEXT NOT NULL,
			method TEXT,
			path TEXT,
			status_code INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			connection_id TEXT,
			connection_name TEXT,
			model TEXT,
			request_id TEXT,
			message TEXT,
			error TEXT,
			body_size INTEGER DEFAULT 0,
			input_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			total_tokens INTEGER DEFAULT 0,
			usage_source TEXT,
			cost_input REAL DEFAULT 0,
			cost_output REAL DEFAULT 0,
			cost_total REAL DEFAULT 0,
			currency TEXT DEFAULT 'USD',
			metadata_json TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS model_prices (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			model_pattern TEXT NOT NULL,
			input_per_1m REAL NOT NULL,
			output_per_1m REAL NOT NULL,
			currency TEXT NOT NULL,
			source_note TEXT,
			updated_at_ms INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_time ON request_logs(timestamp_ms DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_connection_time ON request_logs(connection_id, timestamp_ms DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_request ON request_logs(request_id)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_usage_time ON request_logs(total_tokens, timestamp_ms DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_model_prices_lookup ON model_prices(provider, model_pattern)`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate log db: %w", err)
		}
	}

	return s.seedPrices(ctx)
}

func (s *SQLiteLogStore) seedPrices(ctx context.Context) error {
	now := time.Now().UnixMilli()
	defaults := []domain.ModelPrice{
		{Provider: "kiro", ModelPattern: "%sonnet%", InputPer1M: 3, OutputPer1M: 15, Currency: "USD", SourceNote: "Estimated from public Anthropic API token pricing; Kiro billing may differ."},
		{Provider: "kiro", ModelPattern: "%haiku%", InputPer1M: 0.8, OutputPer1M: 4, Currency: "USD", SourceNote: "Estimated from public Anthropic API token pricing; Kiro billing may differ."},
		{Provider: "kiro", ModelPattern: "%deepseek%", InputPer1M: 0.28, OutputPer1M: 0.42, Currency: "USD", SourceNote: "Estimated from public DeepSeek API token pricing; Kiro billing may differ."},
		{Provider: "openai", ModelPattern: "%gpt-5%", InputPer1M: 1.25, OutputPer1M: 10, Currency: "USD", SourceNote: "Default estimate; edit prices if your provider differs."},
		{Provider: "openai-compatible", ModelPattern: "%", InputPer1M: 0, OutputPer1M: 0, Currency: "USD", SourceNote: "Unknown provider price. Configure this before relying on cost."},
	}

	for _, price := range defaults {
		if price.ID == "" {
			price.ID = uuid.New().String()
		}
		_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO model_prices
			(id, provider, model_pattern, input_per_1m, output_per_1m, currency, source_note, updated_at_ms)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			price.ID, price.Provider, price.ModelPattern, price.InputPer1M, price.OutputPer1M,
			price.Currency, price.SourceNote, now)
		if err != nil {
			return fmt.Errorf("seed model prices: %w", err)
		}
	}
	return nil
}
