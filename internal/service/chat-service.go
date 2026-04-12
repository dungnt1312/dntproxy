package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/dungnt/dntproxy/internal/port"
)

// ChatService orchestrates: resolve model -> get credentials -> execute -> stream.
// Implements port.ChatService.
type ChatService struct {
	resolver        port.ModelResolver
	accountSelector port.AccountSelector
	comboHandler    *ComboHandler
	providers       port.ProviderRegistry
	store           port.CredentialStore
}

// NewChatService creates a new ChatService.
func NewChatService(
	store port.CredentialStore,
	providers port.ProviderRegistry,
) *ChatService {
	return &ChatService{
		resolver:        NewModelResolver(store),
		accountSelector: NewAccountSelector(store),
		comboHandler:    NewComboHandler(),
		providers:       providers,
		store:           store,
	}
}

// NewChatServiceWithDeps creates a ChatService with explicit dependencies (for testing).
func NewChatServiceWithDeps(
	resolver port.ModelResolver,
	accountSelector port.AccountSelector,
	comboHandler *ComboHandler,
	providers port.ProviderRegistry,
	store port.CredentialStore,
) *ChatService {
	return &ChatService{
		resolver:        resolver,
		accountSelector: accountSelector,
		comboHandler:    comboHandler,
		providers:       providers,
		store:           store,
	}
}

// HandleChat processes a chat completion request.
func (s *ChatService) HandleChat(body []byte, modelStr string, requestID string) *port.ChatResult {
	comboModels, err := s.resolver.GetComboModels(modelStr)
	if err != nil {
		log.Printf("[CHAT] Error checking combo: %s", err)
	}

	if comboModels != nil {
		return s.handleComboChat(body, modelStr, comboModels, requestID)
	}

	return s.handleSingleModel(body, modelStr, requestID)
}

func (s *ChatService) handleComboChat(body []byte, comboName string, models []string, requestID string) *port.ChatResult {
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
		return &port.ChatResult{Stream: result.Stream, StatusCode: http.StatusOK}
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
		return &port.ChatResult{StatusCode: status, Error: msg}
	}

	if err != nil {
		return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: err.Error()}
	}

	return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: "All combo models unavailable"}
}

func (s *ChatService) handleSingleModel(body []byte, modelStr string, requestID string) *port.ChatResult {
	modelInfo, err := s.resolver.Resolve(modelStr)
	if err != nil {
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: fmt.Sprintf("resolve model: %s", err)}
	}

	if modelInfo.Provider == "" {
		comboModels, _ := s.resolver.GetComboModels(modelStr)
		if comboModels != nil {
			return s.handleComboChat(body, modelStr, comboModels, requestID)
		}
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: "Invalid model format"}
	}

	provider := modelInfo.Provider
	model := modelInfo.Model

	log.Printf("[ROUTING] %s -> %s/%s", modelStr, provider, model)

	executor := s.providers.GetExecutor(provider)
	if executor == nil {
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: fmt.Sprintf("Provider '%s' not supported", provider)}
	}

	var bodyMap map[string]interface{}
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: "Invalid JSON body"}
	}
	bodyMap["model"] = model
	updatedBody, err := json.Marshal(bodyMap)
	if err != nil {
		return &port.ChatResult{StatusCode: http.StatusInternalServerError, Error: "Failed to serialize request body"}
	}

	excludeIDs := make(map[string]bool)

	for {
		creds, err := s.accountSelector.SelectCredentials(provider, excludeIDs, model)
		if err != nil {
			if mapped := mapSelectionErrorToChatResult(err); mapped != nil {
				return mapped
			}
			return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: "All accounts unavailable"}
		}

		log.Printf("[AUTH] Using %s account: %s", provider, creds.ConnectionName)

		stream, status, execErr := executor.Execute(model, updatedBody, creds, requestID)
		if execErr == nil && status == http.StatusOK {
			s.accountSelector.ClearError(creds.ConnectionID, model)
			return &port.ChatResult{Stream: stream, StatusCode: http.StatusOK}
		}

		errMsg := ""
		if execErr != nil {
			errMsg = execErr.Error()
		}
		log.Printf("[AUTH] Account %s failed (%d): %s", creds.ConnectionName, status, errMsg)

		if !shouldFallbackToNextAccount(status, errMsg) {
			statusCode, message := normalizeExecutorFailure(status, errMsg)
			return &port.ChatResult{StatusCode: statusCode, Error: message}
		}

		if err := s.accountSelector.MarkUnavailable(creds.ConnectionID, status, errMsg, model); err != nil {
			log.Printf("[AUTH] Failed marking account unavailable (%s): %s", creds.ConnectionName, err)
		}
		excludeIDs[creds.ConnectionID] = true
	}
}
