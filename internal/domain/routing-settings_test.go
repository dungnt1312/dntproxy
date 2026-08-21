package domain

import "testing"

func TestSettingsNormalizeRoutingDefaults(t *testing.T) {
	s := Settings{SessionAffinityEnabled: true}
	s.NormalizeRouting()
	if s.SessionAffinityTTLSeconds != 1800 {
		t.Fatalf("ttl=%d", s.SessionAffinityTTLSeconds)
	}
	if !s.CooldownOn() || !s.ModelLockOn() {
		t.Fatal("cooldown/model lock should default on")
	}
}

func TestSettingsMaxRetryCredentialsNegativeNormalized(t *testing.T) {
	s := Settings{MaxRetryCredentials: -3}
	s.NormalizeRouting()
	if s.MaxRetryCredentials != 0 {
		t.Fatalf("got %d", s.MaxRetryCredentials)
	}
}
