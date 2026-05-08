package filters

import (
	"strings"
	"testing"
)

func TestGitFilter_Diff(t *testing.T) {
	in := `diff --git a/foo.go b/foo.go
index abc1234..def5678 100644
--- a/foo.go
+++ b/foo.go
@@ -10,7 +10,7 @@ func Foo() {
 context line
-old line
+new line
 context line
`
	out, ok := GitFilter(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if strings.Contains(out, "context line") {
		t.Error("context lines should be stripped")
	}
	if strings.Contains(out, "index abc1234") {
		t.Error("index lines should be stripped")
	}
	if !strings.Contains(out, "diff --git") {
		t.Error("diff --git line must be kept")
	}
	if !strings.Contains(out, "-old line") {
		t.Error("minus line must be kept")
	}
	if !strings.Contains(out, "+new line") {
		t.Error("plus line must be kept")
	}
	if float64(len(out)) >= float64(len(in))*0.85 {
		t.Errorf("expected >15%% reduction, got orig=%d out=%d", len(in), len(out))
	}
}

func TestGitFilter_Status(t *testing.T) {
	in := `On branch main
Your branch is up to date with 'origin/main'.

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
	modified:   foo.go
	modified:   bar.go

no changes added to commit (use "git add" and/or "git commit -a")
`
	out, ok := GitFilter(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.Contains(out, "On branch main") {
		t.Error("branch line must be kept")
	}
	if !strings.Contains(out, "foo.go") {
		t.Error("modified file must appear in output")
	}
	if strings.Contains(out, `use "git`) {
		t.Error("hint lines should be stripped")
	}
	if float64(len(out)) >= float64(len(in))*0.85 {
		t.Errorf("expected >15%% reduction, got orig=%d out=%d", len(in), len(out))
	}
}

func TestGitFilter_FailOpen_Garbage(t *testing.T) {
	in := "not git output at all just random text"
	out, ok := GitFilter(in)
	// log sub-filter returns whatever it can; if nothing, ok=false
	_ = ok
	_ = out
	// Must not panic
}
