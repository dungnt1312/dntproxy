package storage

import (
	"context"
	"fmt"
	"log"
	"strings"
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
			metadata_json TEXT,
			request_body TEXT,
			response_body TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS model_prices (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			model_pattern TEXT NOT NULL,
			input_per_1m REAL NOT NULL,
			output_per_1m REAL NOT NULL,
			currency TEXT NOT NULL,
			source_note TEXT,
			updated_at_ms INTEGER NOT NULL,
			is_user_edited INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_time ON request_logs(timestamp_ms DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_connection_time ON request_logs(connection_id, timestamp_ms DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_provider_time ON request_logs(provider, timestamp_ms DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_level_time ON request_logs(level, timestamp_ms DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_request ON request_logs(request_id)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_usage_time ON request_logs(total_tokens, timestamp_ms DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_model_prices_lookup ON model_prices(provider, model_pattern)`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate log db: %w", err)
		}
	}

	// Graceful column additions for existing databases
	for _, altStmt := range []string{
		`ALTER TABLE request_logs ADD COLUMN request_body TEXT`,
		`ALTER TABLE request_logs ADD COLUMN response_body TEXT`,
		`ALTER TABLE request_logs ADD COLUMN tenant_id TEXT`,
		`ALTER TABLE model_prices ADD COLUMN is_user_edited INTEGER DEFAULT 0`,
	} {
		s.db.ExecContext(ctx, altStmt) //nolint:errcheck
	}

	s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_logs_tenant_time ON request_logs(tenant_id, timestamp_ms DESC)`) //nolint:errcheck

	return s.syncPricesFromRegistry(ctx)
}

// syncPricesFromRegistry upserts prices from ModelDefinition into model_prices.
// Rows marked is_user_edited=1 are skipped (user has manually set them).
// Rows with is_user_edited=0 are updated to match the current ModelDefinition values.
func (s *SQLiteLogStore) syncPricesFromRegistry(ctx context.Context) error {
	now := time.Now().UnixMilli()
	registry := domain.DefaultModelRegistry()

	// Build price list from all active models in the registry
	seen := make(map[string]bool)
	for key, def := range registry.Models {
		if !def.IsActive {
			continue
		}
		// key format: "provider/model-id"
		idx := strings.Index(key, "/")
		if idx < 0 {
			continue
		}
		provider := key[:idx]
		modelID := key[idx+1:]

		// Generate a LIKE pattern that matches this specific model
		pattern := "%" + modelID + "%"
		uniqKey := provider + ":" + pattern

		// Deduplicate: same provider+pattern means same price row
		dedupKey := provider + ":" + modelID
		if seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true

		// upsert: INSERT OR REPLACE but preserve is_user_edited
		_, err := s.db.ExecContext(ctx, `INSERT INTO model_prices
			(id, provider, model_pattern, input_per_1m, output_per_1m, currency, source_note, updated_at_ms, is_user_edited)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
			ON CONFLICT(provider, model_pattern) DO UPDATE SET
				input_per_1m = CASE WHEN is_user_edited THEN input_per_1m ELSE excluded.input_per_1m END,
				output_per_1m = CASE WHEN is_user_edited THEN output_per_1m ELSE excluded.output_per_1m END,
				source_note = CASE WHEN is_user_edited THEN source_note ELSE excluded.source_note END,
				updated_at_ms = excluded.updated_at_ms`,
			uuid.New().String(), provider, pattern, def.InputPrice, def.OutputPrice,
			"USD", "Auto-synced from ModelDefinition", now)
		if err != nil {
			log.Printf("[PRICES] Failed upsert %s:%s: %s", provider, pattern, err)
			continue
		}
		_ = uniqKey
	}

	log.Printf("[PRICES] Synced %d model prices from ModelDefinition", len(seen))
	return nil
}
