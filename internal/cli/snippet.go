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

// newSnippetCmd creates the get-file-snippet subcommand.
func newSnippetCmd() *cobra.Command {
	var (
		workspace   string
		repo        string
		commit      string
		path        string
		line        int
		contextSize int
	)

	cmd := &cobra.Command{
		Use:   "get-file-snippet",
		Short: "Get a snippet of a file at a specific commit",
		Long:  "Returns a slice of a file at a given commit with surrounding context lines as JSON to stdout.",
		Example: `  crobot get-file-snippet \
    --workspace myteam --repo my-service \
    --commit abc123 --path src/auth.ts --line 42 --context 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadDefault()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			workspace, repo = resolveWorkspaceRepo(workspace, repo, cfg)

			if workspace == "" || repo == "" || commit == "" || path == "" || line <= 0 {
				return fmt.Errorf("--workspace, --repo, --commit, --path, and --line are required")
			}
			if contextSize < 0 {
				return fmt.Errorf("--context must be non-negative")
			}

			slog.Debug("getting file snippet",
				"workspace", workspace, "repo", repo,
				"commit", commit, "path", path,
				"line", line, "context", contextSize,
			)

			plat, err := buildPlatform(cfg)
			if err != nil {
				return fmt.Errorf("creating platform client: %w", err)
			}

			ctx := cmd.Context()
			content, err := plat.GetFileContent(ctx, platform.FileRequest{
				Workspace: workspace,
				Repo:      repo,
				Commit:    commit,
				Path:      path,
			})
			if err != nil {
				return fmt.Errorf("fetching file content: %w", err)
			}

			out, err := platform.ExtractSnippet(content, path, commit, line, contextSize)
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling snippet: %w", err)
			}

			return writeJSON(os.Stdout, data)
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace or organization slug")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository slug")
	cmd.Flags().StringVar(&commit, "commit", "", "Commit hash")
	cmd.Flags().StringVar(&path, "path", "", "File path")
	cmd.Flags().IntVar(&line, "line", 0, "Center line number")
	cmd.Flags().IntVar(&contextSize, "context", 5, "Number of context lines above and below")

	return cmd
}
