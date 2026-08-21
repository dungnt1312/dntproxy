package backup

import (
	"path/filepath"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
	"github.com/dungnt/dntproxy/internal/domain"
)

func newImportTestDB(t *testing.T, connections ...domain.ProviderConnection) *storage.JsonDB {
	t.Helper()
	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(cfg *domain.AppConfig) {
		cfg.ProviderConnections = connections
	}); err != nil {
		t.Fatal(err)
	}
	return db
}

func apiKeyConnection(id string) domain.ProviderConnection {
	return domain.ProviderConnection{
		ID: id, Provider: "openai", AuthType: "apikey", Name: id, APIKey: "sk-test", IsActive: true,
	}
}

func TestImportConnectionsRejectsDuplicateIDsInFile(t *testing.T) {
	db := newImportTestDB(t)
	_, err := ImportConnections(db, &BackupData{
		Version: CurrentBackupVersion,
		ProviderConnections: []domain.ProviderConnection{
			apiKeyConnection("duplicate"),
			apiKeyConnection("duplicate"),
		},
	}, ImportModeMerge)
	if err == nil {
		t.Fatal("ImportConnections() error = nil, want duplicate ID error")
	}

	cfg, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ProviderConnections) != 0 {
		t.Fatalf("connections = %#v, want none persisted", cfg.ProviderConnections)
	}
}

func TestImportConnectionsMergeNormalizesRuntimeStateAndModels(t *testing.T) {
	db := newImportTestDB(t, apiKeyConnection("existing"))
	conn := apiKeyConnection("new")
	conn.TestStatus = "failed"
	conn.LastError = "expired"
	conn.LastErrorAt = "2025-01-01T00:00:00Z"
	conn.RateLimitedUntil = "2027-01-01T00:00:00Z"
	conn.BackoffLevel = 3
	conn.ConsecutiveUseCount = 9
	conn.ModelLocks = map[string]string{"gpt-4": "locked"}

	result, err := ImportConnections(db, &BackupData{
		Version:             CurrentBackupVersion,
		ProviderConnections: []domain.ProviderConnection{conn, apiKeyConnection("existing")},
	}, ImportModeMerge)
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 1 || result.Skipped != 1 {
		t.Fatalf("result = %#v", result)
	}

	cfg, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	var imported domain.ProviderConnection
	for _, candidate := range cfg.ProviderConnections {
		if candidate.ID == "new" {
			imported = candidate
		}
	}
	if imported.ID == "" {
		t.Fatal("new connection was not persisted")
	}
	if imported.TestStatus != "" || imported.LastError != "" || imported.LastErrorAt != "" || imported.RateLimitedUntil != "" || imported.BackoffLevel != 0 || imported.ConsecutiveUseCount != 0 || imported.ModelLocks != nil {
		t.Fatalf("runtime state was retained: %#v", imported)
	}
	if len(imported.SupportedModels) == 0 {
		t.Fatal("default supported models were not applied")
	}
	if imported.UpdatedAt == "" {
		t.Fatal("updatedAt was not set")
	}
}

