package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/cristian-fleischer/crobot/internal/agent"
	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/spf13/cobra"
)

// newModelsCmd creates the models subcommand.
func newModelsCmd() *cobra.Command {
	var (
		agentName    string
		agentCommand string
	)

	cmd := &cobra.Command{
		Use:   "models",
		Short: "List available models from an ACP agent",
		Long:  `Starts an ACP agent, queries its available models, and prints them.`,
		Example: `  # List models from a configured agent
  crobot models --agent claude

  # List models from an agent command
  crobot models --agent-command "gemini --experimental-acp"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Load config.
			cfg, err := config.LoadDefault()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			// 2. Resolve agent config.
			agentCfg, err := resolveAgentConfig(cfg, agentName, agentCommand)
			if err != nil {
				return fmt.Errorf("resolving agent config: %w", err)
			}

			// 3. Start agent.
			clientCfg := agent.ClientConfig{
				Command: agentCfg.Command,
				Args:    agentCfg.Args,
				Timeout: agentCfg.Timeout,
			}
			client := agent.NewClient(clientCfg)

			ctx, cancel := context.WithTimeout(cmd.Context(), agentCfg.Timeout)
			defer cancel()

			if err := client.Start(ctx); err != nil {
				return fmt.Errorf("starting agent: %w", err)
			}
			defer client.Close()

			// 4. Create session to get model info.
			session := agent.NewSession(agent.SessionConfig{
				Client: client,
			})
			if err := session.Initialize(ctx); err != nil {
				return fmt.Errorf("initializing agent: %w", err)
			}
			defer session.Close(ctx)

			if err := session.CreateSession(ctx); err != nil {
				return fmt.Errorf("creating agent session: %w", err)
			}

			// 5. Print models.
			fmt.Fprintf(os.Stdout, "Agent: %s\n", agentCfg.Name)
			fmt.Fprintf(os.Stdout, "Current model: %s\n", session.CurrentModel)

			if len(session.AvailableModels) == 0 {
				fmt.Fprintln(os.Stdout, "\nNo model information reported by agent.")
				return nil
			}

			fmt.Fprintln(os.Stdout, "\nAvailable models:")
			for _, m := range session.AvailableModels {
				marker := "  "
				if m.ID == session.CurrentModel {
					marker = "> "
				}
				desc := m.Name
				if m.Description != "" && m.Description != m.Name {
					desc += " - " + m.Description
				}
				fmt.Fprintf(os.Stdout, "  %s%-28s  %s\n", marker, m.ID, desc)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&agentName, "agent", "", "ACP agent name (from config)")
	cmd.Flags().StringVar(&agentCommand, "agent-command", "", "ACP agent binary to run directly (bypasses config)")

	return cmd
}
