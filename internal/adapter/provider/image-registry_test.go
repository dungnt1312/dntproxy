package provider

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

type stubImageProvider struct{}

func (stubImageProvider) Capabilities(string) domain.ImageCapabilities {
	return domain.ImageCapabilities{Generate: true}
}

func TestImageRegistryConcurrentAccess(t *testing.T) {
	registry := NewImageRegistry()
	var group sync.WaitGroup
	for index := 0; index < 50; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			name := fmt.Sprintf("provider-%02d", index)
			registry.RegisterImageProvider(name, stubImageProvider{})
			if registry.GetImageProvider(name) == nil {
				t.Errorf("provider %q missing after registration", name)
			}
			_ = registry.SupportedImageProviders()
		}(index)
	}
	group.Wait()
	if got := len(registry.SupportedImageProviders()); got != 50 {
		t.Fatalf("providers = %d, want 50", got)
	}
}

func (stubImageProvider) Generate(context.Context, port.ImageRequest) ([]domain.ImageResult, int, error) {
	return nil, 200, nil
}

func (stubImageProvider) Edit(context.Context, port.ImageRequest) ([]domain.ImageResult, int, error) {
	return nil, 400, nil
}

func TestImageRegistry(t *testing.T) {
	registry := NewImageRegistry()
	registry.RegisterImageProvider("zeta", stubImageProvider{})
	registry.RegisterImageProvider("alpha", stubImageProvider{})

	if got := registry.GetImageProvider("alpha"); got == nil {
		t.Fatal("registered provider was not returned")
	}
	if got := registry.GetImageProvider("missing"); got != nil {
		t.Fatalf("missing provider = %#v, want nil", got)
	}
	got := registry.SupportedImageProviders()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Fatalf("providers = %#v, want sorted names", got)
	}
}
