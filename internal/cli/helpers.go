package cli

import (
	"fmt"
	"strings"

	"github.com/cristian-fleischer/crobot/internal/agent"
	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"

	// Register platform implementations via their init() functions.
	_ "github.com/cristian-fleischer/crobot/internal/platform/bitbucket"
	_ "github.com/cristian-fleischer/crobot/internal/platform/github"
)

// buildPlatform creates a Platform instance from the loaded config.
// It delegates to the platform factory, which dispatches to the constructor
// registered for cfg.Platform.
func buildPlatform(cfg config.Config) (platform.Platform, error) {
	return platform.NewPlatform(cfg.Platform, cfg)
}

// resolveWorkspaceRepo applies config-based defaults for workspace and repo.
// CLI flags take precedence; if empty, the corresponding config values are
// used based on the configured platform.
func resolveWorkspaceRepo(workspace, repo string, cfg config.Config) (string, string) {
	switch cfg.Platform {
	case "github":
		if workspace == "" {
			workspace = cfg.GitHub.Owner
		}
		if repo == "" {
			repo = cfg.GitHub.Repo
		}
	default: // "bitbucket" and others
		if workspace == "" {
			workspace = cfg.Bitbucket.Workspace
		}
		if repo == "" {
			repo = cfg.Bitbucket.Repo
		}
	}
	return workspace, repo
}

// resolveAgentConfig resolves the agent run configuration from CLI flags and
// the application config. If agentCommand is non-empty, it is parsed as a
// command line; otherwise the named (or default) agent from config is used.
func resolveAgentConfig(cfg config.Config, agentName, agentCommand string) (*agent.RunConfig, error) {
	if agentCommand != "" {
		parts := strings.Fields(agentCommand)
		if len(parts) == 0 {
			return nil, fmt.Errorf("--agent-command must not be empty")
		}
		return &agent.RunConfig{
			Name:    parts[0],
			Command: parts[0],
			Args:    parts[1:],
			Timeout: agent.DefaultTimeout,
		}, nil
	}
	return agent.ResolveAgentConfig(cfg, agentName)
}
