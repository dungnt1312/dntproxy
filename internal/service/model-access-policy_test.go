package service

import (
	"testing"

	"github.com/dungnt/dntproxy/internal/port"
)

func TestNormalizeModelPolicyString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"kr/opus", "kiro/opus"},
		{"oai/gpt-4", "openai/gpt-4"},
		{"glm/glm/glm-5.1", "glm/glm-5.1"},
		{"kiro/sonnet@conn-1", "kiro/sonnet@conn-1"},
		{"kr/sonnet@auto", "kiro/sonnet"},
		{"not-a-model", "not-a-model"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeModelPolicyString(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeModelPolicyString(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestModelPolicyMatch(t *testing.T) {
	tests := []struct {
		model   string
		allowed string
		want    bool
	}{
		{"kiro/opus", "kiro/opus", true},
		{"kr/opus", "kiro/opus", true},
		{"kiro/opus@conn-1", "kiro/opus", true},
		{"kiro/opus@conn-1", "kiro/opus@conn-1", true},
		{"kiro/opus@conn-1", "kiro/opus@conn-2", false},
		{"kiro/opus", "kiro/opus@conn-1", false},
		{"kiro/sonnet", "kiro/opus", false},
		{"glm/glm/glm-5.1", "glm/glm-5.1", true},
		{"oai/gpt-4", "openai/gpt-4", true},
	}
	for _, tt := range tests {
		t.Run(tt.model+"_vs_"+tt.allowed, func(t *testing.T) {
			got := ModelPolicyMatch(tt.model, tt.allowed)
			if got != tt.want {
				t.Errorf("ModelPolicyMatch(%q, %q) = %v, want %v", tt.model, tt.allowed, got, tt.want)
			}
		})
	}
}

func TestConnectionAllowed(t *testing.T) {
	tests := []struct {
		name     string
		connID   string
		policy   *port.APIKeyPolicy
		expected bool
	}{
		{"nil policy", "conn-1", nil, true},
		{"empty allowlist", "conn-1", &port.APIKeyPolicy{}, true},
		{"allowed", "conn-1", &port.APIKeyPolicy{AllowedConnectionIDs: []string{"conn-1", "conn-2"}}, true},
		{"denied", "conn-3", &port.APIKeyPolicy{AllowedConnectionIDs: []string{"conn-1", "conn-2"}}, false},
		{"empty connID", "", &port.APIKeyPolicy{AllowedConnectionIDs: []string{"conn-1"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConnectionAllowed(tt.connID, tt.policy)
			if got != tt.expected {
				t.Errorf("ConnectionAllowed(%q, %+v) = %v, want %v", tt.connID, tt.policy, got, tt.expected)
			}
		})
	}
}

func TestIntersectConnectionIDs(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{"both nil", nil, nil, nil},
		{"a nil", nil, []string{"c1"}, []string{"c1"}},
		{"b nil", []string{"c1"}, nil, []string{"c1"}},
		{"overlap", []string{"c1", "c2"}, []string{"c2", "c3"}, []string{"c2"}},
		{"no overlap", []string{"c1"}, []string{"c2"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IntersectConnectionIDs(tt.a, tt.b)
			if len(got) != len(tt.want) {
				t.Errorf("IntersectConnectionIDs(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("IntersectConnectionIDs(%v, %v)[%d] = %q, want %q", tt.a, tt.b, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestModelAllowedByPolicy(t *testing.T) {
	tests := []struct {
		name   string
		model  string
		policy *port.APIKeyPolicy
		want   bool
	}{
		{"nil policy", "kiro/opus", nil, true},
		{"empty models", "kiro/opus", &port.APIKeyPolicy{}, true},
		{"allowed exact", "kiro/opus", &port.APIKeyPolicy{AllowedModels: []string{"kiro/opus"}}, true},
		{"allowed alias", "kr/opus", &port.APIKeyPolicy{AllowedModels: []string{"kiro/opus"}}, true},
		{"denied", "kiro/sonnet", &port.APIKeyPolicy{AllowedModels: []string{"kiro/opus"}}, false},
		{"pinned allowed", "kiro/opus@c1", &port.APIKeyPolicy{AllowedModels: []string{"kiro/opus"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ModelAllowedByPolicy(tt.model, tt.policy)
			if got != tt.want {
				t.Errorf("ModelAllowedByPolicy(%q, %+v) = %v, want %v", tt.model, tt.policy, got, tt.want)
			}
		})
	}
}
