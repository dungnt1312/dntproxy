package commandcode

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	fallbackCLIVersion  = "1.27.1"
	versionCacheTTL     = 30 * time.Minute
	npmLatestURL        = "https://registry.npmjs.org/command-code/latest"
	versionFetchTimeout = 5 * time.Second
)

type versionCache struct {
	mu      sync.RWMutex
	version string
	fetched time.Time
}

var cliVersionCache versionCache

// CommandCodeVersion returns the current CLI version required by Command Code APIs.
func CommandCodeVersion() string {
	cliVersionCache.mu.RLock()
	if cliVersionCache.version != "" && time.Since(cliVersionCache.fetched) < versionCacheTTL {
		v := cliVersionCache.version
		cliVersionCache.mu.RUnlock()
		return v
	}
	cliVersionCache.mu.RUnlock()

	v, err := fetchCLIVersion()
	if err != nil || v == "" {
		cliVersionCache.mu.RLock()
		cached := cliVersionCache.version
		cliVersionCache.mu.RUnlock()
		if cached != "" {
			return cached
		}
		return fallbackCLIVersion
	}

	cliVersionCache.mu.Lock()
	cliVersionCache.version = v
	cliVersionCache.fetched = time.Now()
	cliVersionCache.mu.Unlock()
	return v
}

func fetchCLIVersion() (string, error) {
	client := &http.Client{Timeout: versionFetchTimeout}
	resp, err := client.Get(npmLatestURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("npm registry returned %d", resp.StatusCode)
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.Version, nil
}
