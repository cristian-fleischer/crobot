package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
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
	)

	cmd := &cobra.Command{
		Use:   "apply-review-findings",
		Short: "Apply review findings as PR inline comments",
		Long: `Takes ReviewFinding[] JSON and posts them as inline PR comments.

By default, runs in dry-run mode (validates and shows what would be posted).
Use --write to actually post comments.`,
		Example: `  # Dry run (default)
  crobot apply-review-findings \
    --workspace myteam --repo my-service --pr 42 \
    --input findings.json --dry-run

  # Write
  crobot apply-review-findings \
    --workspace myteam --repo my-service --pr 42 \
    --input findings.json --write

  # Read from stdin
  cat findings.json | crobot apply-review-findings \
    --workspace myteam --repo my-service --pr 42 \
    --input - --write`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" {
				return fmt.Errorf("--input is required (use - for stdin)")
			}
			if dryRun && write {
				return fmt.Errorf("--dry-run and --write are mutually exclusive")
			}

			// Dry-run is the default; --write overrides it. Explicitly
			// passing --dry-run is supported for clarity in scripts.
			isDryRun := !write

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

			workspace, repo = resolveWorkspaceRepo(workspace, repo, cfg)

			if workspace == "" || repo == "" || pr <= 0 {
				return fmt.Errorf("--workspace, --repo, and --pr are required")
			}

			slog.Debug("applying review findings",
				"workspace", workspace, "repo", repo, "pr", pr,
				"input", input, "dry_run", isDryRun,
			)

			plat, err := buildPlatform(cfg)
			if err != nil {
				return fmt.Errorf("creating platform client: %w", err)
			}

			// Determine max comments: CLI flag > config > default.
			// Only override from CLI if the flag was explicitly set to a
			// positive value; 0 means unlimited when set explicitly.
			mc := cfg.Review.MaxComments
			if cmd.Flags().Changed("max-comments") {
				mc = maxComments
			}

			engine := review.NewEngine(plat, review.EngineConfig{
				MaxComments:       mc,
				DryRun:            isDryRun,
				BotLabel:          cfg.Review.BotLabel,
				SeverityThreshold: cfg.Review.SeverityThreshold,
			})

			ctx := cmd.Context()
			result, err := engine.Run(ctx, platform.PRRequest{
				Workspace: workspace,
				Repo:      repo,
				PRNumber:  pr,
			}, findings)
			if err != nil {
				return fmt.Errorf("running review engine: %w", err)
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

	return cmd
}
