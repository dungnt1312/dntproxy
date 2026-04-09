package service

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ComboHandler manages combo fallback and round-robin strategies.
type ComboHandler struct {
	rotationState sync.Map // comboName → int (current index)
}

// NewComboHandler creates a new ComboHandler.
func NewComboHandler() *ComboHandler {
	return &ComboHandler{}
}

// ComboResult represents the result of trying a single model in a combo.
type ComboResult struct {
	OK         bool
	StatusCode int
	Error      string
	RetryAfter string
	Body       []byte
}

// HandleCombo tries models in order (or round-robin) until one succeeds.
// handleSingle is called for each model and should return a ComboResult.
func (ch *ComboHandler) HandleCombo(
	models []string,
	comboName string,
	strategy string,
	handleSingle func(modelStr string) (*ComboResult, error),
) (*ComboResult, error) {
	rotated := ch.getRotatedModels(models, comboName, strategy)

	var lastError string
	var lastStatus int
	var earliestRetry string

	for i, modelStr := range rotated {
		log.Printf("[COMBO] Trying model %d/%d: %s", i+1, len(rotated), modelStr)

		result, err := handleSingle(modelStr)
		if err != nil {
			lastError = err.Error()
			lastStatus = 500
			log.Printf("[COMBO] Model %s threw error, trying next: %s", modelStr, err.Error())
			continue
		}

		if result.OK {
			log.Printf("[COMBO] Model %s succeeded", modelStr)
			return result, nil
		}

		// Track earliest retryAfter
		if result.RetryAfter != "" {
			if earliestRetry == "" || result.RetryAfter < earliestRetry {
				earliestRetry = result.RetryAfter
			}
		}

		// Check if should fallback
		if !shouldComboFallback(result.StatusCode) {
			log.Printf("[COMBO] Model %s failed (no fallback), status=%d", modelStr, result.StatusCode)
			return result, nil
		}

		// Transient error cooldown
		if isTransientStatus(result.StatusCode) {
			log.Printf("[COMBO] Model %s transient %d, waiting 2s", modelStr, result.StatusCode)
			time.Sleep(2 * time.Second)
		}

		lastError = result.Error
		if lastStatus == 0 {
			lastStatus = result.StatusCode
		}
		log.Printf("[COMBO] Model %s failed (%d), trying next", modelStr, result.StatusCode)
	}

	// All models failed
	if lastStatus == 0 {
		lastStatus = 503
	}
	msg := lastError
	if msg == "" {
		msg = "All combo models unavailable"
	}

	return &ComboResult{
		OK:         false,
		StatusCode: lastStatus,
		Error:      msg,
		RetryAfter: earliestRetry,
	}, fmt.Errorf("all combo models failed: %s", msg)
}

func (ch *ComboHandler) getRotatedModels(models []string, comboName string, strategy string) []string {
	if len(models) <= 1 || strategy != "round-robin" {
		return models
	}

	val, _ := ch.rotationState.LoadOrStore(comboName, 0)
	currentIndex := val.(int)

	rotated := make([]string, len(models))
	for i := range models {
		rotated[i] = models[(currentIndex+i)%len(models)]
	}

	nextIndex := (currentIndex + 1) % len(models)
	ch.rotationState.Store(comboName, nextIndex)

	return rotated
}

func shouldComboFallback(status int) bool {
	// Most errors should trigger fallback
	switch status {
	case 400: // Bad request — don't fallback, request itself is wrong
		return false
	default:
		return true
	}
}

func isTransientStatus(status int) bool {
	return status == 502 || status == 503 || status == 504
}
