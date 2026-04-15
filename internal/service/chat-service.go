package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/dungnt/dntproxy/internal/port"
)

// ChatService orchestrates: resolve model -> get credentials -> execute -> stream.
// Implements port.ChatService.
type ChatService struct {
	resolver        *ModelResolver
	accountSelector *AccountSelector
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
	resolver *ModelResolver,
	accountSelector *AccountSelector,
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
// Unified flow: resolve → combo/single → weighted random connection → execute.
func (s *ChatService) HandleChat(body []byte, modelStr string, requestID string) *port.ChatResult {
	routing, err := s.resolver.ResolveRouting(modelStr)
	if err != nil {
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: fmt.Sprintf("resolve model: %s", err)}
	}

	// Determine combo strategy
	strategy := "fallback"
	if routing.IsCombo {
		settings, _ := s.store.GetSettings()
		if settings != nil && settings.ComboStrategy != "" {
			strategy = settings.ComboStrategy
		}
		if settings != nil && settings.ComboStrategies != nil {
			if cs, ok := settings.ComboStrategies[routing.ComboName]; ok && cs != "" {
				strategy = cs
			}
		}
	}

	comboName := routing.ComboName
	if comboName == "" {
		comboName = modelStr // use model string as key for non-combo (for logging)
	}

	log.Printf("[CHAT] %q → %d models (strategy: %s, combo: %v)", modelStr, len(routing.Models), strategy, routing.IsCombo)

	// Unified: every request goes through comboHandler.
	// For single models, this is just a loop of 1.
	result, err := s.comboHandler.HandleCombo(routing.Models, comboName, strategy, func(qualifiedModel string) (*ComboResult, error) {
		return s.executeOnProvider(body, qualifiedModel, requestID, routing.AllowedConnectionIDs)
	})
	if err != nil {
		log.Printf("[CHAT] All models exhausted: %s", err)
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
			msg = "All models unavailable"
		}
		return &port.ChatResult{StatusCode: status, Error: msg}
	}

	if err != nil {
		return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: err.Error()}
	}

	return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: "All models unavailable"}
}

// executeOnProvider handles a single "provider/model" string:
// parse → get executor → weighted random connection → execute → retry on fail.
func (s *ChatService) executeOnProvider(body []byte, qualifiedModel string, requestID string, allowedConnectionIDs []string) (*ComboResult, error) {
	// Parse "provider/model"
	idx := strings.Index(qualifiedModel, "/")
	if idx < 0 {
		return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: fmt.Sprintf("invalid model format: %s", qualifiedModel)}, nil
	}
	provider := qualifiedModel[:idx]
	model := qualifiedModel[idx+1:]

	log.Printf("[ROUTING] %s → %s/%s", qualifiedModel, provider, model)

	executor := s.providers.GetExecutor(provider)
	if executor == nil {
		return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: fmt.Sprintf("Provider '%s' not supported", provider)}, nil
	}

	// Inject model into body
	var bodyMap map[string]interface{}
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: "Invalid JSON body"}, nil
	}
	bodyMap["model"] = model
	updatedBody, err := json.Marshal(bodyMap)
	if err != nil {
		return &ComboResult{OK: false, StatusCode: http.StatusInternalServerError, Error: "Failed to serialize request body"}, nil
	}

	// Try connections with weighted random + fallback on failure
	excludeIDs := make(map[string]bool)

	for {
		creds, err := s.accountSelector.SelectCredentials(provider, excludeIDs, model, allowedConnectionIDs)
		if err != nil {
			// Map selection errors to appropriate combo results
			if mapped := mapSelectionErrorToComboResult(err); mapped != nil {
				return mapped, nil
			}
			return &ComboResult{OK: false, StatusCode: http.StatusServiceUnavailable, Error: "All accounts unavailable"}, nil
		}

		log.Printf("[AUTH] Using %s account: %s", provider, creds.ConnectionName)

		stream, status, execErr := executor.Execute(model, updatedBody, creds, requestID)
		if execErr == nil && status == http.StatusOK {
			s.accountSelector.ClearError(creds.ConnectionID, model)
			return &ComboResult{OK: true, Stream: stream, StatusCode: http.StatusOK}, nil
		}

		errMsg := ""
		if execErr != nil {
			errMsg = execErr.Error()
		}
		log.Printf("[AUTH] Account %s failed (%d): %s", creds.ConnectionName, status, errMsg)

		if !shouldFallbackToNextAccount(status, errMsg) {
			statusCode, message := normalizeExecutorFailure(status, errMsg)
			return &ComboResult{OK: false, StatusCode: statusCode, Error: message}, nil
		}

		if err := s.accountSelector.MarkUnavailable(creds.ConnectionID, status, errMsg, model); err != nil {
			log.Printf("[AUTH] Failed marking account unavailable (%s): %s", creds.ConnectionName, err)
		}
		excludeIDs[creds.ConnectionID] = true
	}
}

// mapSelectionErrorToComboResult converts AccountSelectionError to ComboResult
// so the combo handler can decide whether to fall back to the next model.
func mapSelectionErrorToComboResult(err error) *ComboResult {
	var selErr *AccountSelectionError
	if !errors.As(err, &selErr) {
		return nil
	}

	switch selErr.Kind {
	case SelectionErrNoActiveCredentials:
		return &ComboResult{OK: false, StatusCode: http.StatusNotFound, Error: selErr.Error()}
	case SelectionErrUnsupportedModel:
		return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: selErr.Error()}
	case SelectionErrRateLimited, SelectionErrModelLocked:
		return &ComboResult{OK: false, StatusCode: http.StatusTooManyRequests, Error: selErr.Error()}
	default:
		return &ComboResult{OK: false, StatusCode: http.StatusServiceUnavailable, Error: selErr.Error()}
	}
}
