package filters

import (
	"fmt"
	"regexp"
	"strings"
)

var reTimestampPrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}[.,\d]*(?:Z|[+\-]\d{2}:?\d{2})?\s*`)

// LogFilter deduplicates adjacent identical lines, strips timestamp prefixes,
// and truncates logs longer than 200 lines to first 50 + last 50.
func LogFilter(s string) (string, bool) {
	lines := strings.Split(s, "\n")

	// Step 1: strip timestamp prefixes
	stripped := make([]string, len(lines))
	for i, line := range lines {
		stripped[i] = reTimestampPrefix.ReplaceAllString(line, "")
	}

	// Step 2: dedup adjacent identical lines
	var deduped []string
	for i := 0; i < len(stripped); {
		line := stripped[i]
		count := 1
		for i+count < len(stripped) && stripped[i+count] == line {
			count++
		}
		if count > 1 && line != "" {
			deduped = append(deduped, fmt.Sprintf("%s ×%d", line, count))
		} else {
			deduped = append(deduped, line)
		}
		i += count
	}

	// Step 3: truncate if still > 200 lines
	if len(deduped) > 200 {
		const head = 50
		const tail = 50
		elided := len(deduped) - head - tail
		var truncated []string
		truncated = append(truncated, deduped[:head]...)
		truncated = append(truncated, fmt.Sprintf("... [%d lines elided] ...", elided))
		truncated = append(truncated, deduped[len(deduped)-tail:]...)
		deduped = truncated
	}

	return strings.Join(deduped, "\n"), true
}
