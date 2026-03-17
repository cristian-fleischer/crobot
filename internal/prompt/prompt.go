// Package prompt provides the review instructions that CRoBot delivers to
// AI agents. These instructions are the single source of truth for review
// methodology, finding schema, and workflow — used by both the MCP server
// (via WithInstructions) and the CLI (via the review-instructions command).
//
// The instruction content lives in .md files alongside this Go file for
// readability and ease of editing. They are embedded into the binary at
// build time via go:embed.
package prompt

import _ "embed"

//go:embed core.md
var coreInstructions string

//go:embed workflow_mcp.md
var mcpWorkflow string

//go:embed commands_cli.md
var cliCommands string

//go:embed workflow_cli.md
var cliWorkflow string

// CoreInstructions returns the core review methodology, finding schema, and
// rules — without any workflow. Used by the ACP orchestrator which provides
// its own self-contained workflow.
func CoreInstructions() string {
	return coreInstructions
}

// MCPInstructions returns the review instructions for MCP-connected agents.
// MCP agents discover tools automatically, so this omits CLI command syntax
// and references tool names instead.
func MCPInstructions() string {
	return coreInstructions + "\n" + mcpWorkflow
}

// CLIInstructions returns the review instructions for CLI-based agents.
// This includes the full CLI command reference since CLI agents need to
// know exact command syntax.
func CLIInstructions() string {
	return coreInstructions + "\n" + cliCommands + "\n" + cliWorkflow
}
