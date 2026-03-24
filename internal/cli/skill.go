package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cristian-fleischer/crobot/internal/prompt"
	"github.com/spf13/cobra"
)

// newSkillCmd creates the export-skill subcommand.
func newSkillCmd() *cobra.Command {
	var (
		global bool
	)

	cmd := &cobra.Command{
		Use:   "export-skill",
		Short: "Install the CRoBot review skill for AI agents",
		Long: `Installs the CRoBot code review skill to the standard Agent Skills location
(.agents/skills/review-pr/SKILL.md), which is discovered automatically by all
agents that support the Agent Skills standard (Claude Code, Codex, Gemini CLI,
GitHub Copilot, Cursor, OpenCode, and 30+ others).

By default installs to the current project directory. Use --global to install
to your home directory (available across all projects).

If --print is specified, prints the skill content to stdout instead of
installing it.`,
		Example: `  # Install skill (current project)
  crobot export-skill

  # Install skill globally (all projects)
  crobot export-skill --global

  # Print skill to stdout (inspect or pipe)
  crobot export-skill --print`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			content := prompt.DefaultSkill()

			if printOnly, _ := cmd.Flags().GetBool("print"); printOnly {
				fmt.Print(content)
				return nil
			}

			base := "."
			if global {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("resolving home directory: %w", err)
				}
				base = home
			}

			dir := filepath.Join(base, ".agents", "skills", "review-pr")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", dir, err)
			}

			path := filepath.Join(dir, "SKILL.md")

			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", path, err)
			}

			scope := "project"
			if global {
				scope = "global"
			}
			fmt.Fprintf(os.Stderr, "Installed CRoBot review skill (%s): %s\n", scope, path)
			fmt.Fprintf(os.Stderr, "This skill is auto-discovered by all Agent Skills compatible agents.\n")
			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Install to home directory (available across all projects)")
	cmd.Flags().Bool("print", false, "Print skill to stdout instead of installing")
	return cmd
}
