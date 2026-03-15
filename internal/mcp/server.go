// Package mcp implements the MCP (Model Context Protocol) server for CRoBot.
//
// The server exposes CRoBot's CLI commands as MCP tools over stdio, allowing
// MCP-capable agents to discover and invoke them automatically. It is a thin
// adapter over the existing platform and review engine packages.
package mcp

import (
	"context"
	"log/slog"
	"os"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/prompt"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server and manages its lifecycle.
type Server struct {
	stdioServer *server.StdioServer
}

// NewServer creates a new MCP server that exposes CRoBot tools.
// The server uses the given platform for API calls and config for review settings.
func NewServer(plat platform.Platform, cfg config.Config) (*Server, error) {
	mcpSrv := server.NewMCPServer(
		"crobot",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithInstructions(prompt.MCPInstructions()),
	)

	// Register tools.
	h := newHandler(plat, cfg)
	defs := toolDefinitions()
	for _, td := range defs {
		mcpSrv.AddTool(td.tool, h.dispatch(td.name))
	}

	slog.Info("MCP server initialized", "tools", len(defs))

	return &Server{
		stdioServer: server.NewStdioServer(mcpSrv),
	}, nil
}

// Serve starts the MCP server on stdio (stdin/stdout). It blocks until the
// context is cancelled or the transport is closed. Logging goes to stderr
// via slog so it does not interfere with the MCP protocol on stdout.
func (s *Server) Serve(ctx context.Context) error {
	slog.Info("starting MCP server on stdio")
	return s.stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}
