package tunnel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// TunnelState holds persistent tunnel state.
type TunnelState struct {
	ShortID   string `json:"shortId"`
	MachineID string `json:"machineId"`
	TunnelURL string `json:"tunnelUrl"`
	PID       int    `json:"pid"`
}

// StateManager handles tunnel state persistence.
type StateManager struct {
	stateDir string
}

// NewStateManager creates a state manager using ~/.dntproxy/tunnel/ or %APPDATA%/dntproxy/tunnel/.
func NewStateManager() (*StateManager, error) {
	dir := tunnelDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create tunnel state dir: %w", err)
	}
	return &StateManager{stateDir: dir}, nil
}

func tunnelDir() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "dntproxy", "tunnel")
		}
	}
	return filepath.Join(home, ".dntproxy", "tunnel")
}

func (s *StateManager) stateFile() string {
	return filepath.Join(s.stateDir, "state.json")
}

func (s *StateManager) pidFile() string {
	return filepath.Join(s.stateDir, "cloudflared.pid")
}

// LoadState reads persisted tunnel state.
func (s *StateManager) LoadState() (*TunnelState, error) {
	data, err := os.ReadFile(s.stateFile())
	if err != nil {
		if os.IsNotExist(err) {
			return &TunnelState{}, nil
		}
		return nil, err
	}
	var state TunnelState
	if err := json.Unmarshal(data, &state); err != nil {
		return &TunnelState{}, nil
	}
	return &state, nil
}

// SaveState persists tunnel state to disk.
func (s *StateManager) SaveState(state *TunnelState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return os.WriteFile(s.stateFile(), data, 0600)
}

// SavePID writes the cloudflared PID to disk.
func (s *StateManager) SavePID(pid int) error {
	return os.WriteFile(s.pidFile(), []byte(strconv.Itoa(pid)), 0644)
}

// ReadPID reads the stored PID. Returns 0 if not found.
func (s *StateManager) ReadPID() int {
	data, err := os.ReadFile(s.pidFile())
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

// ClearPID removes the PID file.
func (s *StateManager) ClearPID() {
	os.Remove(s.pidFile())
}

// BinDir returns the directory for downloaded binaries (~/.dntproxy/bin/).
func (s *StateManager) BinDir() string {
	base := filepath.Dir(s.stateDir)
	return filepath.Join(base, "bin")
}
