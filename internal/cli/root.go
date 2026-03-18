// Package cli implements the cobra CLI commands for crobot.
package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/cristian-fleischer/crobot/internal/version"
	"github.com/spf13/cobra"
)

// RootCmd returns the root cobra command for crobot.
func RootCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "crobot",
		Short: "CRoBot - AI-powered code review bot",
		Long:  "CRoBot is a local-first CLI tool that enables AI-powered automated code reviews on pull requests.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level := slog.LevelInfo
			if verbose {
				level = slog.LevelDebug
			}
			handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			})
			slog.SetDefault(slog.New(handler))
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose/debug logging")
	cmd.Version = version.Version

	// Register subcommands.
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newSnippetCmd())
	cmd.AddCommand(newCommentsCmd())
	cmd.AddCommand(newApplyCmd())
	cmd.AddCommand(newServeCmd())
	cmd.AddCommand(newInstructionsCmd())
	cmd.AddCommand(newReviewCmd())
	cmd.AddCommand(newModelsCmd())
	cmd.AddCommand(newPhilosophyCmd())
	cmd.AddCommand(newSkillCmd())

	return cmd
}

// writeJSON writes JSON data to the given writer, appending a newline.
func writeJSON(w io.Writer, data []byte) error {
	_, err := fmt.Fprintf(w, "%s\n", data)
	return err
}
