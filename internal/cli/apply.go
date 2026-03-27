package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
	localplatform "github.com/cristian-fleischer/crobot/internal/platform/local"
	"github.com/cristian-fleischer/crobot/internal/review"
	"github.com/spf13/cobra"
)

// newApplyCmd creates the apply-review-findings subcommand.
func newApplyCmd() *cobra.Command {
	var (
		workspace   string
		repo        string
		pr          int
		input       string
		dryRun      bool
		write       bool
		maxComments int
		threshold   string
		local       bool
		uncommitted bool
		base        string
	)

	cmd := &cobra.Command{
		Use:   "apply-review-findings",
		Short: "Apply review findings as PR comments or render locally",
		Long: `Takes ReviewFinding[] JSON and posts them as inline PR comments.

By default, runs in dry-run mode (validates and shows what would be posted).
Use --write to actually post comments.

Use --local to validate and render findings in the terminal without a PR.
Local mode always runs as dry-run and renders findings with diff context.`,
		Example: `  # Dry run (default)
  crobot apply-review-findings --pr 42 --input findings.json --dry-run

  # Write
  crobot apply-review-findings --pr 42 --input findings.json --write

  # Local mode (render to terminal)
  crobot apply-review-findings --local --input findings.json

  # Read from stdin
  cat findings.json | crobot apply-review-findings --pr 42 --input - --write`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" {
				return fmt.Errorf("--input is required (use - for stdin)")
			}
			if local && write {
				return fmt.Errorf("--local and --write are mutually exclusive (local mode is always dry-run)")
			}
			if !local && dryRun && write {
				return fmt.Errorf("--dry-run and --write are mutually exclusive")
			}

			isDryRun := !write
			if local {
				isDryRun = true
			}

			// Read findings from file or stdin.
			const maxInputSize = 10 << 20 // 10 MB
			var findingsData []byte
			var err error
			if input == "-" {
				findingsData, err = io.ReadAll(io.LimitReader(os.Stdin, maxInputSize))
			} else {
				f, openErr := os.Open(input)
				if openErr != nil {
					return fmt.Errorf("reading input: %w", openErr)
				}
				defer f.Close()
				findingsData, err = io.ReadAll(io.LimitReader(f, maxInputSize))
			}
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}

			findings, err := platform.ParseFindings(findingsData)
			if err != nil {
				return fmt.Errorf("parsing findings: %w", err)
			}

			cfg, err := config.LoadDefault()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			var plat platform.Platform
			var req platform.PRRequest

			if local {
				var localPlat *localplatform.Provider
				if uncommitted {
					localPlat = localplatform.NewUncommitted(".")
				} else {
					localPlat = localplatform.New(base, ".")
				}
				plat = localPlat
				req = platform.PRRequest{
					Workspace: "local",
					Repo:      localPlat.RepoName(),
					PRNumber:  0,
				}
			} else {
				workspace, repo = resolveWorkspaceRepo(workspace, repo, cfg)

				if workspace == "" || repo == "" || pr <= 0 {
					return fmt.Errorf("--workspace, --repo, and --pr are required (or use --local)")
				}

				slog.Debug("applying review findings",
					"workspace", workspace, "repo", repo, "pr", pr,
					"input", input, "dry_run", isDryRun,
				)

				plat, err = buildPlatform(cfg)
				if err != nil {
					return fmt.Errorf("creating platform client: %w", err)
				}

				req = platform.PRRequest{
					Workspace: workspace,
					Repo:      repo,
					PRNumber:  pr,
				}
			}

			// Determine max comments: CLI flag > config > default.
			mc := cfg.Review.MaxComments
			if cmd.Flags().Changed("max-comments") {
				mc = maxComments
			}

			// Determine severity threshold: CLI flag > config.
			st := cfg.Review.SeverityThreshold
			if cmd.Flags().Changed("threshold") {
				st = threshold
			}

			engine := review.NewEngine(plat, review.EngineConfig{
				MaxComments:       mc,
				DryRun:            isDryRun,
				BotLabel:          cfg.Review.BotLabel,
				SeverityThreshold: st,
			})

			ctx := cmd.Context()
			result, err := engine.Run(ctx, req, findings)
			if err != nil {
				return fmt.Errorf("running review engine: %w", err)
			}

			// In local mode, render findings to stderr with diff context.
			if local && len(result.Posted) > 0 {
				prCtx, _ := plat.GetPRContext(ctx, req)
				var hunks []platform.DiffHunk
				if prCtx != nil {
					hunks = prCtx.DiffHunks
				}
				RenderFindings(result.Posted, hunks, os.Stderr, false)
			}

			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling result: %w", err)
			}

			return writeJSON(os.Stdout, data)
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace or organization slug")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository slug")
	cmd.Flags().IntVar(&pr, "pr", 0, "Pull request number")
	cmd.Flags().StringVar(&input, "input", "", "Path to findings JSON file (use - for stdin)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and show what would be posted without posting")
	cmd.Flags().BoolVar(&write, "write", false, "Actually post comments to the PR")
	cmd.Flags().IntVar(&maxComments, "max-comments", 0, "Maximum number of comments to post (0 = unlimited; omit to use config default)")
	cmd.Flags().StringVar(&threshold, "threshold", "", "Minimum severity threshold: info, warning, error (omit to use config default)")
	cmd.Flags().BoolVar(&local, "local", false, "Validate and render findings locally (no PR needed)")
	cmd.Flags().BoolVar(&uncommitted, "uncommitted", false, "Only diff uncommitted changes against HEAD (used with --local)")
	cmd.Flags().StringVar(&base, "base", "master", "Base branch for local mode (used with --local)")

	return cmd
}
