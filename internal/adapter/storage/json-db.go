package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gofrs/flock"
)

// JsonDB implements port.CredentialStore using a JSON file with file locking.
type JsonDB struct {
	filePath string
	mu       sync.Mutex
	fileLock *flock.Flock
	cache    *domain.AppConfig
}

// NewJsonDB creates a new JsonDB, ensuring the directory and file exist.
func NewJsonDB(path string) (*JsonDB, error) {
	if path == "" {
		path = defaultDBPath()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Seed file if missing
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := domain.DefaultConfig()
		data, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(path, data, 0644); err != nil {
			return nil, fmt.Errorf("seed db file: %w", err)
		}
	}

	return &JsonDB{
		filePath: path,
		fileLock: flock.New(path + ".lock"),
	}, nil
}

// DataDir returns the directory where app data is stored.
func (db *JsonDB) DataDir() string {
	return filepath.Dir(db.filePath)
}

func defaultDBPath() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "dntproxy", "db.json")
		}
	}
	return filepath.Join(home, ".dntproxy", "db.json")
}

// Load reads the config from disk.
func (db *JsonDB) Load() (*domain.AppConfig, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if err := db.fileLock.Lock(); err != nil {
		return nil, fmt.Errorf("acquire file lock: %w", err)
	}
	defer db.fileLock.Unlock()

	data, err := os.ReadFile(db.filePath)
	if err != nil {
		return nil, fmt.Errorf("read db file: %w", err)
	}

	var cfg domain.AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Corrupt JSON — reset to defaults
		cfg = domain.DefaultConfig()
	}

	db.cache = &cfg
	return &cfg, nil
}

// Save writes the config to disk atomically.
func (db *JsonDB) Save(cfg *domain.AppConfig) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if err := db.fileLock.Lock(); err != nil {
		return fmt.Errorf("acquire file lock: %w", err)
	}
	defer db.fileLock.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Atomic write: temp file → rename
	tmp := db.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, db.filePath); err != nil {
		return fmt.Errorf("rename temp to db: %w", err)
	}

	db.cache = cfg
	return nil
}

// GetActiveConnections returns active connections for a provider, sorted by priority.
func (db *JsonDB) GetActiveConnections(provider string) ([]domain.ProviderConnection, error) {
	cfg, err := db.Load()
	if err != nil {
		return nil, err
	}

	var result []domain.ProviderConnection
	for _, c := range cfg.ProviderConnections {
		if c.Provider == provider && c.IsActive {
			result = append(result, c)
		}
	}

	// Sort by priority (lower = higher priority)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Priority < result[i].Priority {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// GetConnectionByID returns a single connection by ID.
func (db *JsonDB) GetConnectionByID(id string) (*domain.ProviderConnection, error) {
	cfg, err := db.Load()
	if err != nil {
		return nil, err
	}

	for i := range cfg.ProviderConnections {
		if cfg.ProviderConnections[i].ID == id {
			return &cfg.ProviderConnections[i], nil
		}
	}
	return nil, nil
}

// UpdateConnection persists changes to a connection.
func (db *JsonDB) UpdateConnection(conn *domain.ProviderConnection) error {
	cfg, err := db.Load()
	if err != nil {
		return err
	}

	for i := range cfg.ProviderConnections {
		if cfg.ProviderConnections[i].ID == conn.ID {
			cfg.ProviderConnections[i] = *conn
			return db.Save(cfg)
		}
	}
	return fmt.Errorf("connection %s not found", conn.ID)
}

// GetCombos returns all combos.
func (db *JsonDB) GetCombos() ([]domain.Combo, error) {
	cfg, err := db.Load()
	if err != nil {
		return nil, err
	}
	return cfg.Combos, nil
}

// GetComboByName returns a combo by name.
func (db *JsonDB) GetComboByName(name string) (*domain.Combo, error) {
	cfg, err := db.Load()
	if err != nil {
		return nil, err
	}

	for i := range cfg.Combos {
		if cfg.Combos[i].Name == name {
			return &cfg.Combos[i], nil
		}
	}
	return nil, nil
}

// GetModelAliases returns all model aliases.
func (db *JsonDB) GetModelAliases() (domain.AliasMap, error) {
	cfg, err := db.Load()
	if err != nil {
		return nil, err
	}
	return cfg.ModelAliases, nil
}

// GetAPIKeys returns all API keys.
func (db *JsonDB) GetAPIKeys() ([]domain.APIKey, error) {
	cfg, err := db.Load()
	if err != nil {
		return nil, err
	}
	return cfg.APIKeys, nil
}

// ValidateAPIKey checks if a key string is valid and active.
func (db *JsonDB) ValidateAPIKey(key string) bool {
	cfg, err := db.Load()
	if err != nil {
		return false
	}
	for _, k := range cfg.APIKeys {
		if k.Key == key && k.IsActive {
			return true
		}
	}
	return false
}

// GetSettings returns app settings.
func (db *JsonDB) GetSettings() (*domain.Settings, error) {
	cfg, err := db.Load()
	if err != nil {
		return nil, err
	}
	return &cfg.Settings, nil
}

// GetModelRegistry returns the model registry.
func (db *JsonDB) GetModelRegistry() (*domain.ModelRegistry, error) {
	cfg, err := db.Load()
	if err != nil {
		return nil, err
	}
	if cfg.ModelRegistry == nil {
		cfg.ModelRegistry = domain.DefaultModelRegistry()
	}
	return cfg.ModelRegistry, nil
}
