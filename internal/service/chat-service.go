package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
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
	sessionAffinity *SessionAffinityStore
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
		sessionAffinity: NewSessionAffinityStore(),
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
		sessionAffinity: NewSessionAffinityStore(),
	}
}

// HandleChat processes a chat completion request.
// Unified flow: resolve → policy check → combo/single → weighted random connection → execute.
//
// The optional *port.RequestContext (trailing variadic) carries the tenant ID
// used for multi-tenancy isolation. When absent or nil, legacy single-tenant
// behavior is used (no filtering).
func (s *ChatService) HandleChat(body []byte, modelStr string, requestID string, policy *port.APIKeyPolicy, metadata ...port.RequestMetadata) *port.ChatResult {
	tenantID := extractTenantID(metadata)

	plan, err := s.modelAccess.ResolveRouteForTenant(modelStr, policy, tenantID)
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
	result, err := s.comboHandler.HandleCombo(extractContext(metadata), models, comboName, strategy, func(qualifiedModel string) (*ComboResult, error) {
		args := make([]interface{}, 0, 1+len(metadata))
		args = append(args, tenantID)
		for _, m := range metadata {
			args = append(args, m)
		}
		return s.executeOnProvider(body, qualifiedModel, requestID, attemptConnIDs[qualifiedModel], args...)
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
		return &port.ChatResult{StatusCode: status, Error: msg, RetryAfter: result.RetryAfter}
	}

	if err != nil {
		return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: err.Error()}
	}

	return &port.ChatResult{StatusCode: http.StatusServiceUnavailable, Error: "All models unavailable"}
}

