package logger

import (
	"context"

	"github.com/dungnt/dntproxy/internal/domain"
)

func (l *Logger) List(query domain.LogQuery) ([]domain.LogEntry, error) {
	if l.store != nil {
		return l.store.List(context.Background(), query)
	}
	return l.GetAll(), nil
}

func (l *Logger) Summary(query domain.LogQuery) (*domain.LogSummary, error) {
	if l.store != nil {
		return l.store.Summary(context.Background(), query)
	}
	return &domain.LogSummary{Currency: "USD"}, nil
}

func (l *Logger) ConnectionSummaries(query domain.LogQuery) ([]domain.LogConnectionSummary, error) {
	if l.store != nil {
		return l.store.ConnectionSummaries(context.Background(), query)
	}
	return nil, nil
}

func (l *Logger) Prices() ([]domain.ModelPrice, error) {
	if l.store != nil {
		return l.store.ListPrices(context.Background())
	}
	return nil, nil
}
