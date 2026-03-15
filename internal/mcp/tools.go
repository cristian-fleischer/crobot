package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// toolDef pairs a tool definition with its handler name for dispatch.
type toolDef struct {
	name string
	tool mcp.Tool
}

// toolDefinitions returns the MCP tool definitions for all CRoBot commands.
func toolDefinitions() []toolDef {
	return []toolDef{
		{
			name: "export_pr_context",
			tool: mcp.NewTool("export_pr_context",
				mcp.WithDescription("Export PR context (metadata, changed files, diff hunks) as JSON."),
				mcp.WithString("workspace", mcp.Required(), mcp.Description("Workspace or organization slug")),
				mcp.WithString("repo", mcp.Required(), mcp.Description("Repository slug")),
				mcp.WithNumber("pr", mcp.Required(), mcp.Description("Pull request number")),
				mcp.WithReadOnlyHintAnnotation(true),
			),
		},
		{
			name: "get_file_snippet",
			tool: mcp.NewTool("get_file_snippet",
				mcp.WithDescription("Get a snippet of a file at a specific commit with surrounding context lines."),
				mcp.WithString("workspace", mcp.Required(), mcp.Description("Workspace or organization slug")),
				mcp.WithString("repo", mcp.Required(), mcp.Description("Repository slug")),
				mcp.WithString("commit", mcp.Required(), mcp.Description("Commit hash")),
				mcp.WithString("path", mcp.Required(), mcp.Description("File path")),
				mcp.WithNumber("line", mcp.Required(), mcp.Description("Center line number (1-based)")),
				mcp.WithNumber("context", mcp.Description("Number of context lines above and below (default 5)")),
				mcp.WithReadOnlyHintAnnotation(true),
			),
		},
		{
			name: "list_bot_comments",
			tool: mcp.NewTool("list_bot_comments",
				mcp.WithDescription("List existing bot comments on a pull request."),
				mcp.WithString("workspace", mcp.Required(), mcp.Description("Workspace or organization slug")),
				mcp.WithString("repo", mcp.Required(), mcp.Description("Repository slug")),
				mcp.WithNumber("pr", mcp.Required(), mcp.Description("Pull request number")),
				mcp.WithReadOnlyHintAnnotation(true),
			),
		},
		{
			name: "apply_review_findings",
			tool: mcp.NewTool("apply_review_findings",
				mcp.WithDescription("Apply review findings as inline PR comments. Dry-run by default; set dry_run=false to post."),
				mcp.WithString("workspace", mcp.Required(), mcp.Description("Workspace or organization slug")),
				mcp.WithString("repo", mcp.Required(), mcp.Description("Repository slug")),
				mcp.WithNumber("pr", mcp.Required(), mcp.Description("Pull request number")),
				mcp.WithArray("findings", mcp.Required(), mcp.Description("Array of ReviewFinding objects")),
				mcp.WithBoolean("dry_run", mcp.Description("If true (default), validate only without posting")),
				mcp.WithNumber("max_comments", mcp.Description("Maximum number of comments to post (0 = use config default)")),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(true),
			),
		},
	}
}
