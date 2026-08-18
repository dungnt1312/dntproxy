package backup

import (
	"path/filepath"
	"testing"

	"github.com/dungnt/dntproxy/internal/adapter/storage"
)

func TestExportConnectionsRejectsEmptyList(t *testing.T) {
	db, err := storage.NewJsonDB(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExportConnections(db, nil); err == nil {
		t.Fatal("empty export should fail")
	}
}
