package backup

import (
	"path/filepath"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
)

func TestValidateBackupRejectsEmptyKeys(t *testing.T) {
	err := ValidateBackup(&BackupData{Version: CurrentBackupVersion})
	if err == nil {
		t.Fatal("empty apiKeys should fail")
	}
}

func TestValidateBackupRejectsTenantOnlyKeys(t *testing.T) {
	err := ValidateBackup(&BackupData{
		Version: CurrentBackupVersion,
		APIKeys: []domain.APIKey{{
			ID: "k1", Name: "tenant", Key: "sk-t", IsActive: true, TenantID: "acme",
		}},
	})
	if err == nil {
		t.Fatal("tenant-only keys should fail")
	}
}

func TestValidateBackupAcceptsAdminKey(t *testing.T) {
	err := ValidateBackup(&BackupData{
		Version: CurrentBackupVersion,
		APIKeys: []domain.APIKey{{
			ID: "k1", Name: "admin", Key: "sk-a", IsActive: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestImportDoesNotOverwriteTunnelURL(t *testing.T) {
	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(cfg *domain.AppConfig) {
		cfg.Settings.TunnelURL = "https://keep.trycloudflare.com"
		cfg.Settings.Port = 20199
		cfg.Settings.RequireAPIKey = true
		cfg.APIKeys = []domain.APIKey{{
			ID: "old", Name: "old", Key: "sk-old", IsActive: true, DashboardAccess: true,
		}}
	}); err != nil {
		t.Fatal(err)
	}

	_, err = Import(db, &BackupData{
		Version: CurrentBackupVersion,
		APIKeys: []domain.APIKey{{
			ID: "new", Name: "admin", Key: "sk-new", IsActive: true, DashboardAccess: true,
		}},
		Settings: domain.Settings{
			TunnelURL:     "https://evil.trycloudflare.com",
			Port:          1,
			RequireAPIKey: false,
			ComboStrategy: "round-robin",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Settings.TunnelURL != "https://keep.trycloudflare.com" {
		t.Fatalf("tunnel URL overwritten: %q", cfg.Settings.TunnelURL)
	}
	if cfg.Settings.Port != 20199 {
		t.Fatalf("port overwritten: %d", cfg.Settings.Port)
	}
	if !cfg.Settings.RequireAPIKey {
		t.Fatal("requireApiKey overwritten")
	}
	if cfg.Settings.ComboStrategy != "round-robin" {
		t.Fatalf("combo strategy not imported: %q", cfg.Settings.ComboStrategy)
	}
	if len(cfg.APIKeys) != 1 || cfg.APIKeys[0].Key != "sk-new" {
		t.Fatalf("keys = %+v", cfg.APIKeys)
	}
}
