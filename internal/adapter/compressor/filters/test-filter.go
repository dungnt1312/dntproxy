package filters

import "strings"

// TestFilter compresses go test / cargo test / pytest output.
func TestFilter(s string) (string, bool) {
	if strings.Contains(s, "=== RUN") || strings.Contains(s, "--- PASS:") || strings.Contains(s, "--- FAIL:") {
		return goTestFilter(s)
	}
	if strings.Contains(s, "test result: ") {
		return cargoTestFilter(s)
	}
	return pytestFilter(s)
}

func goTestFilter(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	var out []string
	inFail := false

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "=== RUN"):
			inFail = false
			// drop
		case strings.HasPrefix(line, "    --- RUN"):
			// drop sub-test runs
		case strings.HasPrefix(line, "--- PASS:") || strings.HasPrefix(line, "    --- PASS:"):
			inFail = false
			// drop passing tests
		case strings.HasPrefix(line, "--- FAIL:") || strings.HasPrefix(line, "    --- FAIL:"):
			out = append(out, line)
			inFail = true
		case strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "FAIL\t") || strings.HasPrefix(line, "FAIL "):
			out = append(out, line)
			inFail = false
		case inFail && (strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")):
			out = append(out, line)
		case strings.HasPrefix(line, "PASS"):
			out = append(out, line)
			inFail = false
		}
	}

	if len(out) == 0 {
		return "All tests passed", true
	}
	return strings.Join(out, "\n"), true
}

func cargoTestFilter(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	var out []string

	for _, line := range lines {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "running ") && strings.Contains(t, " test"):
			out = append(out, line)
		case strings.HasPrefix(t, "test result:"):
			out = append(out, line)
		case strings.Contains(t, "FAILED"):
			out = append(out, line)
		case strings.HasPrefix(t, "test ") && strings.HasSuffix(t, "... ok"):
			// drop passing tests
		case strings.HasPrefix(t, "test ") && strings.Contains(t, "... FAILED"):
			out = append(out, line)
		default:
			if t != "" && !strings.HasPrefix(t, "test ") {
				out = append(out, line)
			}
		}
	}

	if len(out) == 0 {
		return s, false
	}
	return strings.Join(out, "\n"), true
}

func pytestFilter(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	var out []string

	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "FAILED ") || strings.HasPrefix(t, "ERROR ") ||
			strings.HasPrefix(t, "====") || strings.HasPrefix(t, "____") ||
			(strings.Contains(t, " failed") && strings.Contains(t, "=")) ||
			(strings.Contains(t, " passed") && strings.Contains(t, "=")) ||
			(strings.Contains(t, " error") && strings.Contains(t, "=")) {
			out = append(out, line)
		}
	}

	if len(out) == 0 {
		return s, false
	}
	return strings.Join(out, "\n"), true
}
