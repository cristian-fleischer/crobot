package agent

import (
	"fmt"
	"time"

	"github.com/cristian-fleischer/crobot/internal/config"
)

// DefaultTimeout is the default agent execution timeout.
const DefaultTimeout = 10 * time.Minute

// RunConfig holds the resolved configuration for running an ACP agent.
type RunConfig struct {
	Name    string
	Command string
	Args    []string
	Timeout time.Duration
}

// ResolveAgentConfig resolves the agent configuration from the app config.
// If agentName is empty, uses the default agent from config.
func ResolveAgentConfig(cfg config.Config, agentName string) (*RunConfig, error) {
	name := agentName
	if name == "" {
		name = cfg.Agent.Default
	}
	if name == "" {
		return nil, fmt.Errorf("no agent specified and no default agent configured")
	}

	def, ok := cfg.Agent.Agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not found in configuration", name)
	}

	timeout := DefaultTimeout
	if cfg.Agent.Timeout > 0 {
		timeout = time.Duration(cfg.Agent.Timeout) * time.Second
	}

	return &RunConfig{
		Name:    name,
		Command: def.Command,
		Args:    def.Args,
		Timeout: timeout,
	}, nil
}
