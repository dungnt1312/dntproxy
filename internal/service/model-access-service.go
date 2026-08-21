package service

import (
	"fmt"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// ModelAccessService computes the effective model pool for an API key policy.
// Pool is built per-request from current config — no caching, no persistence.
type ModelAccessService struct {
	store    port.CredentialStore
	resolver *ModelResolver
}

// NewModelAccessService creates a new ModelAccessService.
func NewModelAccessService(store port.CredentialStore) *ModelAccessService {
	return &ModelAccessService{
		store:    store,
		resolver: NewModelResolver(store),
	}
}

// BuildPool computes the effective pool from current config and API key policy.
// policy may be nil (unrestricted access).
func (s *ModelAccessService) BuildPool(policy *port.APIKeyPolicy) (*EffectiveModelPool, error) {
	return s.BuildPoolForTenant(policy, "")
}

// BuildPoolForTenant is the tenant-aware variant of BuildPool.
func (s *ModelAccessService) BuildPoolForTenant(policy *port.APIKeyPolicy, tenantID string) (*EffectiveModelPool, error) {
	cfg, err := s.loadConfigForTenant(tenantID)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if cfg == nil {
		return &EffectiveModelPool{}, nil
	}

	effectiveConns := s.buildEffectiveConnections(cfg, policy)
	models := s.buildDirectModels(cfg, effectiveConns, policy)
	aliases := s.buildEffectiveAliases(cfg, models, policy)
	combos := s.buildEffectiveCombos(cfg, models, policy)

	return &EffectiveModelPool{
		Models:  models,
		Aliases: aliases,
		Combos:  combos,
	}, nil
}

// ResolveRoute builds a RoutePlan for the given model string and policy.
// Unlike BuildPool (which is for listing), ResolveRoute checks connection
// availability directly — connections with empty SupportedModels accept any model.
//
// DeniedByPolicy is set when models exist but are blocked by API key restrictions.
func (s *ModelAccessService) ResolveRoute(modelStr string, policy *port.APIKeyPolicy) (*RoutePlan, error) {
	return s.ResolveRouteForTenant(modelStr, policy, "")
}

// ResolveRouteForTenant is the tenant-aware variant of ResolveRoute.
// When tenantID is empty ("legacy mode"), no filtering is applied.
func (s *ModelAccessService) ResolveRouteForTenant(modelStr string, policy *port.APIKeyPolicy, tenantID string) (*RoutePlan, error) {
	routing, err := s.resolver.ResolveRoutingForTenant(modelStr, tenantID)
	if err != nil {
		return nil, err
	}

	cfg, err := s.loadConfigForTenant(tenantID)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Build connections without policy to check if models are routable at all.
	allActiveConns := s.buildEffectiveConnections(cfg, nil)
	effectiveConns := s.buildEffectiveConnections(cfg, policy)

	// Merge connection restrictions: combo-level + API key policy
	policyConnIDs := policyConnectionIDs(policy)
	comboConnIDs := routing.AllowedConnectionIDs
	mergedConnIDs := IntersectConnectionIDs(comboConnIDs, policyConnIDs)

	plan := &RoutePlan{
		RequestedModel: modelStr,
		IsCombo:        routing.IsCombo,
		ComboName:      routing.ComboName,
	}

	for _, qualifiedModel := range routing.Models {
		attempt, ok := s.buildRouteAttempt(qualifiedModel, effectiveConns, mergedConnIDs, policy, modelStr)
		if ok {
			plan.Attempts = append(plan.Attempts, attempt)
		}
	}

	// If no attempts but models could route without policy, it's a policy denial.
	if len(plan.Attempts) == 0 && policy != nil {
		for _, qualifiedModel := range routing.Models {
			if _, ok := s.buildRouteAttempt(qualifiedModel, allActiveConns, comboConnIDs, nil, ""); ok {
				plan.DeniedByPolicy = true
				break
			}
		}
	}

	return plan, nil
}

// loadConfigForTenant returns the config filtered by tenant when the store
// supports tenant extensions; otherwise returns the global config.
func (s *ModelAccessService) loadConfigForTenant(tenantID string) (*domain.AppConfig, error) {
	if tenantID == "" {
		return s.store.Load()
	}
	if ext := port.AsTenantExt(s.store); ext != nil {
		return ext.LoadForTenant(tenantID)
	}
	return s.store.Load()
}

// buildEffectiveConnections returns active connections allowed by policy.
func (s *ModelAccessService) buildEffectiveConnections(cfg *domain.AppConfig, policy *port.APIKeyPolicy) []domain.ProviderConnection {
	var result []domain.ProviderConnection
	for _, conn := range cfg.ProviderConnections {
		if !conn.IsActive {
			continue
		}
		if !ConnectionAllowed(conn.ID, policy) {
			continue
		}
		result = append(result, conn)
	}
	return result
}

// buildDirectModels builds the model pool from effective connections.
func (s *ModelAccessService) buildDirectModels(cfg *domain.AppConfig, conns []domain.ProviderConnection, policy *port.APIKeyPolicy) []ModelRef {
	seen := make(map[string]*ModelRef)
	var order []string

	for _, conn := range conns {
		modelProvider := publicModelProvider(conn)
		if len(conn.SupportedModels) == 0 {
			// Connection supports all registry models for its provider
			if cfg.ModelRegistry != nil {
				for key, m := range cfg.ModelRegistry.Models {
					if m.Provider != conn.Provider || !m.IsActive {
						continue
					}
					publicKey := key
					publicModel := strings.TrimPrefix(key, m.Provider+"/")
					if modelProvider != conn.Provider {
						publicKey = modelProvider + "/" + publicModel
					}
					if !ModelAllowedByPolicy(publicKey, policy) {
						continue
					}
					if ref, exists := seen[publicKey]; exists {
						ref.ConnectionIDs = appendUnique(ref.ConnectionIDs, conn.ID)
					} else {
						seen[publicKey] = &ModelRef{
							QualifiedID:     publicKey,
							Provider:        m.Provider,
							DisplayProvider: modelProvider,
							Model:           publicModel,
							ConnectionIDs:   []string{conn.ID},
						}
						order = append(order, publicKey)
					}
				}
			}
		} else {
			for _, modelID := range conn.SupportedModels {
				key := modelProvider + "/" + modelID
				if !ModelAllowedByPolicy(key, policy) {
					continue
				}
				if ref, exists := seen[key]; exists {
					ref.ConnectionIDs = appendUnique(ref.ConnectionIDs, conn.ID)
				} else {
					seen[key] = &ModelRef{
						QualifiedID:     key,
						Provider:        conn.Provider,
						DisplayProvider: modelProvider,
						Model:           modelID,
						ConnectionIDs:   []string{conn.ID},
					}
					order = append(order, key)
				}
			}
		}
	}

	result := make([]ModelRef, 0, len(order))
	for _, key := range order {
		result = append(result, *seen[key])
	}
	return result
}

// buildEffectiveAliases returns aliases whose targets are available in the model pool.
func (s *ModelAccessService) buildEffectiveAliases(cfg *domain.AppConfig, models []ModelRef, policy *port.APIKeyPolicy) []AliasRef {
	if len(cfg.ModelAliases) == 0 {
		return nil
	}

	modelSet := make(map[string]bool, len(models))
	for _, m := range models {
		modelSet[m.QualifiedID] = true
	}

	var result []AliasRef
	for alias, target := range cfg.ModelAliases {
		// Check if alias itself is explicitly allowed by policy
		if policy != nil && len(policy.AllowedModels) > 0 {
			aliasAllowed := false
			for _, allowed := range policy.AllowedModels {
				if allowed == alias || ModelPolicyMatch(target, allowed) {
					aliasAllowed = true
					break
				}
			}
			if !aliasAllowed {
				continue
			}
		}

		// Check if target is routable (exists in model pool)
		if _, ok := modelSet[StripConnectionPin(target)]; ok {
			result = append(result, AliasRef{Name: alias, Target: target})
			continue
		}

		normalizedTarget := NormalizeModelPolicyString(target)
		stripped := StripConnectionPin(normalizedTarget)
		if modelSet[stripped] {
			result = append(result, AliasRef{Name: alias, Target: normalizedTarget})
			continue
		}

		if resolvedTarget, ok := resolveOpenAICompatibleKey(stripped, models); ok {
			if modelSet[resolvedTarget] {
				result = append(result, AliasRef{Name: alias, Target: resolvedTarget})
			}
		}
	}
	return result
}

// buildEffectiveCombos returns combos that have at least one effective member.
func (s *ModelAccessService) buildEffectiveCombos(cfg *domain.AppConfig, models []ModelRef, policy *port.APIKeyPolicy) []ComboRef {
	if len(cfg.Combos) == 0 {
		return nil
	}

	modelSet := make(map[string]bool, len(models))
	for _, m := range models {
		modelSet[m.QualifiedID] = true
	}

	var result []ComboRef
	for _, combo := range cfg.Combos {
		// Intersect combo connection restrictions with policy
		comboConnIDs := combo.ConnectionIDs
		policyConnIDs := policyConnectionIDs(policy)
		effectiveConnIDs := IntersectConnectionIDs(comboConnIDs, policyConnIDs)

		// If both have restrictions but intersection is empty, skip combo
		if len(comboConnIDs) > 0 && len(policyConnIDs) > 0 && len(effectiveConnIDs) == 0 {
			continue
		}

		var effectiveModels []string
		for _, m := range combo.Models {
			stripped := StripConnectionPin(m)
			if modelSet[stripped] && ModelAllowedByPolicy(m, policy) {
				effectiveModels = append(effectiveModels, m)
				continue
			}
			normalized := NormalizeModelPolicyString(m)
			stripped = StripConnectionPin(normalized)
			if modelSet[stripped] && ModelAllowedByPolicy(normalized, policy) {
				effectiveModels = append(effectiveModels, normalized)
				continue
			}
			if resolved, ok := resolveOpenAICompatibleKey(stripped, models); ok {
				if modelSet[resolved] && ModelAllowedByPolicy(normalized, policy) {
					effectiveModels = append(effectiveModels, resolved)
				}
			}
		}

		if len(effectiveModels) > 0 {
			result = append(result, ComboRef{
				Name:            combo.Name,
				EffectiveModels: effectiveModels,
				ConnectionIDs:   effectiveConnIDs,
			})
		}
	}
	return result
}

// buildRouteAttempt creates a RouteAttempt by checking effective connections directly.
// A connection supports a model if SupportedModels is empty (all models) or matches.
// Returns false if no effective connection can serve this model or policy denies it.
func (s *ModelAccessService) buildRouteAttempt(qualifiedModel string, effectiveConns []domain.ProviderConnection, mergedConnIDs []string, policy *port.APIKeyPolicy, requestedModel string) (RouteAttempt, bool) {
	parsed, err := ParseModelString(qualifiedModel)
	if err != nil {
		return RouteAttempt{}, false
	}

	// Check model policy — also allow by original requested name (alias/combo name).
	if !ModelAllowedByPolicy(qualifiedModel, policy) {
		if requestedModel == "" || !modelNameAllowedByPolicy(requestedModel, policy) {
			return RouteAttempt{}, false
		}
	}

	// Find connections that can serve this model
	var connIDs []string
	for _, conn := range effectiveConns {
		if conn.Provider != parsed.Provider {
			continue
		}
		if parsed.Provider == "openai-compatible" && parsed.ConnectionID != "" && conn.ID != parsed.ConnectionID {
			continue
		}
		if !conn.SupportsModel(parsed.Model) {
			continue
		}
		connIDs = append(connIDs, conn.ID)
	}

	if len(connIDs) == 0 {
		return RouteAttempt{}, false
	}

	// Merge with route-level connection restrictions
	attemptConnIDs := IntersectConnectionIDs(connIDs, mergedConnIDs)
	if len(mergedConnIDs) > 0 && len(attemptConnIDs) == 0 {
		return RouteAttempt{}, false
	}

	// If pinned, verify the pinned connection is in the allowed set
	if parsed.ConnectionID != "" && len(attemptConnIDs) > 0 {
		found := false
		for _, id := range attemptConnIDs {
			if id == parsed.ConnectionID {
				found = true
				break
			}
		}
		if !found {
			return RouteAttempt{}, false
		}
	}

	return RouteAttempt{
		QualifiedModel:       qualifiedModel,
		Provider:             parsed.Provider,
		Model:                parsed.Model,
		PinnedConnectionID:   parsed.ConnectionID,
		AllowedConnectionIDs: attemptConnIDs,
	}, true
}

func publicModelProvider(conn domain.ProviderConnection) string {
	if conn.Provider == "openai-compatible" {
		prefix := domain.NormalizeRoutePrefix(conn.RoutePrefix)
		if prefix == "" {
			prefix = domain.NormalizeRoutePrefix(conn.Name)
		}
		if prefix != "" {
			return prefix
		}
	}
	return domain.PublicProviderPrefix(conn.Provider)
}

func resolveOpenAICompatibleKey(qualifiedModel string, models []ModelRef) (string, bool) {
	parsed, err := ParseModelString(qualifiedModel)
	if err != nil || parsed.Provider != "openai-compatible" {
		return "", false
	}
	for _, model := range models {
		if model.Provider != "openai-compatible" || model.Model != parsed.Model {
			continue
		}
		return model.QualifiedID, true
	}
	return "", false
}

// policyConnectionIDs extracts the connection allowlist from a policy (nil-safe).
func policyConnectionIDs(policy *port.APIKeyPolicy) []string {
	if policy == nil {
		return nil
	}
	return policy.AllowedConnectionIDs
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
