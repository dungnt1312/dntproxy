package filters

import (
	"fmt"
	"strings"
)

// GitFilter compresses git status / diff / log output.
func GitFilter(s string) (string, bool) {
	if strings.Contains(s, "diff --git ") ||
		(strings.Contains(s, "\n@@ -") && strings.Contains(s, "\n+++ ")) {
		return gitDiffFilter(s)
	}
	if strings.Contains(s, "On branch ") {
		return gitStatusFilter(s)
	}
	return gitLogFilter(s)
}

func gitDiffFilter(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	var out []string
	pendingMinus := ""

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			out = append(out, line)
			pendingMinus = ""
		case strings.HasPrefix(line, "index ") && strings.Contains(line, ".."):
			// drop index lines
		case strings.HasPrefix(line, "--- "):
			pendingMinus = line
		case strings.HasPrefix(line, "+++ "):
			if pendingMinus != "" {
				out = append(out, pendingMinus+" "+line)
				pendingMinus = ""
			} else {
				out = append(out, line)
			}
		case strings.HasPrefix(line, "@@ "):
			// "@@ -10,7 +10,7 @@ ctx" → "@@ -10,7 +10,7 @@"
			parts := strings.SplitN(line, "@@ ", 3)
			if len(parts) >= 3 {
				out = append(out, "@@ "+strings.TrimSpace(parts[1])+" @@")
			} else {
				out = append(out, line)
			}
		case strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-"):
			out = append(out, line)
		case line == `\ No newline at end of file`:
			// drop
		default:
			// context line — drop
		}
	}

	if len(out) == 0 {
		return s, false
	}
	return strings.Join(out, "\n"), true
}

func gitStatusFilter(s string) (string, bool) {
	var out []string
	var modified, added, deleted, untracked []string
	inUntracked := false

	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "On branch "):
			out = append(out, line)
			inUntracked = false
		case strings.HasPrefix(t, "modified:"):
			modified = append(modified, strings.TrimSpace(t[len("modified:"):]))
		case strings.HasPrefix(t, "new file:"):
			added = append(added, strings.TrimSpace(t[len("new file:"):]))
		case strings.HasPrefix(t, "deleted:"):
			deleted = append(deleted, strings.TrimSpace(t[len("deleted:"):]))
		case strings.HasPrefix(t, "renamed:"):
			modified = append(modified, strings.TrimSpace(t[len("renamed:"):]))
		case strings.HasPrefix(t, "Untracked files:"):
			inUntracked = true
		case inUntracked && strings.HasPrefix(line, "\t") && t != "" && !strings.HasPrefix(t, "("):
			untracked = append(untracked, t)
		case t == "nothing to commit" || t == "nothing added to commit but untracked files present":
			out = append(out, t)
		}
	}

	if len(modified) > 0 {
		out = append(out, fmt.Sprintf("Modified: %s (%d files)", strings.Join(modified, ", "), len(modified)))
	}
	if len(added) > 0 {
		out = append(out, fmt.Sprintf("Added: %s (%d files)", strings.Join(added, ", "), len(added)))
	}
	if len(deleted) > 0 {
		out = append(out, fmt.Sprintf("Deleted: %s (%d files)", strings.Join(deleted, ", "), len(deleted)))
	}
	if len(untracked) > 3 {
		out = append(out, fmt.Sprintf("Untracked: %s ... (%d total)", strings.Join(untracked[:3], ", "), len(untracked)))
	} else if len(untracked) > 0 {
		out = append(out, fmt.Sprintf("Untracked: %s", strings.Join(untracked, ", ")))
	}

	if len(out) == 0 {
		return s, false
	}
	return strings.Join(out, "\n"), true
}

func gitLogFilter(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	var out []string
	sha, author, date := "", "", ""

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "commit ") && len(line) >= 47:
			sha = line[7:14]
		case strings.HasPrefix(line, "Author: "):
			a := strings.TrimPrefix(line, "Author: ")
			if idx := strings.Index(a, " <"); idx >= 0 {
				author = a[:idx]
			} else {
				author = a
			}
		case strings.HasPrefix(line, "Date:"):
			d := strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
			if len(d) > 16 {
				d = d[:16]
			}
			date = d
		case line == "" && sha != "":
			for i+1 < len(lines) {
				i++
				msg := strings.TrimSpace(lines[i])
				if msg != "" {
					if len(msg) > 80 {
						msg = msg[:80] + "..."
					}
					out = append(out, fmt.Sprintf("%s %s %s: %s", sha, author, date, msg))
					sha, author, date = "", "", ""
					break
				}
			}
		}
	}

	if len(out) == 0 {
		return s, false
	}
	return strings.Join(out, "\n"), true
}
