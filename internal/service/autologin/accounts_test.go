package autologin

import (
	"strings"
	"testing"
)

func TestParseAccountLines(t *testing.T) {
	accounts, problems := ParseAccountLines([]string{
		"# comment",
		"",
		"a@gmail.com|pass1|JBSWY3DPEHPK3PXP",
		"b@gmail.com|pass2",
		"c@gmail.com\tpass3",
		"d@gmail.com,pass4",
		"e@gmail.com|com,ma|SECRET",
	})
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if len(accounts) != 5 {
		t.Fatalf("got %d accounts, want 5: %+v", len(accounts), accounts)
	}
	if accounts[0].Email != "a@gmail.com" || accounts[0].Password != "pass1" || accounts[0].TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("account 0 wrong: %+v", accounts[0])
	}
	if accounts[1].TOTPSecret != "" {
		t.Errorf("account 1 should have no TOTP: %+v", accounts[1])
	}
	if accounts[2].Password != "pass3" || accounts[3].Password != "pass4" {
		t.Errorf("tab/comma parsing wrong: %+v %+v", accounts[2], accounts[3])
	}
	// password containing a comma survives when separator is |
	if accounts[4].Password != "com,ma" {
		t.Errorf("comma-in-password broken: %+v", accounts[4])
	}
}

func TestParseAccountLinesRejects(t *testing.T) {
	_, problems := ParseAccountLines([]string{
		"not-an-email|pass",
		"x@gmail.com",
		"|password",
	})
	if len(problems) != 3 {
		t.Fatalf("got %d problems, want 3: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "not an email") {
		t.Errorf("problem 0 unexpected: %s", problems[0])
	}
}

func TestParseAccountLinesDedupesCaseInsensitive(t *testing.T) {
	accounts, _ := ParseAccountLines([]string{
		"User@Gmail.com|pass1",
		"user@gmail.com|pass2",
	})
	if len(accounts) != 1 || accounts[0].Password != "pass1" {
		t.Fatalf("expected first entry to win: %+v", accounts)
	}
}

func TestExtractCode(t *testing.T) {
	const state = "abc123"
	u := "http://localhost:1455/auth/callback?code=XYZ&state=" + state
	if got := extractCode(u, state); got != "XYZ" {
		t.Errorf("got %q, want XYZ", got)
	}
	// state mismatch must not yield a code
	if got := extractCode(u, "other"); got != "" {
		t.Errorf("state mismatch should return empty, got %q", got)
	}
	if got := extractCode("::::", state); got != "" {
		t.Errorf("unparseable URL should return empty, got %q", got)
	}
}
