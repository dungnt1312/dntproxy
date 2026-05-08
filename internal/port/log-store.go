package port

import (
	"context"

	"github.com/dungnt/dntproxy/internal/domain"
)

// LogStore persists structured logs, usage, and cost summaries.
type LogStore interface {
	Insert(ctx context.Context, entry *domain.LogEntry) error
	BatchInsert(ctx context.Context, entries []*domain.LogEntry) error
	List(ctx context.Context, query domain.LogQuery) ([]domain.LogEntry, error)
	Summary(ctx context.Context, query domain.LogQuery) (*domain.LogSummary, error)
	ConnectionSummaries(ctx context.Context, query domain.LogQuery) ([]domain.LogConnectionSummary, error)
	DailyStats(ctx context.Context, query domain.LogQuery) ([]domain.DailyUsageStat, error)
	ListPrices(ctx context.Context) ([]domain.ModelPrice, error)
	PriceFor(ctx context.Context, provider, model string) (*domain.ModelPrice, error)
	InsertPrice(ctx context.Context, price *domain.ModelPrice) error
	UpdatePrice(ctx context.Context, price *domain.ModelPrice) error
	DeletePrice(ctx context.Context, id string) error
	Clear(ctx context.Context) error
	PurgeOlderThan(ctx context.Context, cutoffMs int64) error
}
