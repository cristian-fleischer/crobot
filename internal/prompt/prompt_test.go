package prompt

import (
	"strings"
	"testing"
)

func TestMCPInstructions_ContainsCoreContent(t *testing.T) {
	t.Parallel()

	instructions := MCPInstructions()

	required := []string{
		"CRoBot Review Instructions",
		"Review Philosophy",
		"Quality over quantity",
		"ReviewFinding Schema",
		"Severity",
		"Rules",
		"Workflow",
		"export_pr_context",
		"list_bot_comments",
		"apply_review_findings",
		"dry_run",
	}

	for _, s := range required {
		if !strings.Contains(instructions, s) {
			t.Errorf("MCP instructions missing required content: %q", s)
		}
	}
}

func TestMCPInstructions_OmitsCLICommands(t *testing.T) {
	t.Parallel()

	instructions := MCPInstructions()

	// MCP instructions should not contain CLI command syntax.
	cliOnly := []string{
		"crobot export-pr-context",
		"crobot apply-review-findings",
		"--input findings.json",
	}

	for _, s := range cliOnly {
		if strings.Contains(instructions, s) {
			t.Errorf("MCP instructions should not contain CLI syntax: %q", s)
		}
	}
}

func TestCLIInstructions_ContainsCoreContent(t *testing.T) {
	t.Parallel()

	instructions := CLIInstructions()

	required := []string{
		"CRoBot Review Instructions",
		"Review Philosophy",
		"Quality over quantity",
		"ReviewFinding Schema",
		"Severity",
		"Rules",
		"Workflow",
	}

	for _, s := range required {
		if !strings.Contains(instructions, s) {
			t.Errorf("CLI instructions missing required content: %q", s)
		}
	}
}

func TestCLIInstructions_ContainsCLICommands(t *testing.T) {
	t.Parallel()

	instructions := CLIInstructions()

	cliContent := []string{
		"CLI Commands",
		"crobot export-pr-context",
		"crobot get-file-snippet",
		"crobot list-bot-comments",
		"crobot apply-review-findings",
		"--dry-run",
		"--write",
		"--input",
	}

	for _, s := range cliContent {
		if !strings.Contains(instructions, s) {
			t.Errorf("CLI instructions missing CLI content: %q", s)
		}
	}
}

func TestMCPInstructions_NotEmpty(t *testing.T) {
	t.Parallel()

	if len(MCPInstructions()) == 0 {
		t.Fatal("MCP instructions are empty")
	}
}

func TestCLIInstructions_NotEmpty(t *testing.T) {
	t.Parallel()

	if len(CLIInstructions()) == 0 {
		t.Fatal("CLI instructions are empty")
	}
}

func TestCLIInstructions_LongerThanMCP(t *testing.T) {
	t.Parallel()

	mcp := MCPInstructions()
	cli := CLIInstructions()

	if len(cli) <= len(mcp) {
		t.Errorf("CLI instructions (%d bytes) should be longer than MCP (%d bytes) due to command reference",
			len(cli), len(mcp))
	}
}
