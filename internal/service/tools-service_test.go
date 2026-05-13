package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"~/.config/bar", filepath.Join(home, ".config/bar")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetNestedString(t *testing.T) {
	cfg := map[string]interface{}{
		"apiUrl": "http://localhost:20199",
		"provider": map[string]interface{}{
			"baseURL": "http://example.com",
			"apiKey":  "sk-123",
		},
		"models": []interface{}{
			map[string]interface{}{
				"apiBase": "http://proxy.local",
				"name":    "gpt-4",
			},
		},
	}

	tests := []struct {
		field string
		want  string
	}{
		{"apiUrl", "http://localhost:20199"},
		{"provider.baseURL", "http://example.com"},
		{"provider.apiKey", "sk-123"},
		{"models[].apiBase", "http://proxy.local"},
		{"nonexistent", ""},
		{"provider.nonexistent", ""},
	}

	for _, tt := range tests {
		got := getNestedString(cfg, tt.field)
		if got != tt.want {
			t.Errorf("getNestedString(cfg, %q) = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestSetNestedValue(t *testing.T) {
	t.Run("simple field", func(t *testing.T) {
		cfg := map[string]interface{}{}
		setNestedValue(cfg, "apiUrl", "http://localhost:20199")
		if cfg["apiUrl"] != "http://localhost:20199" {
			t.Errorf("expected apiUrl set, got %v", cfg)
		}
	})

	t.Run("dot notation", func(t *testing.T) {
		cfg := map[string]interface{}{}
		setNestedValue(cfg, "provider.baseURL", "http://localhost:20199")
		provider, ok := cfg["provider"].(map[string]interface{})
		if !ok {
			t.Fatal("expected provider map")
		}
		if provider["baseURL"] != "http://localhost:20199" {
			t.Errorf("expected baseURL set, got %v", provider)
		}
	})

	t.Run("array notation", func(t *testing.T) {
		cfg := map[string]interface{}{}
		setNestedValue(cfg, "models[].apiBase", "http://localhost:20199")
		models, ok := cfg["models"].([]interface{})
		if !ok || len(models) == 0 {
			t.Fatal("expected models array")
		}
		m, ok := models[0].(map[string]interface{})
		if !ok {
			t.Fatal("expected map in array")
		}
		if m["apiBase"] != "http://localhost:20199" {
			t.Errorf("expected apiBase set, got %v", m)
		}
	})
}

func TestDeleteNestedValue(t *testing.T) {
	t.Run("simple field", func(t *testing.T) {
		cfg := map[string]interface{}{"apiUrl": "http://localhost:20199"}
		deleteNestedValue(cfg, "apiUrl")
		if _, ok := cfg["apiUrl"]; ok {
			t.Error("expected apiUrl deleted")
		}
	})

	t.Run("dot notation", func(t *testing.T) {
		cfg := map[string]interface{}{
			"provider": map[string]interface{}{
				"baseURL": "http://localhost:20199",
				"name":    "test",
			},
		}
		deleteNestedValue(cfg, "provider.baseURL")
		provider := cfg["provider"].(map[string]interface{})
		if _, ok := provider["baseURL"]; ok {
			t.Error("expected baseURL deleted")
		}
		if provider["name"] != "test" {
			t.Error("expected name preserved")
		}
	})
}

func TestApplyAndRemoveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	def := &domain.ToolDefinition{
		ID:            domain.ToolClaudeCode,
		ProxyURLField: "apiUrl",
		ProxyKeyField: "apiKey",
	}

	svc := &ToolsService{}

	// Apply config
	err := svc.applyConfig(def, configPath, "http://localhost:20199", "sk-test-123")
	if err != nil {
		t.Fatalf("applyConfig: %v", err)
	}

	// Verify written
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg["apiUrl"] != "http://localhost:20199" {
		t.Errorf("expected apiUrl=http://localhost:20199, got %v", cfg["apiUrl"])
	}
	if cfg["apiKey"] != "sk-test-123" {
		t.Errorf("expected apiKey=sk-test-123, got %v", cfg["apiKey"])
	}

	// Remove config
	err = svc.removeProxyConfig(def, configPath)
	if err != nil {
		t.Fatalf("removeProxyConfig: %v", err)
	}

	data, _ = os.ReadFile(configPath)
	cfg = nil
	_ = json.Unmarshal(data, &cfg)

	if _, ok := cfg["apiUrl"]; ok {
		t.Error("expected apiUrl removed")
	}
	if _, ok := cfg["apiKey"]; ok {
		t.Error("expected apiKey removed")
	}
}

func TestToolRegistry(t *testing.T) {
	ids := domain.ListToolIDs()
	if len(ids) == 0 {
		t.Fatal("expected at least one tool in registry")
	}

	// Verify all tools have required fields
	for _, id := range ids {
		def := domain.GetToolDefinition(id)
		if def == nil {
			t.Errorf("GetToolDefinition(%s) returned nil", id)
			continue
		}
		if def.Name == "" {
			t.Errorf("tool %s has empty Name", id)
		}
		if def.ProxyURLField == "" {
			t.Errorf("tool %s has empty ProxyURLField", id)
		}
		if len(def.ConfigPaths) == 0 {
			t.Errorf("tool %s has no ConfigPaths", id)
		}
		if len(def.DetectPaths) == 0 {
			t.Errorf("tool %s has no DetectPaths", id)
		}
	}
}

func TestGetToolDefinitionUnknown(t *testing.T) {
	def := domain.GetToolDefinition("nonexistent-tool")
	if def != nil {
		t.Error("expected nil for unknown tool")
	}
}
