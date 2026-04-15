package service

import (
	"fmt"
	"log"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/google/uuid"
)

// ProfileService handles profile CRUD and activation logic.
type ProfileService struct {
	store port.CredentialStore
}

// NewProfileService creates a new ProfileService.
func NewProfileService(store port.CredentialStore) *ProfileService {
	return &ProfileService{store: store}
}

// CreateProfile creates a new profile with the given name and aliases.
func (s *ProfileService) CreateProfile(name, description string, aliases domain.AliasMap) (*domain.Profile, error) {
	cfg, err := s.store.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Check for duplicate name
	for _, p := range cfg.Profiles {
		if p.Name == name {
			return nil, fmt.Errorf("profile '%s' already exists", name)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	profile := domain.Profile{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Aliases:     aliases,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	cfg.Profiles = append(cfg.Profiles, profile)
	if err := s.store.Save(cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	return &profile, nil
}

// CreateFromPreset creates a new profile from a built-in preset.
func (s *ProfileService) CreateFromPreset(presetName string) (*domain.Profile, error) {
	preset, ok := domain.BuiltinPresets[presetName]
	if !ok {
		return nil, fmt.Errorf("unknown preset: %s (available: %v)", presetName, domain.ListPresetNames())
	}

	// Copy aliases to avoid modifying the preset
	aliases := make(domain.AliasMap, len(preset.Aliases))
	for k, v := range preset.Aliases {
		aliases[k] = v
	}

	return s.CreateProfile(preset.Name, preset.Description, aliases)
}

// GetProfile returns a profile by name.
func (s *ProfileService) GetProfile(name string) (*domain.Profile, error) {
	cfg, err := s.store.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	for i := range cfg.Profiles {
		if cfg.Profiles[i].Name == name {
			return &cfg.Profiles[i], nil
		}
	}
	return nil, fmt.Errorf("profile not found: %s", name)
}

// ListProfiles returns all profiles.
func (s *ProfileService) ListProfiles() ([]domain.Profile, string, error) {
	cfg, err := s.store.Load()
	if err != nil {
		return nil, "", fmt.Errorf("load config: %w", err)
	}
	return cfg.Profiles, cfg.Settings.ActiveProfile, nil
}

// DeleteProfile deletes a profile by name.
// If the profile is active, it is deactivated first.
func (s *ProfileService) DeleteProfile(name string) error {
	return s.store.Update(func(cfg *domain.AppConfig) {
		// If active, deactivate first
		if cfg.Settings.ActiveProfile == name {
			s.removeProfileAliasesFromConfig(cfg, name)
			cfg.Settings.ActiveProfile = ""
		}

		for i, p := range cfg.Profiles {
			if p.Name == name {
				cfg.Profiles = append(cfg.Profiles[:i], cfg.Profiles[i+1:]...)
				return
			}
		}
	})
}

// ActivateProfile merges a profile's aliases into the global modelAliases.
// If another profile is active, its aliases are removed first (smart swap).
func (s *ProfileService) ActivateProfile(name string) error {
	return s.store.Update(func(cfg *domain.AppConfig) {
		// Find the target profile
		var target *domain.Profile
		for i := range cfg.Profiles {
			if cfg.Profiles[i].Name == name {
				target = &cfg.Profiles[i]
				break
			}
		}
		if target == nil {
			log.Printf("[PROFILE] Profile not found: %s", name)
			return
		}

		// If another profile is active, remove its aliases first
		if cfg.Settings.ActiveProfile != "" && cfg.Settings.ActiveProfile != name {
			s.removeProfileAliasesFromConfig(cfg, cfg.Settings.ActiveProfile)
			log.Printf("[PROFILE] Deactivated previous profile: %s", cfg.Settings.ActiveProfile)
		}

		// Merge profile aliases into global modelAliases
		if cfg.ModelAliases == nil {
			cfg.ModelAliases = make(domain.AliasMap)
		}
		for alias, model := range target.Aliases {
			cfg.ModelAliases[alias] = model
		}

		// Merge profile combos if any
		if len(target.Combos) > 0 {
			for _, combo := range target.Combos {
				// Skip if combo with same name already exists
				exists := false
				for _, existing := range cfg.Combos {
					if existing.Name == combo.Name {
						exists = true
						break
					}
				}
				if !exists {
					cfg.Combos = append(cfg.Combos, combo)
				}
			}
		}

		cfg.Settings.ActiveProfile = name
		log.Printf("[PROFILE] Activated profile: %s (%d aliases)", name, len(target.Aliases))
	})
}

// DeactivateProfile removes the active profile's aliases from modelAliases.
func (s *ProfileService) DeactivateProfile() error {
	return s.store.Update(func(cfg *domain.AppConfig) {
		if cfg.Settings.ActiveProfile == "" {
			return
		}

		s.removeProfileAliasesFromConfig(cfg, cfg.Settings.ActiveProfile)
		log.Printf("[PROFILE] Deactivated profile: %s", cfg.Settings.ActiveProfile)
		cfg.Settings.ActiveProfile = ""
	})
}

// UpdateProfileAliases updates the aliases of a profile.
// If the profile is active, its aliases in modelAliases are updated too.
func (s *ProfileService) UpdateProfileAliases(name string, addAliases domain.AliasMap, removeAliases []string) error {
	return s.store.Update(func(cfg *domain.AppConfig) {
		var target *domain.Profile
		for i := range cfg.Profiles {
			if cfg.Profiles[i].Name == name {
				target = &cfg.Profiles[i]
				break
			}
		}
		if target == nil {
			return
		}

		if target.Aliases == nil {
			target.Aliases = make(domain.AliasMap)
		}

		isActive := cfg.Settings.ActiveProfile == name

		// Remove aliases
		for _, alias := range removeAliases {
			delete(target.Aliases, alias)
			if isActive {
				delete(cfg.ModelAliases, alias)
			}
		}

		// Add/update aliases
		for alias, model := range addAliases {
			target.Aliases[alias] = model
			if isActive {
				if cfg.ModelAliases == nil {
					cfg.ModelAliases = make(domain.AliasMap)
				}
				cfg.ModelAliases[alias] = model
			}
		}

		target.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	})
}

// removeProfileAliasesFromConfig removes a profile's aliases from global modelAliases.
// Must be called within a store.Update callback (config already locked).
func (s *ProfileService) removeProfileAliasesFromConfig(cfg *domain.AppConfig, profileName string) {
	var profile *domain.Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].Name == profileName {
			profile = &cfg.Profiles[i]
			break
		}
	}
	if profile == nil {
		return
	}

	// Only remove aliases that still match the profile's value
	// (don't remove if user manually changed the alias to something else)
	for alias, model := range profile.Aliases {
		if current, ok := cfg.ModelAliases[alias]; ok && current == model {
			delete(cfg.ModelAliases, alias)
		}
	}

	// Remove profile-embedded combos
	for _, profileCombo := range profile.Combos {
		for i, combo := range cfg.Combos {
			if combo.Name == profileCombo.Name {
				cfg.Combos = append(cfg.Combos[:i], cfg.Combos[i+1:]...)
				break
			}
		}
	}
}
