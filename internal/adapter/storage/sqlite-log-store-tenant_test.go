package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestSQLiteLogStoreTenantIsolation(t *testing.T) {
	store, err := NewSQLiteLogStore(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("NewSQLiteLogStore() error = %v", err)
	}
	defer store.Close()

	now := time.Now()
	ctx := context.Background()

	entries := []*domain.LogEntry{
		{
			ID:          "legacy-1",
			Timestamp:   now.Format(time.RFC3339Nano),
			TimestampMs: now.UnixMilli(),
			Level:       "INFO",
			Provider:    "OPENAI",
			Direction:   "response",
			StatusCode:  200,
			RequestID:   "r1",
			Message:     "legacy request",
			TenantID:    "",
		},
		{
			ID:          "acme-1",
			Timestamp:   now.Format(time.RFC3339Nano),
			TimestampMs: now.UnixMilli(),
			Level:       "INFO",
			Provider:    "OPENAI",
			Direction:   "response",
			StatusCode:  200,
			RequestID:   "r2",
			Message:     "acme request",
			TenantID:    "acme",
		},
		{
			ID:          "globex-1",
			Timestamp:   now.Format(time.RFC3339Nano),
			TimestampMs: now.UnixMilli(),
			Level:       "INFO",
			Provider:    "OPENAI",
			Direction:   "response",
			StatusCode:  200,
			RequestID:   "r3",
			Message:     "globex request",
			TenantID:    "globex",
		},
	}

	if err := store.BatchInsert(ctx, entries); err != nil {
		t.Fatalf("BatchInsert() error = %v", err)
	}

	// Legacy query (no tenant filter) sees all 3
	logs, err := store.List(ctx, domain.LogQuery{Range: "24h", Limit: 100})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(logs) != 3 {
		t.Errorf("legacy List: got %d logs, want 3", len(logs))
	}

	// acme query sees only its own
	logs, err = store.List(ctx, domain.LogQuery{Range: "24h", Limit: 100, TenantID: "acme"})
	if err != nil {
		t.Fatalf("List(acme) error = %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("acme List: got %d logs, want 1", len(logs))
	}
	if len(logs) == 1 && logs[0].ID != "acme-1" {
		t.Errorf("acme List: got log %s, want acme-1", logs[0].ID)
	}

	// globex query sees only its own
	logs, err = store.List(ctx, domain.LogQuery{Range: "24h", Limit: 100, TenantID: "globex"})
	if err != nil {
		t.Fatalf("List(globex) error = %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("globex List: got %d logs, want 1", len(logs))
	}

	// Summary per tenant
	summary, err := store.Summary(ctx, domain.LogQuery{Range: "24h", TenantID: "acme"})
	if err != nil {
		t.Fatalf("Summary(acme) error = %v", err)
	}
	if summary.Requests != 1 {
		t.Errorf("acme Summary.Requests = %d, want 1", summary.Requests)
	}

	// DailyStats tenant filter — acme only 1 request
	daily, err := store.DailyStats(ctx, domain.LogQuery{Range: "7d", TenantID: "acme"})
	if err != nil {
		t.Fatalf("DailyStats(acme) error = %v", err)
	}
	var acmeReqCount int
	for _, d := range daily {
		acmeReqCount += d.Requests
	}
	if acmeReqCount != 1 {
		t.Errorf("acme DailyStats total requests = %d, want 1", acmeReqCount)
	}
}

func TestSQLiteLogStoreTenantRoundTrip(t *testing.T) {
	store, err := NewSQLiteLogStore(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("NewSQLiteLogStore() error = %v", err)
	}
	defer store.Close()

	now := time.Now()
	entry := &domain.LogEntry{
		ID:          "t-1",
		Timestamp:   now.Format(time.RFC3339Nano),
		TimestampMs: now.UnixMilli(),
		Level:       "INFO",
		Provider:    "TEST",
		Direction:   "response",
		StatusCode:  200,
		RequestID:   "rt1",
		Message:     "tenant test",
		TenantID:    "my-tenant",
	}
	if err := store.Insert(context.Background(), entry); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Fetchable by own tenant
	logs, err := store.List(context.Background(), domain.LogQuery{Range: "24h", TenantID: "my-tenant"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("got %d logs, want 1", len(logs))
	}
	if logs[0].TenantID != "my-tenant" {
		t.Errorf("got TenantID %q, want my-tenant", logs[0].TenantID)
	}

	// Not visible to other tenant
	logs, err = store.List(context.Background(), domain.LogQuery{Range: "24h", TenantID: "other"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("other tenant: got %d logs, want 0", len(logs))
	}
}
