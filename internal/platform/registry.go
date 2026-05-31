package platform

import (
	"fmt"
	"sort"
	"sync"
)

// Registry manages the set of registered platform adapters.
// It is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	platforms map[string]Platform
}

// NewRegistry creates an empty platform registry.
func NewRegistry() *Registry {
	return &Registry{
		platforms: make(map[string]Platform),
	}
}

// Register adds a platform adapter to the registry.
// It panics if a platform with the same name is already registered.
func (r *Registry) Register(p Platform) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.platforms[name]; exists {
		panic(fmt.Sprintf("platform already registered: %s", name))
	}
	r.platforms[name] = p
}

// Get returns the platform adapter for the given name.
func (r *Registry) Get(name string) (Platform, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.platforms[name]
	if !ok {
		return nil, fmt.Errorf("platform not found: %s", name)
	}
	return p, nil
}

// MustGet returns the platform adapter or panics.
func (r *Registry) MustGet(name string) Platform {
	p, err := r.Get(name)
	if err != nil {
		panic(err)
	}
	return p
}

// List returns the names of all registered platforms, sorted alphabetically.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.platforms))
	for name := range r.platforms {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Has reports whether a platform with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.platforms[name]
	return ok
}

// Count returns the number of registered platforms.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.platforms)
}
