package service

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestHandleCombo_ClientAbortDoesNotTryNextModel(t *testing.T) {
	ch := NewComboHandler()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	cancel()

	result, err := ch.HandleCombo(ctx, []string{"a/model", "b/model"}, "c", "fallback", func(modelStr string) (*ComboResult, error) {
		calls++
		return &ComboResult{OK: true, Stream: io.NopCloser(nil)}, nil
	})
	if calls != 0 {
		t.Fatalf("handleSingle called %d times, want 0 after cancel", calls)
	}
	if result == nil || result.StatusCode != 499 || !result.Terminal {
		t.Fatalf("result = %+v, want 499 terminal", result)
	}
	if err == nil {
		t.Fatal("expected cancel error")
	}
}

func TestHandleCombo_499ResultDoesNotTryNextModel(t *testing.T) {
	ch := NewComboHandler()
	calls := 0
	result, _ := ch.HandleCombo(context.Background(), []string{"a/model", "b/model"}, "c", "fallback", func(modelStr string) (*ComboResult, error) {
		calls++
		return &ComboResult{OK: false, StatusCode: 499, Error: "client disconnected"}, errors.New("context canceled")
	})
	if calls != 1 {
		t.Fatalf("handleSingle called %d times, want 1", calls)
	}
	if result == nil || result.StatusCode != 499 || !result.Terminal {
		t.Fatalf("result = %+v, want 499 terminal", result)
	}
}
