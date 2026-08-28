package domain

import "testing"

// The add-connection UI renders one chip per entry in UI.AuthFlows and selects
// UI.PreferredAuthMethod by default. When the preference is not one of the
// advertised flows, the modal falls back to a method the provider's form does
// not render, so the header names one method while the body shows another.
// That mismatch shipped for Kiro ("oauth" vs ["oauth-device", ...]) and is
// invisible without opening the dialog, so pin it here.
func TestPreferredAuthMethodIsAdvertised(t *testing.T) {
	for id, cfg := range ProviderConfigs {
		preferred := cfg.UI.PreferredAuthMethod
		if preferred == "" {
			continue
		}
		found := false
		for _, flow := range cfg.UI.AuthFlows {
			if flow == preferred {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("provider %q: preferredAuthMethod %q is not in authFlows %v", id, preferred, cfg.UI.AuthFlows)
		}
	}
}

// Every provider needs at least one flow, otherwise its card in the picker has
// no way in.
func TestEveryProviderAdvertisesAFlow(t *testing.T) {
	for id, cfg := range ProviderConfigs {
		if len(cfg.UI.AuthFlows) == 0 {
			t.Errorf("provider %q advertises no authFlows", id)
		}
	}
}

// Flow ids are matched against the panels each provider's setup form renders,
// so a typo silently degrades to the wrong panel rather than failing loudly.
// Keep the vocabulary closed.
func TestAuthFlowIDsAreKnown(t *testing.T) {
	known := map[string]bool{
		"apikey": true, "oauth": true, "oauth-device": true,
		"builder-id": true, "idc": true, "social": true,
		"import": true, "file": true, "detect": true, "manual": true,
	}
	for id, cfg := range ProviderConfigs {
		for _, flow := range cfg.UI.AuthFlows {
			if !known[flow] {
				t.Errorf("provider %q advertises unknown authFlow %q", id, flow)
			}
		}
	}
}

// Kiro is the one provider with a hand-written multi-panel form, and the panels
// it renders are exactly these. Locking the set keeps the backend catalog and
// the form from drifting apart again.
func TestKiroAdvertisesEveryRenderedFlow(t *testing.T) {
	want := []string{"builder-id", "social", "idc", "apikey", "import"}
	got := GetProviderConfig("kiro").UI.AuthFlows

	if len(got) != len(want) {
		t.Fatalf("kiro authFlows = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kiro authFlows = %v, want %v", got, want)
		}
	}
}

// Kiro accepts an API key as well as OAuth; the executor keys off this to pick
// the Q endpoint and send TokenType: API_KEY.
func TestKiroSupportsAPIKeyAuth(t *testing.T) {
	methods := GetProviderConfig("kiro").AuthMethods
	for _, m := range methods {
		if m == "apikey" {
			return
		}
	}
	t.Fatalf("kiro authMethods = %v, want to include apikey", methods)
}
