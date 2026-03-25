package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
	localplatform "github.com/cristian-fleischer/crobot/internal/platform/local"
	"github.com/spf13/cobra"
)

// exportResponse wraps the PR context with diff directory information,
// matching the MCP export response format.
type cliExportResponse struct {
	*platform.PRContext
	DiffDir   string              `json:"diff_dir"`
	DiffStats platform.DiffStats  `json:"diff_stats"`
}

// newExportCmd creates the export-pr-context subcommand.
func newExportCmd() *cobra.Command {
	var (
		workspace string
		repo      string
		pr        int
		local     bool
		base      string
	)

	cmd := &cobra.Command{
		Use:   "export-pr-context",
		Short: "Export PR or local context as JSON",
		Long: `Fetches and outputs the full context (metadata, changed files, diff hunks) as JSON
to stdout. Per-file diffs are written to .crobot/diffs-<run-id>/ and the response
includes a diff_dir field pointing to the directory.

Use --local to review local git changes instead of a PR.`,
		Example: `  # Export PR context
  crobot export-pr-context --pr 42

  # Export local changes against master
  crobot export-pr-context --local

  # Export local changes against a different base branch
  crobot export-pr-context --local --base main`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadDefault()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			var plat platform.Platform

			if local {
				plat = localplatform.New(base, ".")
			} else {
				workspace, repo = resolveWorkspaceRepo(workspace, repo, cfg)

				if workspace == "" || repo == "" || pr <= 0 {
					return fmt.Errorf("--workspace, --repo, and --pr are required (or use --local)")
				}

				slog.Debug("exporting PR context", "workspace", workspace, "repo", repo, "pr", pr)

				plat, err = buildPlatform(cfg)
				if err != nil {
					return fmt.Errorf("creating platform client: %w", err)
				}
			}

			ctx := cmd.Context()
			prCtx, err := plat.GetPRContext(ctx, platform.PRRequest{
				Workspace: workspace,
				Repo:      repo,
				PRNumber:  pr,
			})
			if err != nil {
				return fmt.Errorf("fetching context: %w", err)
			}

			// Write per-file diffs to disk for agent consumption.
			if err := platform.CleanupStaleDiffDirs(".crobot"); err != nil {
				slog.Warn("failed to clean stale diff dirs", "error", err)
			}
			stats := platform.ComputeDiffStats(prCtx.DiffHunks)
			diffDir := platform.NewDiffDir(".crobot")
			if err := platform.WriteDiffFiles(prCtx.DiffHunks, stats, diffDir); err != nil {
				return fmt.Errorf("writing diff files: %w", err)
			}

			// Return context without inline hunks; agents read from diff_dir.
			prCtx.DiffHunks = nil
			resp := cliExportResponse{
				PRContext: prCtx,
				DiffDir:   diffDir,
				DiffStats: stats,
			}

			data, err := json.MarshalIndent(resp, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling context: %w", err)
			}

			return writeJSON(os.Stdout, data)
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace or organization slug")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository slug")
	cmd.Flags().IntVar(&pr, "pr", 0, "Pull request number")
	cmd.Flags().BoolVar(&local, "local", false, "Export local git changes instead of a PR")
	cmd.Flags().StringVar(&base, "base", "master", "Base branch for local mode (used with --local)")

	return cmd
}
