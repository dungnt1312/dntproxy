package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/dungnt/dntproxy/internal/adapter/kiro"
	openaiAdapter "github.com/dungnt/dntproxy/internal/adapter/openai"
	"github.com/dungnt/dntproxy/internal/port"
)

// ChatService orchestrates: resolve model -> get credentials -> execute -> stream.
type ChatService struct {
	resolver        *ModelResolver
	accountSelector *AccountSelector
	comboHandler    *ComboHandler
	kiroExecutor    port.ProviderExecutor
	openaiExecutor  port.ProviderExecutor
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
func (s *ChatService) HandleChat(body []byte, modelStr string, requestID string) *ChatResult {
	comboModels, err := s.resolver.GetComboModels(modelStr)
	if err != nil {
		log.Printf("[CHAT] Error checking combo: %s", err)
	}

	if comboModels != nil {
		return s.handleComboChat(body, modelStr, comboModels, requestID)
	}

	return s.handleSingleModel(body, modelStr, requestID)
}

func (s *ChatService) handleComboChat(body []byte, comboName string, models []string, requestID string) *ChatResult {
	settings, _ := s.store.GetSettings()
	strategy := "fallback"
	if settings != nil && settings.ComboStrategy != "" {
		strategy = settings.ComboStrategy
	}
	if settings != nil && settings.ComboStrategies != nil {
		if cs, ok := settings.ComboStrategies[comboName]; ok && cs != "" {
			strategy = cs
		}
	}

	log.Printf("[CHAT] Combo %q with %d models (strategy: %s)", comboName, len(models), strategy)

	result, err := s.comboHandler.HandleCombo(models, comboName, strategy, func(modelStr string) (*ComboResult, error) {
		chatResult := s.handleSingleModel(body, modelStr, requestID)
		if chatResult.Stream != nil && chatResult.StatusCode == http.StatusOK {
			return &ComboResult{
				OK:         true,
				Stream:     chatResult.Stream,
				StatusCode: http.StatusOK,
			}, nil
		}

		return &ComboResult{
			OK:         false,
			StatusCode: chatResult.StatusCode,
			Error:      chatResult.Error,
		}, nil
	})
	if err != nil {
		log.Printf("[COMBO] Handle combo failed: %s", err)
	}

	if result != nil && result.OK && result.Stream != nil {
		return &ChatResult{Stream: result.Stream, StatusCode: http.StatusOK}
	}

	if result != nil {
		status := result.StatusCode
		if status == 0 {
			status = http.StatusServiceUnavailable
		}
		msg := result.Error
		if msg == "" {
			msg = "All combo models unavailable"
		}
		return &ChatResult{StatusCode: status, Error: msg}
	}

	if err != nil {
		return &ChatResult{StatusCode: http.StatusServiceUnavailable, Error: err.Error()}
	}

	return &ChatResult{StatusCode: http.StatusServiceUnavailable, Error: "All combo models unavailable"}
}

func (s *ChatService) handleSingleModel(body []byte, modelStr string, requestID string) *ChatResult {
	modelInfo, err := s.resolver.Resolve(modelStr)
	if err != nil {
		return &ChatResult{StatusCode: http.StatusBadRequest, Error: fmt.Sprintf("resolve model: %s", err)}
	}

	if modelInfo.Provider == "" {
		comboModels, _ := s.resolver.GetComboModels(modelStr)
		if comboModels != nil {
			return s.handleComboChat(body, modelStr, comboModels, requestID)
		}
		return &ChatResult{StatusCode: http.StatusBadRequest, Error: "Invalid model format"}
	}

	provider := modelInfo.Provider
	model := modelInfo.Model

	log.Printf("[ROUTING] %s -> %s/%s", modelStr, provider, model)

	executor := s.getExecutor(provider)
	if executor == nil {
		return &ChatResult{StatusCode: http.StatusBadRequest, Error: fmt.Sprintf("Provider '%s' not supported", provider)}
	}

	var bodyMap map[string]interface{}
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		return &ChatResult{StatusCode: http.StatusBadRequest, Error: "Invalid JSON body"}
	}
	bodyMap["model"] = model
	updatedBody, _ := json.Marshal(bodyMap)

	excludeIDs := make(map[string]bool)

	for {
		creds, err := s.accountSelector.SelectCredentials(provider, excludeIDs, model)
		if err != nil {
			if mapped := mapSelectionErrorToChatResult(err); mapped != nil {
				return mapped
			}
			return &ChatResult{StatusCode: http.StatusServiceUnavailable, Error: "All accounts unavailable"}
		}

		log.Printf("[AUTH] Using %s account: %s", provider, creds.ConnectionName)

		stream, status, execErr := executor.Execute(model, updatedBody, creds, requestID)
		if execErr == nil && status == http.StatusOK {
			s.accountSelector.ClearError(creds.ConnectionID, model)
			return &ChatResult{Stream: stream, StatusCode: http.StatusOK}
		}

		errMsg := ""
		if execErr != nil {
			errMsg = execErr.Error()
		}
		log.Printf("[AUTH] Account %s failed (%d): %s", creds.ConnectionName, status, errMsg)

		if !shouldFallbackToNextAccount(status, errMsg) {
			statusCode, message := normalizeExecutorFailure(status, errMsg)
			return &ChatResult{StatusCode: statusCode, Error: message}
		}

		if err := s.accountSelector.MarkUnavailable(creds.ConnectionID, status, errMsg, model); err != nil {
			log.Printf("[AUTH] Failed marking account unavailable (%s): %s", creds.ConnectionName, err)
		}
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
