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

//go:embed philosophy.md
var defaultPhilosophy string

//go:embed multi_agent.md
var multiAgent string

//go:embed workflow_mcp.md
var mcpWorkflow string

//go:embed workflow_acp.md
var acpWorkflow string

//go:embed commands_cli.md
var cliCommands string

//go:embed workflow_cli.md
var cliWorkflow string

//go:embed skill.md
var defaultSkill string

// DefaultPhilosophy returns the built-in review philosophy (what to comment
// on, what to skip). This is the default used when no custom philosophy is
// provided.
func DefaultPhilosophy() string {
	return defaultPhilosophy
}

// DefaultSkill returns the built-in agent skill for CLI-based code reviews.
func DefaultSkill() string {
	return defaultSkill
}

// base builds the common prefix: core instructions + philosophy + multi-agent.
func base(philosophy string) string {
	phil := defaultPhilosophy
	if philosophy != "" {
		phil = philosophy
	}
	return coreInstructions + "\n" + phil + "\n" + multiAgent
}

// CoreInstructions returns the core review methodology, finding schema, and
// rules — without any workflow. Uses the built-in default philosophy.
func CoreInstructions() string {
	return base("")
}

// CoreInstructionsWithPhilosophy returns the core review methodology with a
// custom review philosophy replacing the built-in default.
func CoreInstructionsWithPhilosophy(philosophy string) string {
	return base(philosophy)
}

// ACPInstructions returns the review instructions for the ACP orchestrator.
// The ACP agent receives all PR data in the prompt and does not need to
// fetch anything.
func ACPInstructions() string {
	return base("") + "\n" + acpWorkflow
}

// ACPInstructionsWithPhilosophy returns ACP instructions with a custom
// review philosophy.
func ACPInstructionsWithPhilosophy(philosophy string) string {
	return base(philosophy) + "\n" + acpWorkflow
}

// MCPInstructions returns the review instructions for MCP-connected agents.
// MCP agents discover tools automatically, so this omits CLI command syntax
// and references tool names instead.
func MCPInstructions() string {
	return base("") + "\n" + mcpWorkflow
}

// MCPInstructionsWithPhilosophy returns MCP instructions with a custom
// review philosophy.
func MCPInstructionsWithPhilosophy(philosophy string) string {
	return base(philosophy) + "\n" + mcpWorkflow
}

// CLIInstructions returns the review instructions for CLI-based agents.
// This includes the full CLI command reference since CLI agents need to
// know exact command syntax.
func CLIInstructions() string {
	return base("") + "\n" + cliCommands + "\n" + cliWorkflow
}

// CLIInstructionsWithPhilosophy returns CLI instructions with a custom
// review philosophy.
func CLIInstructionsWithPhilosophy(philosophy string) string {
	return base(philosophy) + "\n" + cliCommands + "\n" + cliWorkflow
}
