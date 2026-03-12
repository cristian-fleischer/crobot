package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/dizzyc/crobot/internal/config"
	"github.com/dizzyc/crobot/internal/platform"
	"github.com/spf13/cobra"
)

// newCommentsCmd creates the list-bot-comments subcommand.
func newCommentsCmd() *cobra.Command {
	var (
		workspace string
		repo      string
		pr        int
	)

	cmd := &cobra.Command{
		Use:     "list-bot-comments",
		Short:   "List existing bot comments on a PR",
		Long:    "Lists existing comments posted by this bot on the specified pull request as JSON to stdout.",
		Example: `  crobot list-bot-comments --workspace myteam --repo my-service --pr 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadDefault()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			workspace, repo = resolveWorkspaceRepo(workspace, repo, cfg)

			if workspace == "" || repo == "" || pr <= 0 {
				return fmt.Errorf("--workspace, --repo, and --pr are required")
			}

			slog.Debug("listing bot comments", "workspace", workspace, "repo", repo, "pr", pr)

			plat, err := buildPlatform(cfg)
			if err != nil {
				return fmt.Errorf("creating platform client: %w", err)
			}

			ctx := cmd.Context()
			comments, err := plat.ListBotComments(ctx, platform.PRRequest{
				Workspace: workspace,
				Repo:      repo,
				PRNumber:  pr,
			})
			if err != nil {
				return fmt.Errorf("listing bot comments: %w", err)
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

	return cmd
}
