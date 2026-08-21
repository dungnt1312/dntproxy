package backup

import (
	"path/filepath"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
)

func TestParseBackupNormalizesObjectComboStrategies(t *testing.T) {
	data, err := ParseBackup([]byte(`{
		"version": "1.0",
		"settings": {
			"comboStrategies": {
				"native": "round-robin",
				"legacy": {"strategy": "fallback"},
				"typed": {"type": "round-robin"},
				"junk": 123
			}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"native": "round-robin",
		"legacy": "fallback",
		"typed":  "round-robin",
	}
	if len(data.Settings.ComboStrategies) != len(want) {
		t.Fatalf("comboStrategies = %#v", data.Settings.ComboStrategies)
	}
	for name, strategy := range want {
		if data.Settings.ComboStrategies[name] != strategy {
			t.Fatalf("comboStrategies[%q] = %q, want %q", name, data.Settings.ComboStrategies[name], strategy)
		}
	}
}

func TestParseBackupRejectsMalformedComboStrategies(t *testing.T) {
	if _, err := ParseBackup([]byte(`{
		"settings": {"comboStrategies": ["not", "a", "map"]}
	}`)); err == nil {
		t.Fatal("ParseBackup() error = nil, want malformed comboStrategies error")
	}
}

func TestImportAcceptsObjectComboStrategiesShape(t *testing.T) {
	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}

	data, err := ParseBackup([]byte(`{
		"version": "1.0",
		"providerConnections": [],
		"combos": [],
		"modelAliases": {},
		"apiKeys": [{
			"id": "k-admin", "name": "admin", "key": "sk-admin-test",
			"isActive": true, "dashboardAccess": true
		}],
		"settings": {"comboStrategies": {"o1": {"strategy": "fallback"}}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Import(db, data); err != nil {
		t.Fatal(err)
	}

	cfg, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Settings.ComboStrategies["o1"]; got != "fallback" {
		t.Fatalf("comboStrategies[o1] = %q, want fallback", got)
	}
}

func TestParseBackupDefaultModelRegistry(t *testing.T) {
	data, err := ParseBackup([]byte(`{"version": "1.0", "apiKeys": []}`))
	if err != nil {
		t.Fatal(err)
	}
	if data.ModelRegistry != nil {
		t.Fatalf("modelRegistry = %#v, want nil", data.ModelRegistry)
	}
}
