package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/google/uuid"
)

// ListPrices returns configured model price profiles.
func (s *SQLiteLogStore) ListPrices(ctx context.Context) ([]domain.ModelPrice, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, provider, model_pattern, input_per_1m,
		output_per_1m, currency, source_note, updated_at_ms FROM model_prices
		ORDER BY provider, model_pattern`)
	if err != nil {
		return nil, fmt.Errorf("list model prices: %w", err)
	}
	defer rows.Close()

	var prices []domain.ModelPrice
	for rows.Next() {
		var price domain.ModelPrice
		if err := rows.Scan(&price.ID, &price.Provider, &price.ModelPattern, &price.InputPer1M,
			&price.OutputPer1M, &price.Currency, &price.SourceNote, &price.UpdatedAtMs); err != nil {
			return nil, err
		}
		prices = append(prices, price)
	}
	return prices, rows.Err()
}

// PriceFor finds the most specific price profile for provider/model.
func (s *SQLiteLogStore) PriceFor(ctx context.Context, provider, model string) (*domain.ModelPrice, error) {
	if model == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, provider, model_pattern, input_per_1m,
		output_per_1m, currency, source_note, updated_at_ms FROM model_prices
		WHERE (lower(provider) = lower(?) OR provider = '*') AND lower(?) LIKE lower(model_pattern)
		ORDER BY lower(provider) = lower(?) DESC, length(model_pattern) DESC LIMIT 1`, provider, model, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}
	var price domain.ModelPrice
	if err := rows.Scan(&price.ID, &price.Provider, &price.ModelPattern, &price.InputPer1M,
		&price.OutputPer1M, &price.Currency, &price.SourceNote, &price.UpdatedAtMs); err != nil {
		return nil, err
	}
	return &price, rows.Err()
}

// InsertPrice adds a new model price profile.
func (s *SQLiteLogStore) InsertPrice(ctx context.Context, price *domain.ModelPrice) error {
	if price.ID == "" {
		price.ID = uuid.New().String()
	}
	price.UpdatedAtMs = time.Now().UnixMilli()
	_, err := s.db.ExecContext(ctx, `INSERT INTO model_prices
		(id, provider, model_pattern, input_per_1m, output_per_1m, currency, source_note, updated_at_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		price.ID, price.Provider, price.ModelPattern, price.InputPer1M, price.OutputPer1M,
		price.Currency, price.SourceNote, price.UpdatedAtMs)
	if err != nil {
		return fmt.Errorf("insert model price: %w", err)
	}
	return nil
}

// UpdatePrice modifies an existing model price profile.
func (s *SQLiteLogStore) UpdatePrice(ctx context.Context, price *domain.ModelPrice) error {
	price.UpdatedAtMs = time.Now().UnixMilli()
	res, err := s.db.ExecContext(ctx, `UPDATE model_prices SET
		provider = ?, model_pattern = ?, input_per_1m = ?, output_per_1m = ?,
		currency = ?, source_note = ?, updated_at_ms = ?
		WHERE id = ?`,
		price.Provider, price.ModelPattern, price.InputPer1M, price.OutputPer1M,
		price.Currency, price.SourceNote, price.UpdatedAtMs, price.ID)
	if err != nil {
		return fmt.Errorf("update model price: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("price not found: %s", price.ID)
	}
	return nil
}

// DeletePrice removes a model price profile by ID.
func (s *SQLiteLogStore) DeletePrice(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM model_prices WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete model price: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("price not found: %s", id)
	}
	return nil
}
