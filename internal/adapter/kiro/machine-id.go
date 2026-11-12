package kiro

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const undeterminedMachineID = "UNDETERMINED_MACHINE_ID"

var (
	machineIDOnce sync.Once
	machineIDVal  string
)

// MachineID mirrors node-machine-id (sha256 hex of the OS machine GUID), which
// Kiro embeds in its User-Agent (KiroIDE-<version>-<machineId>).
// KIRO_MACHINE_ID overrides it.
func MachineID() string {
	machineIDOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv("KIRO_MACHINE_ID")); v != "" {
			machineIDVal = v
			return
		}
		raw, err := readRawMachineID()
		if err != nil || raw == "" {
			machineIDVal = undeterminedMachineID
			return
		}
		sum := sha256.Sum256([]byte(raw))
		machineIDVal = hex.EncodeToString(sum[:])
	})
	return machineIDVal
}

func readRawMachineID() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	switch runtime.GOOS {
	case "windows":
		out, err := exec.CommandContext(ctx, "REG.exe", "QUERY",
			`HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid").Output()
		if err != nil {
			return "", err
		}
		parts := strings.SplitN(string(out), "REG_SZ", 2)
		if len(parts) < 2 {
			return "", errors.New("MachineGuid not found")
		}
		return normalizeMachineID(parts[1]), nil
	case "darwin":
		out, err := exec.CommandContext(ctx, "ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
		if err != nil {
			return "", err
		}
		parts := strings.SplitN(string(out), "IOPlatformUUID", 2)
		if len(parts) < 2 {
			return "", errors.New("IOPlatformUUID not found")
		}
		line := strings.SplitN(parts[1], "\n", 2)[0]
		line = strings.NewReplacer("=", "", `"`, "").Replace(line)
		return normalizeMachineID(line), nil
	default:
		for _, p := range []string{"/var/lib/dbus/machine-id", "/etc/machine-id"} {
			if b, err := os.ReadFile(p); err == nil && strings.TrimSpace(string(b)) != "" {
				return normalizeMachineID(string(b)), nil
			}
		}
		host, err := os.Hostname()
		return normalizeMachineID(host), err
	}
}

func normalizeMachineID(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), ""))
}
