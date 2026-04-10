package port

import (
	"context"

	"github.com/dungnt/dntproxy/internal/domain"
)

// LogStore persists structured logs, usage, and cost summaries.
type LogStore interface {
	Insert(ctx context.Context, entry *domain.LogEntry) error
	List(ctx context.Context, query domain.LogQuery) ([]domain.LogEntry, error)
	Summary(ctx context.Context, query domain.LogQuery) (*domain.LogSummary, error)
	ConnectionSummaries(ctx context.Context, query domain.LogQuery) ([]domain.LogConnectionSummary, error)
	ListPrices(ctx context.Context) ([]domain.ModelPrice, error)
	PriceFor(ctx context.Context, provider, model string) (*domain.ModelPrice, error)
	Clear(ctx context.Context) error
	PurgeOlderThan(ctx context.Context, cutoffMs int64) error
}
