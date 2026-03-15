// Package cli implements the cobra CLI commands for crobot.
package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via ldflags.
var version = "dev"

// RootCmd returns the root cobra command for crobot.
func RootCmd() *cobra.Command {
	var verbose bool
	var outputFormat string

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
	cmd.PersistentFlags().StringVar(&outputFormat, "output-format", "json", "Output format (json)")
	cmd.Version = version

	// Register subcommands.
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newSnippetCmd())
	cmd.AddCommand(newCommentsCmd())
	cmd.AddCommand(newApplyCmd())
	cmd.AddCommand(newServeCmd())

	return cmd
}

// writeJSON writes JSON data to the given writer, appending a newline.
func writeJSON(w io.Writer, data []byte) error {
	_, err := fmt.Fprintf(w, "%s\n", data)
	return err
}
