package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cristian-fleischer/crobot/internal/prompt"
	"github.com/spf13/cobra"
)

// newPhilosophyCmd creates the export-philosophy subcommand.
func newPhilosophyCmd() *cobra.Command {
	var (
		global bool
		local  bool
	)

	cmd := &cobra.Command{
		Use:   "export-philosophy",
		Short: "Export the default review philosophy for customization",
		Long: `Exports the built-in review philosophy to a file for customization.

By default, prints to stdout. Use --global or --local to write directly to
the standard override location:

  --global   ~/.config/crobot/review-philosophy.md
  --local    .crobot-philosophy.md (in current directory)

Once exported, edit the file to tailor the review focus to your project's
needs. CRoBot automatically picks up overrides from these locations.

Resolution order: built-in default < global file < local file < config
philosophy_path < CROBOT_REVIEW_PHILOSOPHY env var < --review-philosophy flag.`,
		Example: `  # Print to stdout
  crobot export-philosophy

  # Write to global config dir
  crobot export-philosophy --global

  # Write to repo root
  crobot export-philosophy --local

  # Pipe to a custom path
  crobot export-philosophy > ~/my-team-philosophy.md`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if global && local {
				return fmt.Errorf("--global and --local are mutually exclusive")
			}

			content := prompt.DefaultPhilosophy()

			if !global && !local {
				fmt.Print(content)
				return nil
			}

			var path string
			if global {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("resolving home directory: %w", err)
				}
				dir := filepath.Join(home, ".config", "crobot")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("creating config directory: %w", err)
				}
				path = filepath.Join(dir, "review-philosophy.md")
			} else {
				path = ".crobot-philosophy.md"
			}

			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", path, err)
			}
			fmt.Fprintf(os.Stderr, "Wrote review philosophy to %s\n", path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Write to ~/.config/crobot/review-philosophy.md")
	cmd.Flags().BoolVar(&local, "local", false, "Write to .crobot-philosophy.md in current directory")
	return cmd
}
