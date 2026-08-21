package domain

// Routing reliability helpers and defaults for Settings.
// ConnectionStrategy allowed values:
//   - weighted-random
//   - priority-fallback
//   - round-robin
//   - fill-first

// CooldownOn reports whether cooldown is enabled.
// nil/true = on so JSON omit keeps default-on.
func (s *Settings) CooldownOn() bool {
	return s.CooldownEnabled == nil || *s.CooldownEnabled
}

// ModelLockOn reports whether model lock is enabled.
// nil/true = on so JSON omit keeps default-on.
func (s *Settings) ModelLockOn() bool {
	return s.ModelLockEnabled == nil || *s.ModelLockEnabled
}

// NormalizeRouting sets safe defaults for routing-related settings.
// Call after JSON load so zero values still get safe TTL defaults when affinity is enabled.
func (s *Settings) NormalizeRouting() {
	if s.SessionAffinityEnabled && s.SessionAffinityTTLSeconds <= 0 {
		s.SessionAffinityTTLSeconds = 1800
	}
	if s.MaxCooldownSeconds < 0 {
		s.MaxCooldownSeconds = 0
	}
	if s.MaxRetryCredentials < 0 {
		s.MaxRetryCredentials = 0
	}
}
