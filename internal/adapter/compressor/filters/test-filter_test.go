package filters

import (
	"strings"
	"testing"
)

func TestTestFilter_GoTest_KeepFails(t *testing.T) {
	in := `=== RUN   TestFoo
=== RUN   TestFoo/sub1
--- PASS: TestFoo/sub1 (0.00s)
--- PASS: TestFoo (0.01s)
=== RUN   TestBar
--- FAIL: TestBar (0.05s)
    bar_test.go:42: expected 1 got 2
FAIL	github.com/foo/bar	0.06s
`
	out, ok := TestFilter(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if strings.Contains(out, "=== RUN") {
		t.Error("RUN lines should be stripped")
	}
	if strings.Contains(out, "--- PASS:") {
		t.Error("PASS lines should be stripped")
	}
	if !strings.Contains(out, "--- FAIL: TestBar") {
		t.Error("FAIL line must be kept")
	}
	if !strings.Contains(out, "expected 1 got 2") {
		t.Error("fail detail must be kept")
	}
	if !strings.Contains(out, "FAIL\tgithub.com/foo/bar") {
		t.Error("summary FAIL line must be kept")
	}
}

func TestTestFilter_GoTest_AllPass(t *testing.T) {
	in := `=== RUN   TestFoo
--- PASS: TestFoo (0.00s)
ok  github.com/foo/bar  0.01s
`
	out, ok := TestFilter(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if strings.Contains(out, "=== RUN") {
		t.Error("RUN lines should be stripped in all-pass output")
	}
}

func TestTestFilter_Cargo(t *testing.T) {
	in := `running 5 tests
test test_add ... ok
test test_sub ... ok
test test_mul ... ok
test test_div ... FAILED
test test_rem ... ok

failures:
    test_div

test result: FAILED. 4 passed; 1 failed; 0 ignored
`
	out, ok := TestFilter(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.Contains(out, "running 5 tests") {
		t.Error("running line must be kept")
	}
	if !strings.Contains(out, "test result:") {
		t.Error("test result summary must be kept")
	}
}
