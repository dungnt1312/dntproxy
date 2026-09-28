package autologin

import (
	"fmt"
	"strings"
)

// account is one credential line: email|password|totp_secret (TOTP optional).
type account struct {
	Email      string
	Password   string
	TOTPSecret string
}

// ParseAccountLines parses pasted account lines. The separator is detected
// per line: "|" wins, then tab, then comma — passwords containing commas
// stay intact as long as the paste uses "|" or tabs.
// Returns parsed accounts and human-readable problems for rejected lines.
func ParseAccountLines(lines []string) ([]account, []string) {
	var accounts []account
	var problems []string
	seen := make(map[string]bool)

	for i, raw := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := splitAccountLine(line)
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			problems = append(problems, fmt.Sprintf("line %d: expected email|password|2fa_secret", i+1))
			continue
		}
		email := strings.TrimSpace(parts[0])
		if !strings.Contains(email, "@") {
			problems = append(problems, fmt.Sprintf("line %d: %q is not an email", i+1, email))
			continue
		}
		key := strings.ToLower(email)
		if seen[key] {
			continue // duplicate within one run — first entry wins
		}
		seen[key] = true
		totp := ""
		if len(parts) > 2 {
			totp = strings.TrimSpace(parts[2])
		}
		accounts = append(accounts, account{
			Email:      email,
			Password:   strings.TrimSpace(parts[1]),
			TOTPSecret: totp,
		})
	}
	return accounts, problems
}

func splitAccountLine(line string) []string {
	switch {
	case strings.Contains(line, "|"):
		return strings.SplitN(line, "|", 3)
	case strings.Contains(line, "\t"):
		return strings.SplitN(line, "\t", 3)
	default:
		return strings.SplitN(line, ",", 3)
	}
}
