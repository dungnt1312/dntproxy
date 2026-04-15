package logger

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// AsyncWriter batches log entries and writes them to SQLite asynchronously.
// It also maintains an in-memory price cache to avoid per-insert DB queries.
type AsyncWriter struct {
	store      port.LogStore
	queue      chan *domain.LogEntry
	priceCache sync.Map // "provider:model_pattern" → *domain.ModelPrice
	wg         sync.WaitGroup
	cancel     context.CancelFunc
}

// NewAsyncWriter creates an async writer backed by the given LogStore.
func NewAsyncWriter(store port.LogStore) *AsyncWriter {
	return &AsyncWriter{
		store: store,
		queue: make(chan *domain.LogEntry, 2048),
	}
}

// Start launches the background writer goroutine and loads the price cache.
func (w *AsyncWriter) Start(ctx context.Context) {
	ctx, w.cancel = context.WithCancel(ctx)

	// Warm price cache
	w.reloadPriceCache()

	w.wg.Add(1)
	go w.loop(ctx)
}

// Stop drains remaining entries and shuts down the writer.
func (w *AsyncWriter) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
}

// Enqueue adds a log entry to the async write queue (non-blocking).
func (w *AsyncWriter) Enqueue(entry *domain.LogEntry) {
	select {
	case w.queue <- entry:
	default:
		log.Printf("[LOG] Async write queue full, dropping entry: %s", entry.Message)
	}
}

// InvalidatePriceCache reloads the price cache from the DB.
// Call after price CRUD operations.
func (w *AsyncWriter) InvalidatePriceCache() {
	w.reloadPriceCache()
}

func (w *AsyncWriter) loop(ctx context.Context) {
	defer w.wg.Done()

	batch := make([]*domain.LogEntry, 0, 64)
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case entry := <-w.queue:
			w.enrichCost(entry)
			batch = append(batch, entry)
			if len(batch) >= 50 {
				w.flushBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				w.flushBatch(batch)
				batch = batch[:0]
			}

		case <-ctx.Done():
			// Drain remaining
			close(w.queue)
			for entry := range w.queue {
				w.enrichCost(entry)
				batch = append(batch, entry)
			}
			if len(batch) > 0 {
				w.flushBatch(batch)
			}
			return
		}
	}
}

func (w *AsyncWriter) flushBatch(batch []*domain.LogEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := w.store.BatchInsert(ctx, batch); err != nil {
		log.Printf("[LOG] Failed to persist logs batch: %s", err)
	}
}

func (w *AsyncWriter) enrichCost(entry *domain.LogEntry) {
	if entry.TotalTokens <= 0 || entry.Model == "" {
		return
	}

	price := w.lookupPrice(entry.Provider, entry.Model)
	if price == nil {
		return
	}

	entry.CostInput = float64(entry.InputTokens) / 1_000_000 * price.InputPer1M
	entry.CostOutput = float64(entry.OutputTokens) / 1_000_000 * price.OutputPer1M
	entry.CostTotal = entry.CostInput + entry.CostOutput
	entry.Currency = price.Currency
}

func (w *AsyncWriter) lookupPrice(provider, model string) *domain.ModelPrice {
	provider = strings.ToLower(provider)
	model = strings.ToLower(model)

	var bestMatch *domain.ModelPrice
	var bestLen int

	w.priceCache.Range(func(key, value interface{}) bool {
		price := value.(*domain.ModelPrice)
		priceProv := strings.ToLower(price.Provider)
		pricePattern := strings.ToLower(price.ModelPattern)

		if priceProv != provider && priceProv != "*" {
			return true
		}

		if matchLikePattern(model, pricePattern) {
			// Prefer provider-specific over wildcard, and longer patterns
			providerMatch := priceProv == provider
			patternLen := len(pricePattern)
			isBetter := false

			if bestMatch == nil {
				isBetter = true
			} else if providerMatch && strings.ToLower(bestMatch.Provider) == "*" {
				isBetter = true
			} else if patternLen > bestLen {
				isBetter = true
			}

			if isBetter {
				bestMatch = price
				bestLen = patternLen
			}
		}
		return true
	})

	return bestMatch
}

// matchLikePattern matches a SQL LIKE pattern (using % as wildcard).
func matchLikePattern(value, pattern string) bool {
	// Simple LIKE matching: only handles leading/trailing % for common patterns
	pattern = strings.ToLower(pattern)
	value = strings.ToLower(value)

	if pattern == "%" {
		return true
	}

	if strings.HasPrefix(pattern, "%") && strings.HasSuffix(pattern, "%") {
		return strings.Contains(value, pattern[1:len(pattern)-1])
	}
	if strings.HasPrefix(pattern, "%") {
		return strings.HasSuffix(value, pattern[1:])
	}
	if strings.HasSuffix(pattern, "%") {
		return strings.HasPrefix(value, pattern[:len(pattern)-1])
	}
	return value == pattern
}

func (w *AsyncWriter) reloadPriceCache() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := w.store.ListPrices(ctx)
	if err != nil {
		log.Printf("[LOG] Failed to load price cache: %s", err)
		return
	}

	// Clear and reload
	w.priceCache.Range(func(key, _ interface{}) bool {
		w.priceCache.Delete(key)
		return true
	})
	for i := range prices {
		key := prices[i].Provider + ":" + prices[i].ModelPattern
		w.priceCache.Store(key, &prices[i])
	}
}
