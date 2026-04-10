package storage

import (
	"context"
	"fmt"

	"github.com/dungnt/dntproxy/internal/domain"
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
