package platform

import (
	"fmt"
	"sync"
)

// Constructor is a function that creates a Platform from an opaque
// configuration value. Each platform implementation defines its own expected
// config type (e.g. bitbucket expects *bitbucket.Config); the factory caller
// is responsible for passing the correct type.
type Constructor func(cfg any) (Platform, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Constructor{}
)

// Register adds a platform constructor under the given name. It is intended to
// be called from init() functions inside platform implementation packages
// (e.g. internal/platform/bitbucket). Calling Register with a name that is
// already registered will overwrite the previous constructor.
func Register(name string, ctor Constructor) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = ctor
}

// NewPlatform creates a Platform implementation by name. The cfg value is
// forwarded to the constructor registered for that name. Returns
// ErrUnknownPlatform (wrapped) if no constructor is registered for the given
// name.
func NewPlatform(name string, cfg any) (Platform, error) {
	registryMu.RLock()
	ctor, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownPlatform, name)
	}
	return ctor(cfg)
}
