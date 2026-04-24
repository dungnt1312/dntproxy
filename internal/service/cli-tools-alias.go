package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dungnt/dntproxy/internal/domain"
)

var aliasSafeChars = regexp.MustCompile(`[^a-z0-9-]+`)

// resolveModels resolves a map of role→model, creating aliases for models with slashes.
// Returns resolved map (role→aliasOrModel) and aliases map (alias→originalModel).
func (s *CLIToolsService) resolveModels(models map[string]string) (map[string]string, map[string]string, error) {
	resolved := make(map[string]string, len(models))
	aliases := make(map[string]string)

	cfg, err := s.store.Load()
	if err != nil {
		return nil, nil, err
	}
	if cfg.ModelAliases == nil {
		cfg.ModelAliases = make(domain.AliasMap)
	}

	for role, model := range models {
		if !strings.Contains(model, "/") {
			resolved[role] = model
			continue
		}
		alias, err := findAvailableAlias(cfg.ModelAliases, model)
		if err != nil {
			return nil, nil, err
		}
		resolved[role] = alias
		aliases[alias] = model
	}
	return resolved, aliases, nil
}

// ensureAliases persists all alias→model mappings.
func (s *CLIToolsService) ensureAliases(aliases map[string]string) error {
	if len(aliases) == 0 {
		return nil
	}
	return s.store.Update(func(cfg *domain.AppConfig) {
		if cfg.ModelAliases == nil {
			cfg.ModelAliases = make(domain.AliasMap)
		}
		for alias, model := range aliases {
			cfg.ModelAliases[alias] = model
		}
	})
}

func findAvailableAlias(existing domain.AliasMap, model string) (string, error) {
	baseAlias := stableAliasName(model)
	if existing[baseAlias] == model {
		return baseAlias, nil
	}
	if _, taken := existing[baseAlias]; !taken {
		return baseAlias, nil
	}
	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s-%d", baseAlias, i)
		if existing[candidate] == model {
			return candidate, nil
		}
		if _, taken := existing[candidate]; !taken {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find available alias for model %s", model)
}

func stableAliasName(model string) string {
	parts := strings.Split(model, "/")
	last := parts[len(parts)-1]
	last = strings.ToLower(strings.TrimSpace(last))
	last = strings.ReplaceAll(last, "_", "-")
	last = strings.ReplaceAll(last, ".", "-")
	last = aliasSafeChars.ReplaceAllString(last, "-")
	last = strings.Trim(last, "-")
	if last == "" {
		last = "model"
	}
	return "dntproxy-" + last
}
