package auth

import "github.com/dungnt/dntproxy/internal/adapter/kiro"

// kiroMachineID is the node-machine-id hash Kiro embeds in its User-Agent.
// The implementation lives in the kiro package so chat requests use the same id.
func kiroMachineID() string { return kiro.MachineID() }
