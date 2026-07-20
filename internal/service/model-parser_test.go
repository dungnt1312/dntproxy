package service

import (
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestParseModelString(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantProvider string
		wantModel    string
		wantConnID   string
		wantErr      bool
	}{
		{
			name:         "basic provider/model",
			input:        "kr/claude-opus-4.6",
			wantProvider: "kiro",
			wantModel:    "claude-opus-4.6",
			wantConnID:   "",
			wantErr:      false,
		},
		{
			name:         "with connection ID",
			input:        "kr/claude-opus-4.6@conn-123",
			wantProvider: "kiro",
			wantModel:    "claude-opus-4.6",
			wantConnID:   "conn-123",
			wantErr:      false,
		},
		{
			name:         "with explicit auto",
			input:        "oai/gpt-4@auto",
			wantProvider: "openai",
			wantModel:    "gpt-4",
			wantConnID:   "",
			wantErr:      false,
		},
		{
			name:         "full provider name",
			input:        "openai/gpt-4@conn-456",
			wantProvider: "openai",
			wantModel:    "gpt-4",
			wantConnID:   "conn-456",
			wantErr:      false,
		},
		{
			name:         "glm provider",
			input:        "glm/glm-4-flash",
			wantProvider: "glm",
			wantModel:    "glm-4-flash",
			wantConnID:   "",
			wantErr:      false,
		},
		{
			name:    "invalid format - no slash",
			input:   "invalid-model",
			wantErr: true,
		},
		{
			name:    "invalid format - empty",
			input:   "",
			wantErr: true,
		},
		{
			name:         "model with multiple slashes",
			input:        "kr/path/to/model@conn-789",
			wantProvider: "kiro",
			wantModel:    "path/to/model",
			wantConnID:   "conn-789",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseModelString(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseModelString() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseModelString() unexpected error: %v", err)
				return
			}

			if got.Provider != tt.wantProvider {
				t.Errorf("ParseModelString() Provider = %v, want %v", got.Provider, tt.wantProvider)
			}

			if got.Model != tt.wantModel {
				t.Errorf("ParseModelString() Model = %v, want %v", got.Model, tt.wantModel)
			}

			if got.ConnectionID != tt.wantConnID {
				t.Errorf("ParseModelString() ConnectionID = %v, want %v", got.ConnectionID, tt.wantConnID)
			}
		})
	}
}

func TestParseModelStringGrokPrefix(t *testing.T) {
	parsed, err := ParseModelString("grok/grok-4.3@conn-xai")
	if err != nil {
		t.Fatalf("ParseModelString() error = %v", err)
	}
	if parsed.Provider != "xai" {
		t.Fatalf("provider = %q, want xai", parsed.Provider)
	}
	if parsed.ProviderAlias != "grok" {
		t.Fatalf("provider alias = %q, want grok", parsed.ProviderAlias)
	}
	if parsed.Model != "grok-4.3" {
		t.Fatalf("model = %q, want grok-4.3", parsed.Model)
	}
	if parsed.ConnectionID != "conn-xai" {
		t.Fatalf("connection ID = %q, want conn-xai", parsed.ConnectionID)
	}
}

func TestNormalizeModelStr(t *testing.T) {
	store := &mockCredentialStore{}
	resolver := NewModelResolver(store)

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "basic alias expansion",
			input:   "kr/opus",
			want:    "kiro/opus",
			wantErr: false,
		},
		{
			name:    "preserve connection ID",
			input:   "kr/opus@conn-123",
			want:    "kiro/opus@conn-123",
			wantErr: false,
		},
		{
			name:    "full provider name with connection",
			input:   "openai/gpt-4@conn-456",
			want:    "openai/gpt-4@conn-456",
			wantErr: false,
		},
		{
			name:    "normalize auto to no suffix",
			input:   "oai/gpt-4@auto",
			want:    "openai/gpt-4",
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.normalizeModelStr(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("normalizeModelStr() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("normalizeModelStr() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("normalizeModelStr() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockCredentialStore for testing
type mockCredentialStore struct{}

func (m *mockCredentialStore) Load() (*domain.AppConfig, error) {
	return &domain.AppConfig{}, nil
}

func (m *mockCredentialStore) Save(cfg *domain.AppConfig) error {
	return nil
}

func (m *mockCredentialStore) Update(fn func(*domain.AppConfig)) error {
	return nil
}

func (m *mockCredentialStore) GetActiveConnections(provider string) ([]domain.ProviderConnection, error) {
	return nil, nil
}

func (m *mockCredentialStore) GetConnectionByID(id string) (*domain.ProviderConnection, error) {
	return nil, nil
}

func (m *mockCredentialStore) UpdateConnection(conn *domain.ProviderConnection) error {
	return nil
}

func (m *mockCredentialStore) GetCombos() ([]domain.Combo, error) {
	return nil, nil
}

func (m *mockCredentialStore) GetComboByName(name string) (*domain.Combo, error) {
	return nil, nil
}

func (m *mockCredentialStore) GetModelAliases() (domain.AliasMap, error) {
	return nil, nil
}

func (m *mockCredentialStore) GetAPIKeys() ([]domain.APIKey, error) {
	return nil, nil
}

func (m *mockCredentialStore) ValidateAPIKey(key string) bool {
	return false
}

func (m *mockCredentialStore) GetAPIKeyByValue(key string) (*domain.APIKey, bool) {
	return nil, false
}

func (m *mockCredentialStore) GetSettings() (*domain.Settings, error) {
	return nil, nil
}

func (m *mockCredentialStore) GetModelRegistry() (*domain.ModelRegistry, error) {
	return nil, nil
}

func (m *mockCredentialStore) GetConnectionIDsForCombo(comboName string) ([]string, error) {
	return nil, nil
}

func (m *mockCredentialStore) GetTenants() ([]domain.Tenant, error) {
	return []domain.Tenant{}, nil
}

func (m *mockCredentialStore) GetTenantBySlug(slug string) (*domain.Tenant, error) {
	return nil, nil
}
