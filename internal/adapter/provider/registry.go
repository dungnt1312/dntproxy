package provider

import (
	"sort"
	"sync"

	"github.com/dungnt/dntproxy/internal/port"
)

// Registry is a thread-safe map-based provider registry.
// Adding a new provider = call RegisterExecutor("providerID", executor) at startup.
type Registry struct {
	mu        sync.RWMutex
	executors map[string]port.ProviderExecutor
}

// NewRegistry creates a new empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		executors: make(map[string]port.ProviderExecutor),
	}
}

// GetExecutor returns the executor for a given provider ID.
func (r *Registry) GetExecutor(provider string) port.ProviderExecutor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.executors[provider]
}

// RegisterExecutor registers an executor for a provider ID.
// Multiple aliases can point to the same executor.
func (r *Registry) RegisterExecutor(provider string, executor port.ProviderExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[provider] = executor
}

// SupportedProviders returns all registered provider IDs sorted alphabetically.
func (r *Registry) SupportedProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]string, 0, len(r.executors))
	for p := range r.executors {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	return providers
}
