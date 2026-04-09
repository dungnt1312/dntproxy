package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/dungnt/dntproxy/internal/adapter/kiro"
	openaiAdapter "github.com/dungnt/dntproxy/internal/adapter/openai"
	"github.com/dungnt/dntproxy/internal/port"
)

// ChatService orchestrates: resolve model → get credentials → execute → stream.
type ChatService struct {
	resolver        *ModelResolver
	accountSelector *AccountSelector
	comboHandler    *ComboHandler
	kiroExecutor    *kiro.Executor
	openaiExecutor  *openaiAdapter.Executor
	store           port.CredentialStore
}

// NewChatService creates a new ChatService.
func NewChatService(store port.CredentialStore) *ChatService {
	return &ChatService{
		resolver:        NewModelResolver(store),
		accountSelector: NewAccountSelector(store),
		comboHandler:    NewComboHandler(),
		kiroExecutor:    kiro.NewExecutor(),
		openaiExecutor:  openaiAdapter.NewExecutor(),
		store:           store,
	}
}

// ChatResult is the result of a chat request.
type ChatResult struct {
	// Stream is the SSE stream reader (nil on error).
	Stream io.ReadCloser
	// StatusCode is the HTTP status code.
	StatusCode int
	// Error message (empty on success).
	Error string
}

// HandleChat processes a chat completion request.
func (s *ChatService) HandleChat(body []byte, modelStr string) *ChatResult {
	// Check if model is a combo
	comboModels, err := s.resolver.GetComboModels(modelStr)
	if err != nil {
		log.Printf("[CHAT] Error checking combo: %s", err)
	}

	if comboModels != nil {
		return s.handleComboChat(body, modelStr, comboModels)
	}

	// Single model request
	return s.handleSingleModel(body, modelStr)
}

func (s *ChatService) handleComboChat(body []byte, comboName string, models []string) *ChatResult {
	settings, _ := s.store.GetSettings()
	strategy := "fallback"
	if settings != nil && settings.ComboStrategy != "" {
		strategy = settings.ComboStrategy
	}
	// Check combo-specific strategy
	if settings != nil && settings.ComboStrategies != nil {
		if cs, ok := settings.ComboStrategies[comboName]; ok && cs != "" {
			strategy = cs
		}
	}

	log.Printf("[CHAT] Combo \"%s\" with %d models (strategy: %s)", comboName, len(models), strategy)

	result, _ := s.comboHandler.HandleCombo(models, comboName, strategy, func(modelStr string) (*ComboResult, error) {
		chatResult := s.handleSingleModel(body, modelStr)

		if chatResult.Stream != nil && chatResult.StatusCode == 200 {
			return &ComboResult{
				OK:         true,
				StatusCode: 200,
			}, nil
		}

		// For combo fallback, we need to return the error info
		// But if we have a stream, we can't easily inspect it
		// So we treat non-200 as failure
		return &ComboResult{
			OK:         false,
			StatusCode: chatResult.StatusCode,
			Error:      chatResult.Error,
		}, nil
	})

	if result != nil && result.OK {
		// The combo handler doesn't carry the stream through — re-execute the successful model
		// This is a simplification; in production you'd want to pass the stream through
		// For now, the combo handler identifies which model works, then we re-execute
	}

	// Fallback: try each model directly until one works
	for _, modelStr := range models {
		chatResult := s.handleSingleModel(body, modelStr)
		if chatResult.StatusCode == 200 {
			return chatResult
		}
		log.Printf("[COMBO] Model %s failed (%d), trying next", modelStr, chatResult.StatusCode)
	}

	return &ChatResult{
		StatusCode: 503,
		Error:      "All combo models unavailable",
	}
}

func (s *ChatService) handleSingleModel(body []byte, modelStr string) *ChatResult {
	// Resolve model
	modelInfo, err := s.resolver.Resolve(modelStr)
	if err != nil {
		return &ChatResult{StatusCode: 400, Error: fmt.Sprintf("resolve model: %s", err)}
	}

	if modelInfo.Provider == "" {
		// Could be a combo — try again
		comboModels, _ := s.resolver.GetComboModels(modelStr)
		if comboModels != nil {
			return s.handleComboChat(body, modelStr, comboModels)
		}
		return &ChatResult{StatusCode: 400, Error: "Invalid model format"}
	}

	provider := modelInfo.Provider
	model := modelInfo.Model

	log.Printf("[ROUTING] %s → %s/%s", modelStr, provider, model)

	// Get the executor for this provider
	executor := s.getExecutor(provider)
	if executor == nil {
		return &ChatResult{StatusCode: 400, Error: fmt.Sprintf("Provider '%s' not supported", provider)}
	}

	// Update model in request body
	var bodyMap map[string]interface{}
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		return &ChatResult{StatusCode: 400, Error: "Invalid JSON body"}
	}
	bodyMap["model"] = model
	updatedBody, _ := json.Marshal(bodyMap)

	// Try accounts with fallback
	excludeIDs := make(map[string]bool)

	for {
		creds, err := s.accountSelector.SelectCredentials(provider, excludeIDs, model)
		if err != nil {
			if len(excludeIDs) == 0 {
				return &ChatResult{StatusCode: 404, Error: fmt.Sprintf("No active credentials for provider: %s", provider)}
			}
			return &ChatResult{StatusCode: 503, Error: "All accounts unavailable"}
		}

		log.Printf("[AUTH] Using %s account: %s", provider, creds.ConnectionName)

		stream, status, execErr := executor.Execute(model, updatedBody, creds)
		if execErr == nil && status == 200 {
			// Success — clear error state
			s.accountSelector.ClearError(creds.ConnectionID, model)
			return &ChatResult{Stream: stream, StatusCode: 200}
		}

		// Error — mark account unavailable and try next
		errMsg := ""
		if execErr != nil {
			errMsg = execErr.Error()
		}
		log.Printf("[AUTH] Account %s failed (%d): %s", creds.ConnectionName, status, errMsg)

		s.accountSelector.MarkUnavailable(creds.ConnectionID, status, errMsg, model)
		excludeIDs[creds.ConnectionID] = true
	}
}

// getExecutor returns the appropriate executor for a provider.
func (s *ChatService) getExecutor(provider string) port.ProviderExecutor {
	switch provider {
	case "kiro":
		return s.kiroExecutor
	case "openai", "openai-compatible":
		return s.openaiExecutor
	default:
		return nil
	}
}

