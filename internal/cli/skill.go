package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cristian-fleischer/crobot/internal/prompt"
	"github.com/spf13/cobra"
)

// Supported agent targets and their skill install paths.
var agentTargets = map[string]struct {
	// dir returns the directory path for the skill relative to the base.
	dir  func(base string) string
	desc string
}{
	"claude-code": {
		dir:  func(base string) string { return filepath.Join(base, ".claude", "skills", "review-pr") },
		desc: "Claude Code (.claude/skills/review-pr/SKILL.md)",
	},
	"codex": {
		dir:  func(base string) string { return filepath.Join(base, ".codex", "skills", "review-pr") },
		desc: "Codex CLI (.codex/skills/review-pr/SKILL.md)",
	},
	"opencode": {
		dir:  func(base string) string { return filepath.Join(base, ".opencode", "skills", "review-pr") },
		desc: "OpenCode (.opencode/skills/review-pr/SKILL.md)",
	},
	"generic": {
		dir:  func(base string) string { return filepath.Join(base, ".agents", "skills") },
		desc: "Generic (.agents/skills/review-pr.md)",
	},
}

// newSkillCmd creates the export-skill subcommand.
func newSkillCmd() *cobra.Command {
	var (
		agent  string
		global bool
	)

	cmd := &cobra.Command{
		Use:   "export-skill",
		Short: "Export the CRoBot review skill for an AI agent",
		Long: `Exports the CRoBot code review skill to the correct location for your AI agent.

By default, installs to the current project directory. Use --global to install
to your home directory (available across all projects).

Supported agents: claude-code, codex, opencode, generic.
If no --agent is specified, prints the skill to stdout.`,
		Example: `  # Print skill to stdout
  crobot export-skill

  # Install for Claude Code in current project
  crobot export-skill --agent claude-code

  # Install for Claude Code globally
  crobot export-skill --agent claude-code --global

  # Install for Codex CLI in current project
  crobot export-skill --agent codex

  # Install for any agent (generic location)
  crobot export-skill --agent generic`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			content := prompt.DefaultSkill()

			if agent == "" {
				fmt.Print(content)
				return nil
			}

			target, ok := agentTargets[agent]
			if !ok {
				return fmt.Errorf("unknown agent %q; supported: claude-code, codex, opencode, generic", agent)
			}

			base := "."
			if global {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("resolving home directory: %w", err)
				}
				base = home
			}

			dir := target.dir(base)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", dir, err)
			}

			// Claude Code, Codex, and OpenCode use SKILL.md; generic uses the name directly.
			filename := "SKILL.md"
			if agent == "generic" {
				filename = "review-pr.md"
			}
			path := filepath.Join(dir, filename)

			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", path, err)
			}

			scope := "project"
			if global {
				scope = "global"
			}
			fmt.Fprintf(os.Stderr, "Installed CRoBot review skill for %s (%s): %s\n", agent, scope, path)
			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "Target agent: claude-code, codex, opencode, generic")
	cmd.Flags().BoolVar(&global, "global", false, "Install to home directory (available across all projects)")
	return cmd
}
