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

// newExportCmd creates the export-pr-context subcommand.
func newExportCmd() *cobra.Command {
	var (
		workspace string
		repo      string
		pr        int
	)

	cmd := &cobra.Command{
		Use:     "export-pr-context",
		Short:   "Export PR context as JSON",
		Long:    "Fetches and outputs the full PR context (metadata, changed files, diff hunks) as JSON to stdout.",
		Example: `  crobot export-pr-context --workspace myteam --repo my-service --pr 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadDefault()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			workspace, repo = resolveWorkspaceRepo(workspace, repo, cfg)

			if workspace == "" || repo == "" || pr <= 0 {
				return fmt.Errorf("--workspace, --repo, and --pr are required")
			}

			slog.Debug("exporting PR context", "workspace", workspace, "repo", repo, "pr", pr)

			plat, err := buildPlatform(cfg)
			if err != nil {
				return fmt.Errorf("creating platform client: %w", err)
			}

			ctx := cmd.Context()
			prCtx, err := plat.GetPRContext(ctx, platform.PRRequest{
				Workspace: workspace,
				Repo:      repo,
				PRNumber:  pr,
			})
			if err != nil {
				return fmt.Errorf("fetching PR context: %w", err)
			}

			data, err := json.MarshalIndent(prCtx, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling PR context: %w", err)
			}

			return writeJSON(os.Stdout, data)
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace or organization slug")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository slug")
	cmd.Flags().IntVar(&pr, "pr", 0, "Pull request number")

	return cmd
}
