package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
)

// ToolStatus represents the detection/configuration state of a tool.
type ToolStatus struct {
	ID           domain.ToolID `json:"id"`
	Name         string        `json:"name"`
	Installed    bool          `json:"installed"`
	Configured   bool          `json:"configured"`
	ConfigPath   string        `json:"configPath,omitempty"`
	ProxyURL     string        `json:"proxyUrl,omitempty"`
	BackupExists bool          `json:"backupExists,omitempty"`
}

// ToolsService handles detection, configuration, and reset of AI coding tools.
type ToolsService struct {
	store *storage.JsonDB
}

// NewToolsService creates a new ToolsService.
func NewToolsService(store *storage.JsonDB) *ToolsService {
	return &ToolsService{store: store}
}

// ListTools returns the status of all supported tools.
func (s *ToolsService) ListTools() ([]ToolStatus, error) {
	var statuses []ToolStatus
	for _, id := range domain.ListToolIDs() {
		status, err := s.GetStatus(id)
		if err != nil {
			// Non-fatal: include with installed=false
			statuses = append(statuses, ToolStatus{
				ID:   id,
				Name: domain.ToolRegistry[id].Name,
			})
			continue
		}
		statuses = append(statuses, *status)
	}
	return statuses, nil
}

// GetStatus checks whether a tool is installed and configured.
func (s *ToolsService) GetStatus(id domain.ToolID) (*ToolStatus, error) {
	def := domain.GetToolDefinition(id)
	if def == nil {
		return nil, fmt.Errorf("unknown tool: %s", id)
	}

	status := &ToolStatus{
		ID:   id,
		Name: def.Name,
	}

	// Check installation
	status.Installed = s.detectInstalled(def)

	// Check configuration
	configPath := s.resolveConfigPath(def)
	if configPath != "" {
		status.ConfigPath = configPath
		status.Configured = s.isConfigured(configPath, def)
		status.BackupExists = fileExists(configPath + ".dntproxy.bak")
		if status.Configured {
			status.ProxyURL = s.readProxyURL(configPath, def)
		}
	}

	return status, nil
}

// Configure sets up a tool to use dntproxy as its backend.
func (s *ToolsService) Configure(id domain.ToolID) error {
	def := domain.GetToolDefinition(id)
	if def == nil {
		return fmt.Errorf("unknown tool: %s", id)
	}

	configPath := s.resolveConfigPath(def)
	if configPath == "" {
		return fmt.Errorf("cannot determine config path for %s on %s", def.Name, runtime.GOOS)
	}

	proxyURL, apiKey, err := s.getProxyEndpoint()
	if err != nil {
		return fmt.Errorf("get proxy endpoint: %w", err)
	}

	// Backup existing config
	if fileExists(configPath) {
		backupPath := configPath + ".dntproxy.bak"
		if !fileExists(backupPath) {
			data, err := os.ReadFile(configPath)
			if err == nil {
				_ = os.WriteFile(backupPath, data, 0644)
			}
		}
	}

	// Apply configuration based on tool
	return s.applyConfig(def, configPath, proxyURL, apiKey)
}

// Reset reverts a tool's configuration to its pre-dntproxy state.
func (s *ToolsService) Reset(id domain.ToolID) error {
	def := domain.GetToolDefinition(id)
	if def == nil {
		return fmt.Errorf("unknown tool: %s", id)
	}

	configPath := s.resolveConfigPath(def)
	if configPath == "" {
		return fmt.Errorf("cannot determine config path for %s on %s", def.Name, runtime.GOOS)
	}

	backupPath := configPath + ".dntproxy.bak"
	if fileExists(backupPath) {
		// Restore from backup
		data, err := os.ReadFile(backupPath)
		if err != nil {
			return fmt.Errorf("read backup: %w", err)
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("restore config: %w", err)
		}
		_ = os.Remove(backupPath)
		return nil
	}

	// No backup — remove proxy fields from config
	return s.removeProxyConfig(def, configPath)
}

// getProxyEndpoint returns the URL and optional API key for the proxy.
func (s *ToolsService) getProxyEndpoint() (string, string, error) {
	cfg, err := s.store.Load()
	if err != nil {
		return "", "", err
	}

	// Use tunnel URL if available, otherwise localhost
	var baseURL string
	if cfg.Settings.TunnelURL != "" && cfg.Settings.TunnelEnabled {
		baseURL = cfg.Settings.TunnelURL
	} else {
		port := cfg.Settings.Port
		if port == 0 {
			port = 20199
		}
		baseURL = fmt.Sprintf("http://localhost:%d", port)
	}

	// Proxy auth is always enforced by middleware — always inject an active key
	// when available so installed tools can call /v1 without manual setup.
	var apiKey string
	for _, k := range cfg.APIKeys {
		if k.IsActive {
			apiKey = k.Key
			break
		}
	}

	return baseURL, apiKey, nil
}

// detectInstalled checks if a tool is installed by looking for known paths.
func (s *ToolsService) detectInstalled(def *domain.ToolDefinition) bool {
	paths, ok := def.DetectPaths[runtime.GOOS]
	if !ok {
		return false
	}
	for _, p := range paths {
		expanded := expandHome(p)
		if fileExists(expanded) || dirExists(expanded) {
			return true
		}
	}
	return false
}

