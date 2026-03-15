package cli

import (
	"fmt"
	"log/slog"

	"github.com/cristian-fleischer/crobot/internal/config"
	mcpserver "github.com/cristian-fleischer/crobot/internal/mcp"
	"github.com/spf13/cobra"
)

// newServeCmd creates the serve subcommand, which starts CRoBot as a server.
func newServeCmd() *cobra.Command {
	var mcpMode bool

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start CRoBot as a server",
		Long: `Starts CRoBot as a server exposing its commands as tools.

Use --mcp to start as an MCP (Model Context Protocol) server over stdio.`,
		Example: `  # Start as MCP server (stdio transport)
  crobot serve --mcp`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !mcpMode {
				return fmt.Errorf("specify a server mode (e.g., --mcp)")
			}

			cfg, err := config.LoadDefault()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			slog.Debug("starting MCP server", "platform", cfg.Platform)

			plat, err := buildPlatform(cfg)
			if err != nil {
				return fmt.Errorf("creating platform client: %w", err)
			}

			srv, err := mcpserver.NewServer(plat, cfg)
			if err != nil {
				return fmt.Errorf("creating MCP server: %w", err)
			}

			return srv.Serve(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&mcpMode, "mcp", false, "Start as MCP server over stdio")

	return cmd
}
