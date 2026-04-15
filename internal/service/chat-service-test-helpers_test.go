package service

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// === Test CredentialStore ===

type testCredentialStore struct {
	mu  sync.Mutex
	cfg *domain.AppConfig
}

func newTestCredentialStore(cfg *domain.AppConfig) *testCredentialStore {
	if cfg.ModelRegistry == nil {
		cfg.ModelRegistry = domain.DefaultModelRegistry()
	}
	if cfg.ModelAliases == nil {
		cfg.ModelAliases = domain.AliasMap{}
	}
	return &testCredentialStore{cfg: cfg}
}

func (s *testCredentialStore) Load() (*domain.AppConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg, nil
}

func (s *testCredentialStore) Save(cfg *domain.AppConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
	return nil
}

func (s *testCredentialStore) GetActiveConnections(provider string) ([]domain.ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]domain.ProviderConnection, 0)
	for _, c := range s.cfg.ProviderConnections {
		if c.Provider == provider && c.IsActive {
			result = append(result, c)
		}
	}
	return result, nil
}

func (s *testCredentialStore) GetConnectionByID(id string) (*domain.ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cfg.ProviderConnections {
		if s.cfg.ProviderConnections[i].ID == id {
			copyConn := s.cfg.ProviderConnections[i]
			return &copyConn, nil
		}
	}
	return nil, nil
}

func (s *testCredentialStore) Update(fn func(cfg *domain.AppConfig)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.cfg)
	return nil
}

func (s *testCredentialStore) UpdateConnection(conn *domain.ProviderConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cfg.ProviderConnections {
		if s.cfg.ProviderConnections[i].ID == conn.ID {
			s.cfg.ProviderConnections[i] = *conn
			return nil
		}
	}
	return fmt.Errorf("connection %s not found", conn.ID)
}

func (s *testCredentialStore) GetCombos() ([]domain.Combo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Combos, nil
}

func (s *testCredentialStore) GetComboByName(name string) (*domain.Combo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cfg.Combos {
		if s.cfg.Combos[i].Name == name {
			combo := s.cfg.Combos[i]
			return &combo, nil
		}
	}
	return nil, nil
}

func (s *testCredentialStore) GetModelAliases() (domain.AliasMap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.ModelAliases, nil
}

func (s *testCredentialStore) GetAPIKeys() ([]domain.APIKey, error) {
	return nil, nil
}

func (s *testCredentialStore) ValidateAPIKey(key string) bool {
	return false
}

func (s *testCredentialStore) GetSettings() (*domain.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings := s.cfg.Settings
	return &settings, nil
}

func (s *testCredentialStore) GetModelRegistry() (*domain.ModelRegistry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.ModelRegistry, nil
}

func (s *testCredentialStore) GetConnectionIDsForCombo(comboName string) ([]string, error) {
	combo, err := s.GetComboByName(comboName)
	if err != nil || combo == nil {
		return nil, err
	}
	return combo.ConnectionIDs, nil
}

// === Test ProviderRegistry ===

type testProviderRegistry struct {
	executors map[string]port.ProviderExecutor
}

func newTestProviderRegistry() *testProviderRegistry {
	return &testProviderRegistry{executors: make(map[string]port.ProviderExecutor)}
}

func (r *testProviderRegistry) GetExecutor(provider string) port.ProviderExecutor {
	return r.executors[provider]
}

func (r *testProviderRegistry) RegisterExecutor(provider string, executor port.ProviderExecutor) {
	r.executors[provider] = executor
}

func (r *testProviderRegistry) SupportedProviders() []string {
	providers := make([]string, 0, len(r.executors))
	for p := range r.executors {
		providers = append(providers, p)
	}
	return providers
}

// === Fake Executor ===

type fakeExecuteCall struct {
	ConnectionID string
	Model        string
}

type fakeExecuteResponse struct {
	Status int
	Err    error
	Body   string
}

type fakeExecutor struct {
	calls     []fakeExecuteCall
	responses map[string]fakeExecuteResponse
}

func newFakeExecutor(responses map[string]fakeExecuteResponse) *fakeExecutor {
	return &fakeExecutor{responses: responses, calls: make([]fakeExecuteCall, 0)}
}

func (e *fakeExecutor) Execute(model string, _ []byte, credentials *domain.Credentials, _ port.RequestLogger) (io.ReadCloser, int, error) {
	e.calls = append(e.calls, fakeExecuteCall{ConnectionID: credentials.ConnectionID, Model: model})
	key := credentials.ConnectionID + "|" + model
	resp, ok := e.responses[key]
	if !ok {
		return nil, 500, fmt.Errorf("missing fake response for %s", key)
	}
	if resp.Status == 200 {
		if resp.Body == "" {
			resp.Body = "data: [DONE]\n\n"
		}
		return io.NopCloser(strings.NewReader(resp.Body)), 200, nil
	}
	if resp.Err == nil {
		resp.Err = fmt.Errorf("status %d", resp.Status)
	}
	return nil, resp.Status, resp.Err
}

func (e *fakeExecutor) callCountForModel(model string) int {
	count := 0
	for _, call := range e.calls {
		if call.Model == model {
			count++
		}
	}
	return count
}
