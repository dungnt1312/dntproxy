package commandcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAuthFile(t *testing.T) {
	got, err := ParseAuthFile([]byte(`{"apiKey":" user_abc ","userName":"dung","keyName":"cli"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "user_abc" || got.DisplayName() != "dung" {
		t.Fatalf("got %+v", got)
	}
	if _, err := ParseAuthFile([]byte(`{"userName":"x"}`)); err == nil {
		t.Fatal("expected missing apiKey error")
	}
}

func TestFindAuthFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(path, []byte(`{"apiKey":"user_live","userName":"tester"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := FindAuthFile([]string{
		filepath.Join(dir, "missing.json"),
		path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "user_live" || got.Source != path {
		t.Fatalf("got %+v", got)
	}
}

func TestAuthFileCandidatesIncludesHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".commandcode", "auth.json")
	found := false
	for _, p := range AuthFileCandidates() {
		if p == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("candidates missing %s: %s", want, strings.Join(AuthFileCandidates(), ", "))
	}
}
