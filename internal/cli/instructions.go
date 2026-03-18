package cli

import (
	"fmt"
	"log/slog"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/prompt"
	"github.com/spf13/cobra"
)

func newInstructionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review-instructions",
		Short: "Print the review instructions for AI agents",
		Long:  "Outputs the CRoBot review methodology, finding schema, workflow, and rules that an AI agent should follow when performing a code review.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadDefault()
			if err != nil {
				slog.Debug("config load failed, using defaults", "error", err)
			}
			philosophy, _ := config.LoadPhilosophy(cfg)
			instructions := prompt.CLIInstructionsWithPhilosophy(philosophy)
			_, err = fmt.Fprintln(cmd.OutOrStdout(), instructions)
			return err
		},
	}

	return cmd
}
