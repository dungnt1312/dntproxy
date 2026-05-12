package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestSQLiteLogStoreInsertListSummaryAndRetention(t *testing.T) {
	store, err := NewSQLiteLogStore(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("NewSQLiteLogStore() error = %v", err)
	}
	defer store.Close()

	now := time.Now()
	ctx := context.Background()
	entries := []domain.LogEntry{
		{
			ID:          "client-1",
			Timestamp:   now.Format(time.RFC3339Nano),
			TimestampMs: now.UnixMilli(),
			Level:       "INFO",
			Provider:    "CLIENT",
			Direction:   "response",
			StatusCode:  200,
			RequestID:   "req-1",
			Message:     "Client request",
			Currency:    "USD",
		},
		{
			ID:             "usage-1",
			Timestamp:      now.Format(time.RFC3339Nano),
			TimestampMs:    now.UnixMilli(),
			Level:          "INFO",
			Provider:       "KIRO",
			Direction:      "usage",
			ConnectionID:   "conn-1",
			ConnectionName: "Main",
			Model:          "claude-sonnet-4.5",
			RequestID:      "req-1",
			Message:        "Usage",
			InputTokens:    1000,
			OutputTokens:   100,
			TotalTokens:    1100,
			UsageSource:    "provider_metrics",
		},
		{
			ID:             "usage-opus-1",
			Timestamp:      now.Format(time.RFC3339Nano),
			TimestampMs:    now.UnixMilli(),
			Level:          "INFO",
			Provider:       "KIRO",
			Direction:      "usage",
			ConnectionID:   "conn-1",
			ConnectionName: "Main",
			Model:          "claude-opus-4.6",
			RequestID:      "req-opus-1",
			Message:        "Usage",
			InputTokens:    1000,
			OutputTokens:   100,
			TotalTokens:    1100,
			UsageSource:    "provider_metrics",
		},
	}

	for i := range entries {
		if err := store.Insert(ctx, &entries[i]); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
	}

	logs, err := store.List(ctx, domain.LogQuery{Range: "24h", Limit: 10})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("List() len = %d, want 3", len(logs))
	}
	for _, entry := range logs {
		if entry.Model == "claude-opus-4.6" && entry.CostTotal <= 0 {
			t.Fatalf("opus log cost = %f, want > 0", entry.CostTotal)
		}
	}

	summary, err := store.Summary(ctx, domain.LogQuery{Range: "24h"})
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if summary.Requests != 1 || summary.TotalTokens != 2200 || summary.CostTotal <= 0 {
		t.Fatalf("Summary() = %+v, want requests=1 tokens=2200 cost>0", summary)
	}

	if err := store.PurgeOlderThan(ctx, now.Add(time.Second).UnixMilli()); err != nil {
		t.Fatalf("PurgeOlderThan() error = %v", err)
	}
	logs, err = store.List(ctx, domain.LogQuery{Range: "30d", Limit: 10})
	if err != nil {
		t.Fatalf("List() after purge error = %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("List() after purge len = %d, want 0", len(logs))
	}
}

func TestSQLiteLogStoreEmptyListsAreNotNil(t *testing.T) {
	store, err := NewSQLiteLogStore(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("NewSQLiteLogStore() error = %v", err)
	}
	defer store.Close()

	logs, err := store.List(context.Background(), domain.LogQuery{Range: "24h"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if logs == nil {
		t.Fatal("List() returned nil slice, want empty slice")
	}

	connections, err := store.ConnectionSummaries(context.Background(), domain.LogQuery{Range: "24h"})
	if err != nil {
		t.Fatalf("ConnectionSummaries() error = %v", err)
	}
	if connections == nil {
		t.Fatal("ConnectionSummaries() returned nil slice, want empty slice")
	}
}