func TestParse9RouterBackupMapsCodexToOpenAI(t *testing.T) {
	converted, err := Parse9RouterBackup([]byte(`{
		"providerConnections": [{
			"id": "source-codex", "provider": "codex", "authType": "oauth",
			"name": "Imported account", "isActive": true,
			"accessToken": "test-access-token", "refreshToken": "test-refresh-token",
			"expiresAt": "2026-09-01T00:00:00Z", "expiresIn": 3600,
			"providerSpecificData": {"idToken": "test-id-token"}
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(converted.Skipped) != 0 {
		t.Fatalf("skipped = %#v", converted.Skipped)
	}
	if converted.Data.Version != CurrentBackupVersion || len(converted.Data.ProviderConnections) != 1 {
		t.Fatalf("converted backup = %#v", converted.Data)
	}
	conn := converted.Data.ProviderConnections[0]
	if conn.Provider != "openai" || conn.AuthType != "oauth" || conn.RefreshToken != "test-refresh-token" {
		t.Fatalf("converted connection has unexpected mapping")
	}
	if conn.ProviderSpecificData["authMethod"] != "oauth" || conn.ProviderSpecificData["idToken"] != "test-id-token" {
		t.Fatalf("provider metadata = %#v", conn.ProviderSpecificData)
	}
}

func TestParse9RouterBackupMapsSupportedProviders(t *testing.T) {
	converted, err := Parse9RouterBackup([]byte(`{
		"providerConnections": [
			{"id": "g1", "provider": "glm", "authType": "apikey", "apiKey": "test-glm-key", "name": "glm"},
			{"id": "c1", "provider": "commandcode", "authType": "apikey", "apiKey": "test-cc-key", "name": "cc"},
			{"id": "x1", "provider": "xai", "authType": "oauth", "refreshToken": "test-xai-refresh", "providerSpecificData": {"idToken": "test-xai-id"}, "name": "xai"},
			{"id": "k1", "provider": "kiro", "authType": "oauth", "refreshToken": "test-kiro-refresh", "providerSpecificData": {"clientId": "cid", "clientSecret": "csec", "region": "us-east-1", "authMethod": "idc"}, "name": "kiro"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(converted.Skipped) != 0 || len(converted.Data.ProviderConnections) != 4 {
		t.Fatalf("converted = %d conns, skipped %#v", len(converted.Data.ProviderConnections), converted.Skipped)
	}
	byID := map[string]domain.ProviderConnection{}
	for _, conn := range converted.Data.ProviderConnections {
		byID[conn.ID] = conn
	}
	if byID["g1"].APIKey != "test-glm-key" || byID["g1"].Provider != "glm" {
		t.Fatalf("glm mapping = %#v", byID["g1"])
	}
	if byID["c1"].APIKey != "test-cc-key" || byID["c1"].Provider != "commandcode" {
		t.Fatalf("commandcode mapping = %#v", byID["c1"])
	}
	if byID["x1"].Provider != "xai" || byID["x1"].ProviderSpecificData["authMethod"] != "xai-oauth" || byID["x1"].ProviderSpecificData["idToken"] != "test-xai-id" {
		t.Fatalf("xai mapping = %#v", byID["x1"])
	}
	if byID["k1"].Provider != "kiro" || byID["k1"].ProviderSpecificData["clientId"] != "cid" || byID["k1"].ProviderSpecificData["region"] != "us-east-1" {
		t.Fatalf("kiro mapping = %#v", byID["k1"])
	}
}

func TestParse9RouterBackupSkipsUnsupportedProvider(t *testing.T) {
	converted, err := Parse9RouterBackup([]byte(`{
		"providerConnections": [
			{"id": "codex", "provider": "codex", "authType": "oauth", "refreshToken": "test-refresh-token", "name": "ok"},
			{"id": "unsupported", "provider": "claude", "authType": "oauth", "refreshToken": "test-refresh-token", "name": "claude"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(converted.Data.ProviderConnections) != 1 || converted.Data.ProviderConnections[0].ID != "codex" {
		t.Fatalf("converted = %#v, want only codex", converted.Data.ProviderConnections)
	}
	if len(converted.Skipped) != 1 {
		t.Fatalf("skipped = %#v, want 1 reason", converted.Skipped)
	}
}

func TestConvert9RouterBackupDoesNotRewriteVersionedData(t *testing.T) {
	_, err := Convert9RouterBackup(&BackupData{
		Version: CurrentBackupVersion,
		ProviderConnections: []domain.ProviderConnection{{
			ID: "codex", Provider: "codex", AuthType: "oauth", RefreshToken: "test-refresh-token",
		}},
	})
	if err == nil {
		t.Fatal("Convert9RouterBackup() error = nil, want versioned data rejection")
	}
}

func TestImportConnectionsValidatesCredentialsBeforeSaving(t *testing.T) {
	db := newImportTestDB(t)
	invalid := apiKeyConnection("missing-key")
	invalid.APIKey = ""

	_, err := ImportConnections(db, &BackupData{
		Version:             CurrentBackupVersion,
		ProviderConnections: []domain.ProviderConnection{apiKeyConnection("valid"), invalid},
	}, ImportModeMerge)
	if err == nil {
		t.Fatal("ImportConnections() error = nil, want credential validation failure")
	}

	cfg, err := db.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ProviderConnections) != 0 {
		t.Fatalf("connections = %#v, want no partial import", cfg.ProviderConnections)
	}
}
