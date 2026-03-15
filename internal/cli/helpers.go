package cli

import (
	"fmt"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/platform/bitbucket"
)

// buildPlatform creates a Platform instance from the loaded config.
// It resolves the platform type and passes the appropriate platform-specific
// configuration to the constructor.
func buildPlatform(cfg config.Config) (platform.Platform, error) {
	switch cfg.Platform {
	case "bitbucket":
		return bitbucket.NewClient(&bitbucket.Config{
			Workspace: cfg.Bitbucket.Workspace,
			User:      cfg.Bitbucket.User,
			Token:     cfg.Bitbucket.Token,
		})
	default:
		return nil, fmt.Errorf("%w: %q", platform.ErrUnknownPlatform, cfg.Platform)
	}
}

// resolveWorkspaceRepo applies config-based defaults for workspace and repo.
// CLI flags take precedence; if empty, the corresponding config values are used.
func resolveWorkspaceRepo(workspace, repo string, cfg config.Config) (string, string) {
	if workspace == "" {
		workspace = cfg.Bitbucket.Workspace
	}
	if repo == "" {
		repo = cfg.Bitbucket.Repo
	}
	return workspace, repo
}
