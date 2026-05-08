package filters

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var reLSEntry = regexp.MustCompile(`^[d\-][rwx\-]{9}`)

// LsFilter compresses ls -l / tree / find output.
func LsFilter(s string) (string, bool) {
	if reLSEntry.MatchString(s) {
		return lsLongFilter(s)
	}
	if strings.ContainsAny(s, "├└│") {
		return treeFilter(s)
	}
	return findFilter(s)
}

func lsLongFilter(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	var header []string
	files, dirs := 0, 0

	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "total ") {
			header = append(header, line)
			continue
		}
		if reLSEntry.MatchString(line) {
			if strings.HasPrefix(line, "d") {
				dirs++
			} else {
				files++
			}
		}
	}

	if files+dirs < 10 {
		return s, false
	}

	out := append(header, fmt.Sprintf("(%d files, %d dirs)", files, dirs))
	return strings.Join(out, "\n"), true
}

func treeFilter(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	if len(lines) <= 30 {
		return s, false
	}

	const keepHead = 20
	const keepTail = 10
	if len(lines) <= keepHead+keepTail {
		return s, false
	}

	var out []string
	out = append(out, lines[:keepHead]...)
	out = append(out, fmt.Sprintf("... (%d lines omitted) ...", len(lines)-keepHead-keepTail))
	out = append(out, lines[len(lines)-keepTail:]...)
	return strings.Join(out, "\n"), true
}

func findFilter(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	if len(lines) <= 20 {
		return s, false
	}

	type dirEntry struct {
		dir   string
		count int
		first []string
	}
	dirMap := make(map[string]*dirEntry)
	var order []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(line))
		if _, exists := dirMap[dir]; !exists {
			dirMap[dir] = &dirEntry{dir: dir}
			order = append(order, dir)
		}
		e := dirMap[dir]
		e.count++
		if len(e.first) < 3 {
			e.first = append(e.first, line)
		}
	}

	var out []string
	for _, dir := range order {
		e := dirMap[dir]
		if e.count > 10 {
			out = append(out, fmt.Sprintf("%s/ (%d matches)", dir, e.count))
		} else {
			out = append(out, e.first...)
		}
	}

	if len(out) == 0 {
		return s, false
	}
	return strings.Join(out, "\n"), true
}
