package compressor

import (
	"os"
	"path/filepath"
	"testing"
)

func mustReadTestdata(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return string(b)
}

func TestDetect(t *testing.T) {
	cases := []struct {
		file string
		want ContentType
	}{
		{"git-diff.txt", ContentGitDiff},
		{"git-status.txt", ContentGitStatus},
		{"go-test-output.txt", ContentGoTest},
		{"log-app.txt", ContentLog},
		{"api-response.json", ContentJSON},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			s := mustReadTestdata(t, tc.file)
			got := Detect(s)
			if got != tc.want {
				t.Errorf("Detect(%s) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}

func TestDetect_GenericFallback(t *testing.T) {
	got := Detect("hello world this is just plain prose text without any special structure")
	if got != ContentGeneric {
		t.Fatalf("got %v, want ContentGeneric", got)
	}
}

func TestDetect_CodeFilePriority(t *testing.T) {
	src := `package main
import "fmt"
func main() {
	fmt.Println("hello")
}`
	got := Detect(src)
	if got != ContentCodeFile {
		t.Fatalf("got %v, want ContentCodeFile", got)
	}
}

func TestDetect_GitDiffInline(t *testing.T) {
	s := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1,3 +1,3 @@\n-old\n+new\n"
	if got := Detect(s); got != ContentGitDiff {
		t.Fatalf("got %v, want ContentGitDiff", got)
	}
}

func TestDetect_GitStatusInline(t *testing.T) {
	s := "On branch main\nChanges not staged for commit:\n\tmodified:   foo.go\n"
	if got := Detect(s); got != ContentGitStatus {
		t.Fatalf("got %v, want ContentGitStatus", got)
	}
}

func TestDetect_GoTestInline(t *testing.T) {
	s := "=== RUN   TestFoo\n--- PASS: TestFoo (0.00s)\nok  github.com/foo/bar  0.01s\n"
	if got := Detect(s); got != ContentGoTest {
		t.Fatalf("got %v, want ContentGoTest", got)
	}
}

func TestHasBase64Line(t *testing.T) {
	b64 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if !HasBase64Line(b64) {
		t.Fatal("expected base64 detection")
	}
	if HasBase64Line("normal text without base64") {
		t.Fatal("false positive on normal text")
	}
}
