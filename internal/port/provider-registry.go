package port

// ProviderRegistry manages provider executors.
// Adding a new provider = register it once at startup, all routing is automatic.
type ProviderRegistry interface {
	// GetExecutor returns the executor for a given provider ID.
	// Returns nil if provider is not registered.
	GetExecutor(provider string) ProviderExecutor

	// RegisterExecutor registers an executor for a provider ID.
	RegisterExecutor(provider string, executor ProviderExecutor)

	// SupportedProviders returns all registered provider IDs.
	SupportedProviders() []string
}
