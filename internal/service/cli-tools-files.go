package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isConfigWritable(path string) bool {
	if info, err := os.Stat(path); err == nil {
		return !info.IsDir() && info.Mode().Perm()&0200 != 0
	}
	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err == nil {
		return info.IsDir() && info.Mode().Perm()&0200 != 0
	}
	return false
}

func readConfigOrEmpty(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

func latestBackup(path string) string {
	matches, err := filepath.Glob(path + ".dntproxy-backup-*")
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[len(matches)-1]
}

func backupAndWrite(path string, content string, now time.Time) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}

	var backupPath string
	if fileExists(path) {
		backupPath = fmt.Sprintf("%s.dntproxy-backup-%s", path, now.Format("20060102-150405"))
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read existing config: %w", err)
		}
		if err := os.WriteFile(backupPath, data, 0600); err != nil {
			return "", fmt.Errorf("write backup: %w", err)
		}
	}

	tmp := path + ".dntproxy-tmp"
	if err := os.WriteFile(tmp, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("replace config: %w", err)
	}
	return backupPath, nil
}

func restoreBackup(path string, backupPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmp := path + ".dntproxy-restore-tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write restore temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("restore config: %w", err)
	}
	return nil
}

func normalizeBaseURL(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	return strings.TrimRight(endpoint, "/")
}
