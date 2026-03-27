package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/spf13/cobra"
)

// newPRCommentsCmd creates the list-pr-comments subcommand.
func newPRCommentsCmd() *cobra.Command {
	var (
		workspace  string
		repo       string
		pr         int
		unresolved bool
	)

	cmd := &cobra.Command{
		Use:   "list-pr-comments",
		Short: "List inline comments on a PR",
		Long: `Lists all inline review comments on the specified pull request as JSON to stdout.
Use --unresolved to filter to only unresolved (open) comments.

Note: GitHub's REST API does not expose per-comment resolution status;
on GitHub all comments will have is_resolved=false.`,
		Example: `  # List all inline comments
  crobot list-pr-comments --pr 42

  # List only unresolved comments
  crobot list-pr-comments --pr 42 --unresolved`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadDefault()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			workspace, repo = resolveWorkspaceRepo(workspace, repo, cfg)

			if workspace == "" || repo == "" || pr <= 0 {
				return fmt.Errorf("--workspace, --repo, and --pr are required")
			}

			slog.Debug("listing PR comments", "workspace", workspace, "repo", repo, "pr", pr, "unresolved", unresolved)

			plat, err := buildPlatform(cfg)
			if err != nil {
				return fmt.Errorf("creating platform client: %w", err)
			}

			ctx := cmd.Context()
			comments, err := plat.ListPRComments(ctx, platform.PRRequest{
				Workspace: workspace,
				Repo:      repo,
				PRNumber:  pr,
			})
			if err != nil {
				return fmt.Errorf("listing PR comments: %w", err)
			}

			if unresolved {
				filtered := comments[:0]
				for _, c := range comments {
					if !c.IsResolved {
						filtered = append(filtered, c)
					}
				}
				comments = filtered
			}

			data, err := json.MarshalIndent(comments, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling comments: %w", err)
			}

			return writeJSON(os.Stdout, data)
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace or organization slug")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository slug")
	cmd.Flags().IntVar(&pr, "pr", 0, "Pull request number")
	cmd.Flags().BoolVar(&unresolved, "unresolved", false, "Only show unresolved (open) comments")

	return cmd
}
