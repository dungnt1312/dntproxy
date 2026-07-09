package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/logger"
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
	modelAccess     *ModelAccessService
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
		modelAccess:     NewModelAccessService(store),
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
		modelAccess:     NewModelAccessService(store),
	}
}

// HandleChat processes a chat completion request.
// Unified flow: resolve → policy check → combo/single → weighted random connection → execute.
func (s *ChatService) HandleChat(body []byte, modelStr string, requestID string, policy *port.APIKeyPolicy, metadata ...port.RequestMetadata) *port.ChatResult {
	plan, err := s.modelAccess.ResolveRoute(modelStr, policy)
	if err != nil {
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: fmt.Sprintf("resolve model: %s", err)}
	}

	if len(plan.Attempts) == 0 {
		if plan.DeniedByPolicy {
			return &port.ChatResult{StatusCode: http.StatusForbidden, Error: fmt.Sprintf("Model %q not allowed for this API key", modelStr)}
		}
		return &port.ChatResult{StatusCode: http.StatusBadRequest, Error: fmt.Sprintf("No connections available for model %q", modelStr)}
	}

	// Determine combo strategy
	strategy := "fallback"
	if plan.IsCombo {
		settings, _ := s.store.GetSettings()
		if settings != nil && settings.ComboStrategy != "" {
			strategy = settings.ComboStrategy
		}
		if settings != nil && settings.ComboStrategies != nil {
			if cs, ok := settings.ComboStrategies[plan.ComboName]; ok && cs != "" {
				strategy = cs
			}
		}
	}

	comboName := plan.ComboName
	if comboName == "" {
		comboName = modelStr // use model string as key for non-combo (for logging)
	}

	// Build per-attempt connection allowlists and model list for combo handler.
	attemptConnIDs := make(map[string][]string, len(plan.Attempts))
	models := make([]string, 0, len(plan.Attempts))
	for _, a := range plan.Attempts {
		models = append(models, a.QualifiedModel)
		attemptConnIDs[a.QualifiedModel] = a.AllowedConnectionIDs
	}

	if logger.IsDevMode() {
		logger.Get().Add("CHAT", "INFO", fmt.Sprintf("%q → %d models (strategy: %s, combo: %v)", modelStr, len(models), strategy, plan.IsCombo))
	}

	// Unified: every request goes through comboHandler.
	// For single models, this is just a loop of 1.
	result, err := s.comboHandler.HandleCombo(models, comboName, strategy, func(qualifiedModel string) (*ComboResult, error) {
		return s.executeOnProvider(body, qualifiedModel, requestID, attemptConnIDs[qualifiedModel], metadata...)
	})

	if err != nil && logger.IsDevMode() {
		logger.Get().Add("CHAT", "ERROR", fmt.Sprintf("All models exhausted: %s", err))
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

// executeOnProvider handles a single "provider/model@connectionId" string:
// parse → get executor → select connection (pinned or weighted random) → execute → retry on fail.
func (s *ChatService) executeOnProvider(body []byte, qualifiedModel string, requestID string, allowedConnectionIDs []string, metadata ...port.RequestMetadata) (*ComboResult, error) {
	reqlog := logger.NewRequestLog(requestID)
	reqlog.Begin("POST", "/v1/chat/completions", qualifiedModel, len(body))
	if len(metadata) > 0 {
		reqlog.AttachCompression(metadata[0].Compression)
	}

	// Parse "provider/model@connectionId"
	parsed, err := ParseModelString(qualifiedModel)
	if err != nil {
		msg := fmt.Sprintf("invalid model format: %s", err)
		reqlog.End(http.StatusBadRequest, msg)
		return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: msg}, nil
	}
	provider := parsed.Provider
	model := parsed.Model

	// Validate pinned connection against allowed list
	if parsed.ConnectionID != "" && len(allowedConnectionIDs) > 0 {
		allowed := false
		for _, id := range allowedConnectionIDs {
			if id == parsed.ConnectionID {
				allowed = true
				break
			}
		}
		if !allowed {
			msg := fmt.Sprintf("Connection %q not allowed for this API key", parsed.ConnectionID)
			reqlog.End(http.StatusForbidden, msg)
			return &ComboResult{OK: false, StatusCode: http.StatusForbidden, Error: msg, Terminal: true}, nil
		}
	}

	reqlog.Route(provider, model)

	executor := s.providers.GetExecutor(provider)
	if executor == nil {
		msg := fmt.Sprintf("Provider '%s' not supported", provider)
		reqlog.End(http.StatusBadRequest, msg)
		return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: msg}, nil
	}

	// Inject model into body (strip provider prefix if present)
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawMap); err != nil {
		reqlog.End(http.StatusBadRequest, "Invalid JSON body")
		return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: "Invalid JSON body"}, nil
	}
	cleanModel := model
	if idx := strings.LastIndex(model, "/"); idx >= 0 {
		cleanModel = model[idx+1:]
	}
	modelJSON, _ := json.Marshal(cleanModel)
	rawMap["model"] = modelJSON
	updatedBody, err := json.Marshal(rawMap)
	if err != nil {
		reqlog.End(http.StatusInternalServerError, "Failed to serialize request body")
		return &ComboResult{OK: false, StatusCode: http.StatusInternalServerError, Error: "Failed to serialize request body"}, nil
	}

	// Try connections with weighted random + fallback on failure
	// If model has @connectionId, SelectCredentialsForModel will pin to that connection
	excludeIDs := make(map[string]bool)
	attempted := 0

	for {
		// Use new method that handles @connectionId pinning
		creds, err := s.accountSelector.SelectCredentialsForModel(qualifiedModel, excludeIDs, allowedConnectionIDs)
		if err != nil {
			// Map selection errors to appropriate combo results
			if mapped := mapSelectionErrorToComboResult(err); mapped != nil {
				reqlog.End(mapped.StatusCode, mapped.Error)
				return mapped, nil
			}
			msg := "All accounts unavailable"
			reqlog.End(http.StatusServiceUnavailable, msg)
			return &ComboResult{OK: false, StatusCode: http.StatusServiceUnavailable, Error: msg}, nil
		}

		// Count each successfully selected distinct connection once.
		// OAuth refresh re-Execute on the same connection must not increment again.
		attempted++

		reqlog.SelectAccount(creds.ConnectionID, creds.ConnectionName)

		start := time.Now()
		stream, status, execErr := executor.Execute(model, updatedBody, creds, reqlog)
		duration := time.Since(start)

		if execErr == nil && status == http.StatusOK {
			reqlog.Upstream("", "", status, duration, nil)
			s.accountSelector.ClearError(creds.ConnectionID, model)
			s.accountSelector.AdvanceConnectionRotation(provider, model, allowedConnectionIDs)

			// Stream wrapper will finalize reqlog.End() when consumer closes stream
			wrappedStream := reqlog.WrapStream(stream)
			return &ComboResult{OK: true, Stream: wrappedStream, StatusCode: http.StatusOK}, nil
		}

		errMsg := ""
		if execErr != nil {
			errMsg = execErr.Error()
		}

		reqlog.Upstream("", "", status, duration, execErr)

		if !shouldFallbackToNextAccount(status, errMsg) {
			statusCode, message := normalizeExecutorFailure(status, errMsg)
			reqlog.End(statusCode, message)
			return &ComboResult{OK: false, StatusCode: statusCode, Error: message}, nil
		}

		// Mark unavailable and retry...
		if err := s.accountSelector.MarkUnavailable(creds.ConnectionID, status, errMsg, model); err != nil {
			if logger.IsDevMode() {
				logger.Get().AddError("PROXY", "Failed marking account unavailable: %s", err)
			}
		}
		excludeIDs[creds.ConnectionID] = true

		// If this was a pinned connection, don't retry (no other connections to try)
		if parsed.ConnectionID != "" {
			msg := fmt.Sprintf("Pinned connection failed: %s", errMsg)
			reqlog.End(http.StatusServiceUnavailable, msg)
			return &ComboResult{OK: false, StatusCode: http.StatusServiceUnavailable, Error: msg}, nil
		}

		max := 0
		if settings, err := s.store.GetSettings(); err == nil && settings != nil {
			max = settings.MaxRetryCredentials
		}
		if shouldStopCredentialRetry(attempted, max) {
			msg := fmt.Sprintf("credential retry budget exhausted (%d)", max)
			reqlog.End(http.StatusServiceUnavailable, msg)
			return &ComboResult{OK: false, StatusCode: http.StatusServiceUnavailable, Error: msg, AllowFallback: true}, nil
		}
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
	case SelectionErrNoAllowedConnection:
		return &ComboResult{OK: false, StatusCode: http.StatusForbidden, Error: selErr.Error(), AllowFallback: true}
	case SelectionErrUnsupportedModel:
		return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: selErr.Error()}
	case SelectionErrRateLimited, SelectionErrModelLocked:
		return &ComboResult{OK: false, StatusCode: http.StatusTooManyRequests, Error: selErr.Error()}
	default:
		return &ComboResult{OK: false, StatusCode: http.StatusServiceUnavailable, Error: selErr.Error()}
	}
}

// ClearComboRotation clears the rotation state for a deleted combo.
func (s *ChatService) ClearComboRotation(comboName string) {
	s.comboHandler.ClearRotation(comboName)
}
