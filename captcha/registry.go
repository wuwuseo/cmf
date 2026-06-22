package captcha

import (
	"fmt"
	"sort"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(provider Provider) error {
	if provider == nil || provider.Name() == "" {
		return fmt.Errorf("captcha provider name is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[provider.Name()]; exists {
		return fmt.Errorf("captcha provider %q is already registered", provider.Name())
	}
	r.providers[provider.Name()] = provider
	return nil
}

func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[name]
	return provider, ok
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
