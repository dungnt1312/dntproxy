package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

type cliToolFakeStore struct {
	cfg *domain.AppConfig
}

func newCLIToolFakeStore() *cliToolFakeStore {
	cfg := domain.DefaultConfig()
	return &cliToolFakeStore{cfg: &cfg}
}

func (s *cliToolFakeStore) Load() (*domain.AppConfig, error) { return s.cfg, nil }
func (s *cliToolFakeStore) Save(cfg *domain.AppConfig) error {
	s.cfg = cfg
	return nil
}
func (s *cliToolFakeStore) Update(fn func(cfg *domain.AppConfig)) error {
	fn(s.cfg)
	return nil
}
func (s *cliToolFakeStore) GetActiveConnections(string) ([]domain.ProviderConnection, error) {
	return nil, nil
}
func (s *cliToolFakeStore) GetConnectionByID(string) (*domain.ProviderConnection, error) {
	return nil, nil
}
func (s *cliToolFakeStore) UpdateConnection(*domain.ProviderConnection) error { return nil }
func (s *cliToolFakeStore) GetCombos() ([]domain.Combo, error)                { return nil, nil }
func (s *cliToolFakeStore) GetComboByName(string) (*domain.Combo, error)      { return nil, nil }
func (s *cliToolFakeStore) GetModelAliases() (domain.AliasMap, error) {
	return s.cfg.ModelAliases, nil
}
func (s *cliToolFakeStore) GetAPIKeys() ([]domain.APIKey, error) { return nil, nil }
func (s *cliToolFakeStore) ValidateAPIKey(string) bool           { return true }
func (s *cliToolFakeStore) GetSettings() (*domain.Settings, error) {
	return &s.cfg.Settings, nil
}
func (s *cliToolFakeStore) GetModelRegistry() (*domain.ModelRegistry, error) {
	return s.cfg.ModelRegistry, nil
}
func (s *cliToolFakeStore) GetConnectionIDsForCombo(string) ([]string, error) {
	return nil, nil
}

func TestRenderClaudeConfigPreservesExistingJSON(t *testing.T) {
	models := map[string]string{"sonnet": "claude-sonnet", "opus": "claude-opus", "haiku": "claude-haiku"}
	content, err := renderClaudeConfig([]byte(`{"theme":"dark","env":{"KEEP":"yes"}}`), "http://127.0.0.1:20199/v1", "sk-test", models)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"theme": "dark"`,
		`"KEEP": "yes"`,
		`"ANTHROPIC_BASE_URL": "http://127.0.0.1:20199/v1"`,
		`"ANTHROPIC_DEFAULT_SONNET_MODEL": "claude-sonnet"`,
		`"ANTHROPIC_DEFAULT_OPUS_MODEL": "claude-opus"`,
		`"ANTHROPIC_DEFAULT_HAIKU_MODEL": "claude-haiku"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("rendered Claude config missing %q:\n%s", want, content)
		}
	}
	// Legacy ANTHROPIC_MODEL should not be present
	if strings.Contains(content, "ANTHROPIC_MODEL") {
		t.Fatalf("rendered Claude config should not contain legacy ANTHROPIC_MODEL:\n%s", content)
	}
}

func TestRenderCodexConfigPreservesExistingTOML(t *testing.T) {
	models := map[string]string{"model": "dntproxy-gpt-4o"}
	content, err := renderCodexConfig([]byte("approval_policy = 'on-request'\n"), "http://127.0.0.1:20199/v1", models)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`approval_policy = 'on-request'`, `model_provider = 'dntproxy'`, `base_url = 'http://127.0.0.1:20199/v1'`, `wire_api = 'chat'`} {
		if !strings.Contains(content, want) {
			t.Fatalf("rendered Codex config missing %q:\n%s", want, content)
		}
	}
}

func TestRenderOpenCodeConfigUsesDntproxyProvider(t *testing.T) {
	models := map[string]string{"model": "kr/claude-sonnet-4.5"}
	content, err := renderOpenCodeConfig([]byte(`{}`), "https://dntproxy.example.com/v1", "sk-test", models)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"dntproxy/kr/claude-sonnet-4.5"`,
		`"@ai-sdk/openai-compatible"`,
		`"baseURL": "https://dntproxy.example.com/v1"`,
		`"apiKey": "sk-test"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("rendered OpenCode config missing %q:\n%s", want, content)
		}
	}
}

func TestApplyCreatesAliasBackupAndRestore(t *testing.T) {
	home := t.TempDir()
	store := newCLIToolFakeStore()
	now := func() time.Time { return time.Date(2026, 4, 24, 10, 11, 12, 0, time.UTC) }
	svc := NewCLIToolsServiceForTest(store, home, now)
	claudePath := filepath.Join(home, ".claude", "settings.json")

	if err := os.MkdirAll(filepath.Dir(claudePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudePath, []byte(`{"env":{"OLD":"1"}}`), 0600); err != nil {
		t.Fatal(err)
	}

	resp, err := svc.Apply(domain.CLIToolsConfigRequest{
		Endpoint: "http://127.0.0.1:20199/v1",
		APIKey:   "sk-test",
		Models:   map[string]string{"sonnet": "oai/gpt-4o", "opus": "oai/gpt-4o", "haiku": "oai/gpt-4o"},
		Tools:    []domain.CLIToolID{domain.CLIToolClaudeCode},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should have created alias for oai/gpt-4o
	alias := "dntproxy-gpt-4o"
	if store.cfg.ModelAliases[alias] != "oai/gpt-4o" {
		t.Fatalf("alias not created correctly: %#v", store.cfg.ModelAliases)
	}
	if len(resp.Results) != 1 || !resp.Results[0].Applied || resp.Results[0].BackupPath == "" {
		t.Fatalf("apply result missing backup: %#v", resp.Results)
	}

	restored := svc.Restore(domain.CLIToolsRestoreRequest{Tools: []domain.CLIToolID{domain.CLIToolClaudeCode}})
	if len(restored) != 1 || !restored[0].Restored {
		t.Fatalf("restore failed: %#v", restored)
	}
	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"OLD":"1"`) {
		t.Fatalf("restore did not recover original config: %s", string(data))
	}
}