// resolveConfigPath returns the absolute config path for the current OS.
func (s *ToolsService) resolveConfigPath(def *domain.ToolDefinition) string {
	p, ok := def.ConfigPaths[runtime.GOOS]
	if !ok {
		return ""
	}
	return expandHome(p)
}

// isConfigured checks if the config file contains dntproxy proxy URL.
func (s *ToolsService) isConfigured(configPath string, def *domain.ToolDefinition) bool {
	url := s.readProxyURL(configPath, def)
	return strings.Contains(url, "localhost:") || strings.Contains(url, "dntproxy") || strings.Contains(url, "trycloudflare.com")
}

// readProxyURL reads the current proxy URL from a tool's config.
func (s *ToolsService) readProxyURL(configPath string, def *domain.ToolDefinition) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}

	// Handle nested fields like "provider.baseURL"
	return getNestedString(cfg, def.ProxyURLField)
}

// applyConfig writes the proxy configuration to the tool's config file.
func (s *ToolsService) applyConfig(def *domain.ToolDefinition, configPath, proxyURL, apiKey string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Read existing config or start fresh
	var cfg map[string]interface{}
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	// Set proxy URL field
	setNestedValue(cfg, def.ProxyURLField, proxyURL)

	// Set API key if provided
	if apiKey != "" && def.ProxyKeyField != "" {
		setNestedValue(cfg, def.ProxyKeyField, apiKey)
	}

	// Write config
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// removeProxyConfig removes dntproxy-specific fields from the config.
func (s *ToolsService) removeProxyConfig(def *domain.ToolDefinition, configPath string) error {
	if !fileExists(configPath) {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Remove proxy fields
	deleteNestedValue(cfg, def.ProxyURLField)
	if def.ProxyKeyField != "" {
		deleteNestedValue(cfg, def.ProxyKeyField)
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(configPath, out, 0644)
}

// --- helpers ---

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// getNestedString retrieves a string value from a nested map using dot notation.
// e.g. "provider.baseURL" → cfg["provider"]["baseURL"]
// Array notation like "models[].apiBase" returns the first match.
func getNestedString(cfg map[string]interface{}, field string) string {
	// Handle simple (non-nested) fields
	if !strings.Contains(field, ".") && !strings.Contains(field, "[") {
		if v, ok := cfg[field]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	// Handle array notation: "models[].apiBase"
	if strings.Contains(field, "[]") {
		parts := strings.SplitN(field, "[].", 2)
		if len(parts) != 2 {
			return ""
		}
		arr, ok := cfg[parts[0]].([]interface{})
		if !ok || len(arr) == 0 {
			return ""
		}
		if m, ok := arr[0].(map[string]interface{}); ok {
			if v, ok := m[parts[1]].(string); ok {
				return v
			}
		}
		return ""
	}

	// Handle dot notation: "provider.baseURL"
	parts := strings.Split(field, ".")
	current := interface{}(cfg)
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = m[part]
	}
	if s, ok := current.(string); ok {
		return s
	}
	return ""
}

// setNestedValue sets a value in a nested map using dot notation.
func setNestedValue(cfg map[string]interface{}, field string, value string) {
	// Handle array notation — skip for now (complex tools like Continue)
	if strings.Contains(field, "[]") {
		// For array fields, set at top level as fallback
		parts := strings.SplitN(field, "[].", 2)
		if len(parts) == 2 {
			// Ensure array exists with at least one entry
			arr, ok := cfg[parts[0]].([]interface{})
			if !ok || len(arr) == 0 {
				arr = []interface{}{map[string]interface{}{}}
				cfg[parts[0]] = arr
			}
			if m, ok := arr[0].(map[string]interface{}); ok {
				m[parts[1]] = value
			}
		}
		return
	}

	// Handle simple field
	if !strings.Contains(field, ".") {
		cfg[field] = value
		return
	}

	// Handle dot notation
	parts := strings.Split(field, ".")
	current := cfg
	for i := 0; i < len(parts)-1; i++ {
		next, ok := current[parts[i]].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[parts[i]] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

// deleteNestedValue removes a value from a nested map using dot notation.
func deleteNestedValue(cfg map[string]interface{}, field string) {
	if strings.Contains(field, "[]") {
		// For array fields, remove from first element
		parts := strings.SplitN(field, "[].", 2)
		if len(parts) == 2 {
			if arr, ok := cfg[parts[0]].([]interface{}); ok && len(arr) > 0 {
				if m, ok := arr[0].(map[string]interface{}); ok {
					delete(m, parts[1])
				}
			}
		}
		return
	}

	if !strings.Contains(field, ".") {
		delete(cfg, field)
		return
	}

	parts := strings.Split(field, ".")
	current := cfg
	for i := 0; i < len(parts)-1; i++ {
		next, ok := current[parts[i]].(map[string]interface{})
		if !ok {
			return
		}
		current = next
	}
	delete(current, parts[len(parts)-1])
}
