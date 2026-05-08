package compressor

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	reGitDiffHead = regexp.MustCompile(`(?m)^diff --git `)
	reGitHunk     = regexp.MustCompile(`(?m)^@@ -`)
	reGitDiffPlus = regexp.MustCompile(`(?m)^\+\+\+ `)
	reGitCommit   = regexp.MustCompile(`(?m)^commit [a-f0-9]{40}`)
	reGoTest      = regexp.MustCompile(`(?m)(--- PASS:|--- FAIL:|=== RUN   |^FAIL\t)`)
	reCargo       = regexp.MustCompile(`test result: `)
	reTimestampDet = regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}`)
	reLSLineDet   = regexp.MustCompile(`(?m)^[d\-][rwx\-]{9}`)
	reTreeLineDet = regexp.MustCompile(`(?m)^[├└│]`)
	reBase64Line  = regexp.MustCompile(`(?m)^[A-Za-z0-9+/]{60,}={0,2}$`)
)

// Detect returns the ContentType of s using priority-ordered rules.
func Detect(s string) ContentType {
	// 1. CodeFile — never compress source code
	if isCodeFile(s) {
		return ContentCodeFile
	}
	// 2. GitDiff
	if reGitDiffHead.MatchString(s) || (reGitHunk.MatchString(s) && reGitDiffPlus.MatchString(s)) {
		return ContentGitDiff
	}
	// 3. GitStatus
	if strings.Contains(s, "On branch ") &&
		(strings.Contains(s, "Changes not staged") ||
			strings.Contains(s, "nothing to commit") ||
			strings.Contains(s, "Untracked files:")) {
		return ContentGitStatus
	}
	// 4. GitLog — two or more commit SHAs
	if len(reGitCommit.FindAllString(s, 3)) >= 2 {
		return ContentGitLog
	}
	// 5. GoTest
	if reGoTest.MatchString(s) {
		return ContentGoTest
	}
	// 6. CargoTest
	if strings.Contains(s, "running ") && reCargo.MatchString(s) && strings.Contains(s, " passed; ") {
		return ContentCargoTest
	}
	// 7. Pytest
	if strings.Contains(s, "=====") &&
		strings.Contains(strings.ToLower(s), "pytest") &&
		(strings.Contains(s, "passed") || strings.Contains(s, "failed") || strings.Contains(s, "error")) {
		return ContentPytest
	}
	// 8. LS / tree
	if len(reLSLineDet.FindAllString(s, 4)) >= 3 || len(reTreeLineDet.FindAllString(s, 6)) >= 5 {
		return ContentLS
	}
	// 9. Log — ≥10 lines, ≥30% have timestamp prefix
	if isLogContent(s) {
		return ContentLog
	}
	// 10. JSON
	trimmed := strings.TrimSpace(s)
	if (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(s)) {
		return ContentJSON
	}
	return ContentGeneric
}

// HasBase64Line reports whether s contains a base64-encoded line.
func HasBase64Line(s string) bool {
	return reBase64Line.MatchString(s)
}

func isCodeFile(s string) bool {
	keywords := []string{"package ", "import ", "func ", "class ", "def ", "const ", "type "}
	seen := make(map[string]bool, len(keywords))
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		for _, kw := range keywords {
			if !seen[kw] && strings.HasPrefix(t, kw) {
				seen[kw] = true
				if len(seen) >= 3 {
					return true
				}
				break
			}
		}
	}
	return false
}

func isLogContent(s string) bool {
	lines := strings.Split(s, "\n")
	if len(lines) < 10 {
		return false
	}
	matched := len(reTimestampDet.FindAllString(s, -1))
	return float64(matched)/float64(len(lines)) >= 0.30
}
