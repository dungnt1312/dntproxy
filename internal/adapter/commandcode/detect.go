package commandcode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AuthFile is the Command Code CLI credential file (~/.commandcode/auth.json).
type AuthFile struct {
	APIKey   string `json:"apiKey"`
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	KeyName  string `json:"keyName"`
	Source   string `json:"-"`
}

func (a AuthFile) DisplayName() string {
	if name := strings.TrimSpace(a.UserName); name != "" {
		return name
	}
	if name := strings.TrimSpace(a.KeyName); name != "" {
		return name
	}
	return "Command Code"
}

// AuthFileCandidates returns likely auth.json locations on this machine.
func AuthFileCandidates() []string {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".commandcode", "auth.json"))
	}
	for _, env := range []string{"COMMANDCODE_HOME", "COMMAND_CODE_HOME"} {
		if dir := strings.TrimSpace(os.Getenv(env)); dir != "" {
			paths = append(paths, filepath.Join(dir, "auth.json"))
		}
	}
	return uniquePaths(paths)
}

func ParseAuthFile(data []byte) (AuthFile, error) {
	var auth AuthFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return AuthFile{}, fmt.Errorf("parse auth.json: %w", err)
	}
	auth.APIKey = strings.TrimSpace(auth.APIKey)
	auth.UserName = strings.TrimSpace(auth.UserName)
	auth.KeyName = strings.TrimSpace(auth.KeyName)
	if auth.APIKey == "" {
		return AuthFile{}, fmt.Errorf("auth.json is missing apiKey")
	}
	return auth, nil
}

func FindAuthFile(candidates []string) (AuthFile, error) {
	if len(candidates) == 0 {
		candidates = AuthFileCandidates()
	}
	var lastErr error
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		auth, err := ParseAuthFile(data)
		if err != nil {
			lastErr = err
			continue
		}
		auth.Source = path
		return auth, nil
	}
	if lastErr != nil {
		return AuthFile{}, fmt.Errorf("no Command Code auth.json found (last error: %w); searched: %s", lastErr, strings.Join(candidates, ", "))
	}
	return AuthFile{}, fmt.Errorf("no Command Code auth.json found; searched: %s", strings.Join(candidates, ", "))
}

func uniquePaths(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}
