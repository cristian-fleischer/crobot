package cli

import (
	"fmt"

	"github.com/cristian-fleischer/crobot/internal/prompt"
	"github.com/spf13/cobra"
)

func newInstructionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review-instructions",
		Short: "Print the review instructions for AI agents",
		Long:  "Outputs the CRoBot review methodology, finding schema, workflow, and rules that an AI agent should follow when performing a code review.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), prompt.CLIInstructions())
			return err
		},
	}

	return cmd
}
