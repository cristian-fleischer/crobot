package platform

import (
	"fmt"
	"sync"

	"github.com/cristian-fleischer/crobot/internal/config"
)

// Constructor is a function that creates a Platform from the application
// configuration. Each platform implementation extracts its own settings from
// the Config (e.g. bitbucket reads cfg.Bitbucket).
type Constructor func(cfg config.Config) (Platform, error)

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
func NewPlatform(name string, cfg config.Config) (Platform, error) {
	registryMu.RLock()
	ctor, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownPlatform, name)
	}
	return ctor(cfg)
}
