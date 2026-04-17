package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/gofrs/flock"
)

// JsonDB implements port.CredentialStore using a JSON file with file locking.
type JsonDB struct {
	filePath string
	mu       sync.Mutex
	fileLock *flock.Flock
	cache    *domain.AppConfig

	// Config cache to reduce disk I/O (TTL-based, invalidated on Save)
	configCacheMu  sync.RWMutex
	cachedConfig   *domain.AppConfig
	cachedConfigAt time.Time
	configCacheTTL time.Duration
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
		if err := os.WriteFile(path, data, 0600); err != nil {
			return nil, fmt.Errorf("seed db file: %w", err)
		}
	}

	return &JsonDB{
		filePath:       path,
		fileLock:       flock.New(path + ".lock"),
		configCacheTTL: 2 * time.Second,
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

// Load reads the config from disk (with TTL-based cache).
func (db *JsonDB) Load() (*domain.AppConfig, error) {
	// Fast path: return from cache if valid
	db.configCacheMu.RLock()
	if db.cachedConfig != nil && time.Since(db.cachedConfigAt) < db.configCacheTTL {
		cfg, err := cloneConfig(db.cachedConfig)
		db.configCacheMu.RUnlock()
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	db.configCacheMu.RUnlock()

	// Slow path: acquire file lock and read from disk
	db.configCacheMu.Lock()
	defer db.configCacheMu.Unlock()

	// Double-check after acquiring write lock
	if db.cachedConfig != nil && time.Since(db.cachedConfigAt) < db.configCacheTTL {
		return cloneConfig(db.cachedConfig)
	}

	cfg, err := db.readFromDisk()
	if err != nil {
		return nil, err
	}

	db.cachedConfig, err = cloneConfig(cfg)
	if err != nil {
		return nil, err
	}
	db.cachedConfigAt = time.Now()

	return cloneConfig(cfg)
}

func (db *JsonDB) readFromDisk() (*domain.AppConfig, error) {
	data, err := os.ReadFile(db.filePath)
	if err != nil {
		return nil, fmt.Errorf("read db file: %w", err)
	}

	var cfg domain.AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		cfg = domain.DefaultConfig()
	}

	// Migrate: ensure all connections have a valid weight.
	// Old configs may have Priority (now removed) or Weight=0.
	for i := range cfg.ProviderConnections {
		if cfg.ProviderConnections[i].Weight <= 0 {
			cfg.ProviderConnections[i].Weight = 100
		}
	}

	db.cache = &cfg
	return &cfg, nil
}

// Save writes the config to disk atomically (invalidates cache).
func (db *JsonDB) Save(cfg *domain.AppConfig) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if err := db.fileLock.Lock(); err != nil {
		return fmt.Errorf("acquire file lock: %w", err)
	}
	defer db.fileLock.Unlock()

	if err := db.writeToDisk(cfg); err != nil {
		return err
	}

	// Update cache with new config
	cacheCfg, err := cloneConfig(cfg)
	if err != nil {
		return err
	}
	db.configCacheMu.Lock()
	db.cachedConfig = cacheCfg
	db.cachedConfigAt = time.Now()
	db.configCacheMu.Unlock()

	return nil
}

func (db *JsonDB) writeToDisk(cfg *domain.AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmp := db.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, db.filePath); err != nil {
		return fmt.Errorf("rename temp to db: %w", err)
	}

	cacheCfg, err := cloneConfig(cfg)
	if err != nil {
		return err
	}
	db.cache = cacheCfg
	return nil
}

// GetActiveConnections returns active connections for a provider.
// No sorting: the AccountSelector uses weighted random selection.
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

// Update loads config, applies fn, and saves atomically under a single lock.
func (db *JsonDB) Update(fn func(cfg *domain.AppConfig)) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if err := db.fileLock.Lock(); err != nil {
		return fmt.Errorf("acquire file lock: %w", err)
	}
	defer db.fileLock.Unlock()

	cfg, err := db.readFromDisk()
	if err != nil {
		return err
	}

	fn(cfg)

	if err := db.writeToDisk(cfg); err != nil {
		return err
	}

	// Update cache with new config
	cacheCfg, err := cloneConfig(cfg)
	if err != nil {
		return err
	}
	db.configCacheMu.Lock()
	db.cachedConfig = cacheCfg
	db.cachedConfigAt = time.Now()
	db.configCacheMu.Unlock()

	return nil
}

func cloneConfig(cfg *domain.AppConfig) (*domain.AppConfig, error) {
	if cfg == nil {
		return nil, nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("clone config marshal: %w", err)
	}
	var cloned domain.AppConfig
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("clone config unmarshal: %w", err)
	}
	return &cloned, nil
}

// UpdateConnection persists changes to a connection (atomic).
func (db *JsonDB) UpdateConnection(conn *domain.ProviderConnection) error {
	return db.Update(func(cfg *domain.AppConfig) {
		for i := range cfg.ProviderConnections {
			if cfg.ProviderConnections[i].ID == conn.ID {
				cfg.ProviderConnections[i] = *conn
				return
			}
		}
	})
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

// GetConnectionIDsForCombo returns ConnectionIDs for a combo name (empty if combo not found or no restriction).
func (db *JsonDB) GetConnectionIDsForCombo(comboName string) ([]string, error) {
	combo, err := db.GetComboByName(comboName)
	if err != nil || combo == nil {
		return nil, err
	}
	return combo.ConnectionIDs, nil
}
