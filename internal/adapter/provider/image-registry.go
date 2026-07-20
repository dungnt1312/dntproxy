package provider

import (
	"sort"
	"sync"

	"github.com/dungnt/dntproxy/internal/port"
)

// ImageRegistry is a thread-safe registry dedicated to image providers.
type ImageRegistry struct {
	mu        sync.RWMutex
	providers map[string]port.ImageProvider
}

func NewImageRegistry() *ImageRegistry {
	return &ImageRegistry{providers: make(map[string]port.ImageProvider)}
}

func (r *ImageRegistry) GetImageProvider(provider string) port.ImageProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[provider]
}

func (r *ImageRegistry) RegisterImageProvider(provider string, imageProvider port.ImageProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider] = imageProvider
}

func (r *ImageRegistry) SupportedImageProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.providers))
	for name := range r.providers {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
