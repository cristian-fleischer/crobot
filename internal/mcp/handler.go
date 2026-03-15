package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/review"
	"github.com/mark3labs/mcp-go/mcp"
)

// handler routes MCP tool calls to the appropriate platform/review-engine
// methods. It holds the shared dependencies needed by all tool handlers.
type handler struct {
	platform platform.Platform
	config   config.Config
}

// newHandler creates a handler with the given dependencies.
func newHandler(plat platform.Platform, cfg config.Config) *handler {
	return &handler{
		platform: plat,
		config:   cfg,
	}
}

// dispatch returns a ToolHandlerFunc that routes to the correct handler by name.
func (h *handler) dispatch(name string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Debug("tool call", "tool", name)
		switch name {
		case "export_pr_context":
			return h.handleExportPRContext(ctx, req)
		case "get_file_snippet":
			return h.handleGetFileSnippet(ctx, req)
		case "list_bot_comments":
			return h.handleListBotComments(ctx, req)
		case "apply_review_findings":
			return h.handleApplyReviewFindings(ctx, req)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("unknown tool: %s", name)), nil
		}
	}
}

// handleExportPRContext fetches and returns the full PR context as JSON.
func (h *handler) handleExportPRContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workspace := mcp.ParseString(req, "workspace", "")
	repo := mcp.ParseString(req, "repo", "")
	pr := mcp.ParseInt(req, "pr", 0)

	if workspace == "" || repo == "" || pr <= 0 {
		return mcp.NewToolResultError("workspace, repo, and pr are required"), nil
	}

	prCtx, err := h.platform.GetPRContext(ctx, platform.PRRequest{
		Workspace: workspace,
		Repo:      repo,
		PRNumber:  pr,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("fetching PR context: %v", err)), nil
	}

	data, err := json.MarshalIndent(prCtx, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling PR context: %w", err)
	}

	return mcp.NewToolResultText(string(data)), nil
}

// handleGetFileSnippet fetches a file and returns a snippet around the specified line.
func (h *handler) handleGetFileSnippet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workspace := mcp.ParseString(req, "workspace", "")
	repo := mcp.ParseString(req, "repo", "")
	commit := mcp.ParseString(req, "commit", "")
	path := mcp.ParseString(req, "path", "")
	line := mcp.ParseInt(req, "line", 0)
	contextSize := mcp.ParseInt(req, "context", 5)

	if workspace == "" || repo == "" || commit == "" || path == "" || line <= 0 {
		return mcp.NewToolResultError("workspace, repo, commit, path, and line are required"), nil
	}
	if contextSize < 0 {
		return mcp.NewToolResultError("context must be non-negative"), nil
	}

	content, err := h.platform.GetFileContent(ctx, platform.FileRequest{
		Workspace: workspace,
		Repo:      repo,
		Commit:    commit,
		Path:      path,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("fetching file content: %v", err)), nil
	}

	out, err := platform.ExtractSnippet(content, path, commit, line, contextSize)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling snippet: %w", err)
	}

	return mcp.NewToolResultText(string(data)), nil
}

// handleListBotComments fetches and returns existing bot comments on a PR.
func (h *handler) handleListBotComments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workspace := mcp.ParseString(req, "workspace", "")
	repo := mcp.ParseString(req, "repo", "")
	pr := mcp.ParseInt(req, "pr", 0)

	if workspace == "" || repo == "" || pr <= 0 {
		return mcp.NewToolResultError("workspace, repo, and pr are required"), nil
	}

	comments, err := h.platform.ListBotComments(ctx, platform.PRRequest{
		Workspace: workspace,
		Repo:      repo,
		PRNumber:  pr,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing bot comments: %v", err)), nil
	}

	data, err := json.MarshalIndent(comments, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling comments: %w", err)
	}

	return mcp.NewToolResultText(string(data)), nil
}

// handleApplyReviewFindings validates, deduplicates, and optionally posts review findings.
func (h *handler) handleApplyReviewFindings(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workspace := mcp.ParseString(req, "workspace", "")
	repo := mcp.ParseString(req, "repo", "")
	pr := mcp.ParseInt(req, "pr", 0)
	dryRun := mcp.ParseBoolean(req, "dry_run", true)
	maxComments := mcp.ParseInt(req, "max_comments", 0)

	if workspace == "" || repo == "" || pr <= 0 {
		return mcp.NewToolResultError("workspace, repo, and pr are required"), nil
	}

	// Extract findings from the request arguments.
	findingsRaw := mcp.ParseArgument(req, "findings", nil)
	if findingsRaw == nil {
		return mcp.NewToolResultError("findings is required"), nil
	}

	// Round-trip through JSON: mcp.ParseArgument returns []any with map values,
	// and there is no way to type-assert directly to []platform.ReviewFinding.
	findingsJSON, err := json.Marshal(findingsRaw)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling findings: %v", err)), nil
	}

	findings, err := platform.ParseFindings(findingsJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("parsing findings: %v", err)), nil
	}

	// Determine max comments: tool arg > config default.
	mc := h.config.Review.MaxComments
	if maxComments > 0 {
		mc = maxComments
	}

	engine := review.NewEngine(h.platform, review.EngineConfig{
		MaxComments:       mc,
		DryRun:            dryRun,
		BotLabel:          h.config.Review.BotLabel,
		SeverityThreshold: h.config.Review.SeverityThreshold,
	})

	result, err := engine.Run(ctx, platform.PRRequest{
		Workspace: workspace,
		Repo:      repo,
		PRNumber:  pr,
	}, findings)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("running review engine: %v", err)), nil
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling result: %w", err)
	}

	return mcp.NewToolResultText(string(data)), nil
}
