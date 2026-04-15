package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dungnt/dntproxy/internal/logger"
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
	if err != nil && logger.IsDevMode() {
		logger.Get().Add("PROXY", "ERROR", fmt.Sprintf("Error checking combo: %s", err))
	}

	if comboModels != nil {
		return s.handleComboChat(body, modelStr, comboModels, requestID)
	}

	return s.handleSingleModel(body, modelStr, requestID, false, "")
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

	result, err := s.comboHandler.HandleCombo(models, comboName, strategy, func(modelStr string) (*ComboResult, error) {
		chatResult := s.handleSingleModel(body, modelStr, requestID, true, comboName)
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

	if err != nil && logger.IsDevMode() {
		logger.Get().Add("PROXY", "ERROR", fmt.Sprintf("Combo %s failed: %s", comboName, err))
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

func (s *ChatService) handleSingleModel(body []byte, modelStr string, requestID string, isCombo bool, comboName string) *port.ChatResult {
	reqlog := logger.NewRequestLog(requestID)
	reqlog.Begin("POST", "/v1/chat/completions", modelStr, len(body))
	if isCombo {
		reqlog.RouteCombo(comboName)
	}

	modelInfo, err := s.resolver.Resolve(modelStr)
	if err != nil {
		msg := fmt.Sprintf("resolve model: %s", err)
		reqlog.End(http.StatusBadRequest, msg)
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: msg}
	}

	if modelInfo.Provider == "" {
		reqlog.End(http.StatusBadRequest, "Invalid model format")
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: "Invalid model format"}
	}

	provider := modelInfo.Provider
	model := modelInfo.Model

	reqlog.Route(provider, model)

	executor := s.providers.GetExecutor(provider)
	if executor == nil {
		msg := fmt.Sprintf("Provider '%s' not supported", provider)
		reqlog.End(http.StatusBadRequest, msg)
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: msg}
	}

	var bodyMap map[string]interface{}
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		reqlog.End(http.StatusBadRequest, "Invalid JSON body")
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: "Invalid JSON body"}
	}
	bodyMap["model"] = model
	updatedBody, err := json.Marshal(bodyMap)
	if err != nil {
		reqlog.End(http.StatusInternalServerError, "Failed to serialize request body")
		return &port.ChatResult{StatusCode: http.StatusInternalServerError, Error: "Failed to serialize request body"}
	}

	excludeIDs := make(map[string]bool)

	for {
		creds, err := s.accountSelector.SelectCredentials(provider, excludeIDs, model)
		if err != nil {
			msg := "All accounts unavailable"
			if mapped := mapSelectionErrorToChatResult(err); mapped != nil {
				reqlog.End(mapped.StatusCode, mapped.Error)
				return mapped
			}
			reqlog.End(http.StatusServiceUnavailable, msg)
			return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: msg}
		}

		reqlog.SelectAccount(creds.ConnectionID, creds.ConnectionName)

		start := time.Now()
		stream, status, execErr := executor.Execute(model, updatedBody, creds, reqlog)
		duration := time.Since(start)

		if execErr == nil && status == http.StatusOK {
			reqlog.Upstream("", "", status, duration, nil) // Specific URL given by executor via reqlog interface if it wants to
			s.accountSelector.ClearError(creds.ConnectionID, model)
			
			// Stream wrapper will finalize reqlog.End() when consumer closes stream
			wrappedStream := reqlog.WrapStream(stream)
			return &port.ChatResult{Stream: wrappedStream, StatusCode: http.StatusOK}
		}

		errMsg := ""
		if execErr != nil {
			errMsg = execErr.Error()
		}
		
		reqlog.Upstream("", "", status, duration, execErr)

		if !shouldFallbackToNextAccount(status, errMsg) {
			statusCode, message := normalizeExecutorFailure(status, errMsg)
			reqlog.End(statusCode, message)
			return &port.ChatResult{StatusCode: statusCode, Error: message}
		}

		// Mark unavailable and retry...
		if err := s.accountSelector.MarkUnavailable(creds.ConnectionID, status, errMsg, model); err != nil {
			if logger.IsDevMode() {
				logger.Get().AddError("PROXY", "Failed marking account unavailable: %s", err)
			}
		}
		excludeIDs[creds.ConnectionID] = true
	}
}
