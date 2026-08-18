package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestLoadRejectsCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := NewJsonDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Load(); err == nil {
		t.Fatal("Load succeeded on corrupt JSON")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "{not-json" {
		t.Fatalf("corrupt file was overwritten: %q", raw)
	}
}

func TestDefaultConfigRequiresAPIKey(t *testing.T) {
	if !domain.DefaultConfig().Settings.RequireAPIKey {
		t.Fatal("default RequireAPIKey should be true")
	}
}

func TestDefaultConnectionStrategyIsWeightedRandom(t *testing.T) {
	if got := domain.DefaultConfig().Settings.ConnectionStrategy; got != "weighted-random" {
		t.Fatalf("ConnectionStrategy = %q, want weighted-random", got)
	}
}
