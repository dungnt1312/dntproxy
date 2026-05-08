package logger

import (
	"context"
	"fmt"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

func (l *Logger) List(query domain.LogQuery) ([]domain.LogEntry, error) {
	if l.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return l.store.List(ctx, query)
	}
	return l.GetAll(), nil
}

func (l *Logger) Summary(query domain.LogQuery) (*domain.LogSummary, error) {
	if l.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return l.store.Summary(ctx, query)
	}
	return &domain.LogSummary{Currency: "USD"}, nil
}

func (l *Logger) ConnectionSummaries(query domain.LogQuery) ([]domain.LogConnectionSummary, error) {
	if l.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return l.store.ConnectionSummaries(ctx, query)
	}
	return nil, nil
}

func (l *Logger) DailyStats(query domain.LogQuery) ([]domain.DailyUsageStat, error) {
	if l.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return l.store.DailyStats(ctx, query)
	}
	return nil, nil
}

func (l *Logger) Prices() ([]domain.ModelPrice, error) {
	if l.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return l.store.ListPrices(ctx)
	}
	return nil, nil
}

func (l *Logger) InsertPrice(price *domain.ModelPrice) error {
	if l.store == nil {
		return fmt.Errorf("no log store configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return l.store.InsertPrice(ctx, price)
}

func (l *Logger) UpdatePrice(price *domain.ModelPrice) error {
	if l.store == nil {
		return fmt.Errorf("no log store configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return l.store.UpdatePrice(ctx, price)
}

func (l *Logger) DeletePrice(id string) error {
	if l.store == nil {
		return fmt.Errorf("no log store configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return l.store.DeletePrice(ctx, id)
}