// executeOnProvider handles a single "provider/model@connectionId" string:
// parse → get executor → select connection (pinned or weighted random) → execute → retry on fail.
func (s *ChatService) executeOnProvider(body []byte, qualifiedModel string, requestID string, allowedConnectionIDs []string, args ...interface{}) (*ComboResult, error) {
	tenantID := ""
	metadata := make([]port.RequestMetadata, 0, len(args))
	for _, arg := range args {
		switch value := arg.(type) {
		case string:
			tenantID = value
		case port.RequestMetadata:
			metadata = append(metadata, value)
		}
	}
	reqlog := logger.NewRequestLogForTenant(requestID, tenantID)
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

	// Inject the resolved provider-local model into the body.
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawMap); err != nil {
		reqlog.End(http.StatusBadRequest, "Invalid JSON body")
		return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: "Invalid JSON body"}, nil
	}
	modelJSON, _ := json.Marshal(model)
	rawMap["model"] = modelJSON
	updatedBody, err := json.Marshal(rawMap)
	if err != nil {
		reqlog.End(http.StatusInternalServerError, "Failed to serialize request body")
		return &ComboResult{OK: false, StatusCode: http.StatusInternalServerError, Error: "Failed to serialize request body"}, nil
	}

	// Try connections with configured selection and fallback on failure.
	excludeIDs := make(map[string]bool)
	var lastExecStatus int
	var lastExecErr string
	var meta port.RequestMetadata
	if len(metadata) > 0 {
		meta = metadata[0]
	}
	hardPin := parsed.ConnectionID != ""
	affinityKey := ""
	stickyTried := false
	if !hardPin {
		if settings, err := s.store.GetSettings(); err == nil && settings != nil && settings.SessionAffinityEnabled {
			affinityKey = AffinityKey(meta.APIKeyID, provider, model, meta.SessionKey)
		}
	}
	attempted := 0
	ctx := extractContext(metadata)

	for {
		if err := ctx.Err(); err != nil {
			msg := "client disconnected"
			reqlog.End(499, msg)
			return &ComboResult{OK: false, StatusCode: 499, Error: msg}, err
		}

		selectModel := qualifiedModel
		if affinityKey != "" && !stickyTried && s.sessionAffinity != nil {
			if stickyID, ok := s.sessionAffinity.Get(affinityKey); ok {
				stickyTried = true
				if stickyAllowed(stickyID, allowedConnectionIDs) {
					selectModel = provider + "/" + model + "@" + stickyID
				}
			}
		}

		// SelectCredentialsForModel handles both hard and temporary affinity pins.
		creds, err := s.accountSelector.SelectCredentialsForModel(selectModel, excludeIDs, allowedConnectionIDs, tenantID)
		if err != nil {
			if affinityKey != "" && stickyTried && selectModel != qualifiedModel {
				if sticky, parseErr := ParseModelString(selectModel); parseErr == nil && sticky.ConnectionID != "" {
					excludeIDs[sticky.ConnectionID] = true
				}
				if s.sessionAffinity != nil {
					s.sessionAffinity.Delete(affinityKey)
				}
				continue
			}
			if lastExecErr != "" {
				statusCode, message := normalizeExecutorFailure(lastExecStatus, lastExecErr)
				reqlog.End(statusCode, message)
				return &ComboResult{
					OK:            false,
					StatusCode:    statusCode,
					Error:         message,
					RetryAfter:    retryAfterFromError(lastExecErr),
					AllowFallback: shouldFallbackToNextAccount(lastExecStatus, lastExecErr),
				}, nil
			}
			// Map selection errors to appropriate combo results
			if mapped := mapSelectionErrorToComboResult(err); mapped != nil {
				reqlog.End(mapped.StatusCode, mapped.Error)
				return mapped, nil
			}
			msg := "All accounts unavailable"
			reqlog.End(http.StatusServiceUnavailable, msg)
			return &ComboResult{OK: false, StatusCode: http.StatusServiceUnavailable, Error: msg}, nil
		}

		// Count each successfully selected distinct connection once. OAuth refresh
		// retries execute on the same connection and do not consume more budget.
		attempted++
		reqlog.SelectAccount(creds.ConnectionID, creds.ConnectionName)

		if creds.BaseURL != "" {
			if err := shared.ValidateOutboundURL(creds.BaseURL, shared.AllowPrivateOutbound(creds.TenantID)); err != nil {
				msg := "unsafe connection base URL: " + err.Error()
				reqlog.End(http.StatusBadRequest, msg)
				return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: msg, Terminal: true}, nil
			}
		}

		start := time.Now()
		stream, status, execErr := executor.Execute(ctx, model, updatedBody, creds, reqlog)
		duration := time.Since(start)

		if status == http.StatusUnauthorized && strings.TrimSpace(creds.RefreshToken) != "" {
			if refreshedCreds, refErr := s.accountSelector.RefreshCredentialsForOAuth(creds.ConnectionID); refErr == nil && refreshedCreds != nil {
				creds = refreshedCreds
				start = time.Now()
				stream, status, execErr = executor.Execute(ctx, model, updatedBody, creds, reqlog)
				duration = time.Since(start)
			}
		}

		if execErr == nil && status == http.StatusOK {
			reqlog.Upstream("", "", status, duration, nil)
			s.accountSelector.ClearError(creds.ConnectionID, model)
			s.accountSelector.AdvanceConnectionRotation(provider, model, allowedConnectionIDs)
			if affinityKey != "" && !hardPin && s.sessionAffinity != nil {
				s.sessionAffinity.Put(affinityKey, creds.ConnectionID, sessionAffinityTTL(s.store))
			}

			// Stream wrapper will finalize reqlog.End() when consumer closes stream
			wrappedStream := reqlog.WrapStream(stream)
			return &ComboResult{OK: true, Stream: wrappedStream, StatusCode: http.StatusOK}, nil
		}

		errMsg := ""
		if execErr != nil {
			errMsg = execErr.Error()
		}

		reqlog.Upstream("", "", status, duration, execErr)

		if shared.IsCanceledOrClosedStream(execErr) {
			reqlog.End(499, errMsg)
			return &ComboResult{OK: false, StatusCode: 499, Error: "client disconnected"}, execErr
		}

		if !shouldFallbackToNextAccount(status, errMsg) {
			statusCode, message := normalizeExecutorFailure(status, errMsg)
			reqlog.End(statusCode, message)
			return &ComboResult{OK: false, StatusCode: statusCode, Error: message, RetryAfter: retryAfterFromError(errMsg)}, nil
		}

		// Mark unavailable and retry...
		if err := s.accountSelector.MarkUnavailable(creds.ConnectionID, status, errMsg, model); err != nil {
			if logger.IsDevMode() {
				logger.Get().AddError("PROXY", "Failed marking account unavailable: %s", err)
			}
		}
		excludeIDs[creds.ConnectionID] = true
		lastExecStatus = status
		lastExecErr = errMsg

		if affinityKey != "" && stickyTried && selectModel != qualifiedModel {
			// A soft-pinned connection failed. Forget it and continue normally.
			if s.sessionAffinity != nil {
				s.sessionAffinity.Delete(affinityKey)
			}
		} else if hardPin {
			// A user-specified pin must not spill over to another connection.
			statusCode, message := normalizeExecutorFailure(status, errMsg)
			reqlog.End(statusCode, message)
			return &ComboResult{OK: false, StatusCode: statusCode, Error: message, RetryAfter: retryAfterFromError(errMsg), AllowFallback: shouldFallbackToNextAccount(status, errMsg)}, nil
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

// stickyAllowed reports whether stickyID may be used given the allowlist.
func stickyAllowed(stickyID string, allowedConnectionIDs []string) bool {
	if stickyID == "" {
		return false
	}
	if len(allowedConnectionIDs) == 0 {
		return true
	}
	for _, id := range allowedConnectionIDs {
		if id == stickyID {
			return true
		}
	}
	return false
}

func sessionAffinityTTL(store port.CredentialStore) time.Duration {
	settings, err := store.GetSettings()
	if err != nil || settings == nil {
		return 1800 * time.Second
	}
	copy := *settings
	copy.NormalizeRouting()
	return time.Duration(copy.SessionAffinityTTLSeconds) * time.Second
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
		return &ComboResult{OK: false, StatusCode: http.StatusBadRequest, Error: selErr.Error(), AllowFallback: true}
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

// extractTenantID pulls the tenant ID from variadic metadata (if any).
func extractTenantID(metadata []port.RequestMetadata) string {
	for _, m := range metadata {
		if m.TenantID != "" {
			return m.TenantID
		}
	}
	return ""
}

func extractContext(metadata []port.RequestMetadata) context.Context {
	for _, m := range metadata {
		if m.Context != nil {
			return m.Context
		}
	}
	return context.Background()
}
