package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/mark3labs/mcp-go/mcp"
)

// resultText extracts the text content from a CallToolResult.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("first content item is not TextContent: %T", result.Content[0])
	}
	return tc.Text
}

// mockPlatform implements platform.Platform for testing.
type mockPlatform struct {
	prContext      *platform.PRContext
	prContextErr   error
	fileContent    []byte
	fileContentErr error
	botComments    []platform.Comment
	botCommentsErr error
	createComment  *platform.Comment
	createErr      error
	deleteErr      error
}

func (m *mockPlatform) GetPRContext(_ context.Context, _ platform.PRRequest) (*platform.PRContext, error) {
	return m.prContext, m.prContextErr
}

func (m *mockPlatform) GetFileContent(_ context.Context, _ platform.FileRequest) ([]byte, error) {
	return m.fileContent, m.fileContentErr
}

func (m *mockPlatform) ListBotComments(_ context.Context, _ platform.PRRequest) ([]platform.Comment, error) {
	return m.botComments, m.botCommentsErr
}

func (m *mockPlatform) CreateInlineComment(_ context.Context, _ platform.PRRequest, _ platform.InlineComment) (*platform.Comment, error) {
	return m.createComment, m.createErr
}

func (m *mockPlatform) DeleteComment(_ context.Context, _ platform.PRRequest, _ string) error {
	return m.deleteErr
}

func newTestHandler(plat platform.Platform) *handler {
	return newHandler(plat, config.Defaults())
}

// makeRequest builds a CallToolRequest with the given arguments.
func makeRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func TestHandleExportPRContext_Success(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{
		prContext: &platform.PRContext{
			ID:           42,
			Title:        "Test PR",
			SourceBranch: "feature",
			TargetBranch: "main",
			HeadCommit:   "abc123",
			Files: []platform.ChangedFile{
				{Path: "main.go", Status: "modified"},
			},
			DiffHunks: []platform.DiffHunk{
				{Path: "main.go", NewStart: 1, NewLines: 5, Body: "+code"},
			},
		},
	}

	h := newTestHandler(mock)
	result, err := h.handleExportPRContext(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"pr":        float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Test PR") {
		t.Errorf("result does not contain PR title: %s", text)
	}
	if !strings.Contains(text, "abc123") {
		t.Errorf("result does not contain head commit: %s", text)
	}

	// Verify the output is valid JSON.
	var prCtx platform.PRContext
	if err := json.Unmarshal([]byte(text), &prCtx); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if prCtx.ID != 42 {
		t.Errorf("PR ID = %d, want 42", prCtx.ID)
	}
}

func TestHandleExportPRContext_MissingArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{name: "no args", args: map[string]any{}},
		{name: "missing repo", args: map[string]any{"workspace": "ws", "pr": float64(1)}},
		{name: "missing pr", args: map[string]any{"workspace": "ws", "repo": "rp"}},
		{name: "zero pr", args: map[string]any{"workspace": "ws", "repo": "rp", "pr": float64(0)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newTestHandler(&mockPlatform{})
			result, err := h.handleExportPRContext(t.Context(), makeRequest(tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Error("expected error result for missing args")
			}
		})
	}
}

func TestHandleExportPRContext_PlatformError(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{
		prContextErr: fmt.Errorf("API rate limited"),
	}

	h := newTestHandler(mock)
	result, err := h.handleExportPRContext(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"pr":        float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for platform failure")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "failed to fetch PR context") {
		t.Errorf("error result should contain sanitized message: %s", text)
	}
}

func TestHandleGetFileSnippet_Success(t *testing.T) {
	t.Parallel()

	fileContent := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	mock := &mockPlatform{fileContent: []byte(fileContent)}

	h := newTestHandler(mock)
	result, err := h.handleGetFileSnippet(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"commit":    "abc123",
		"path":      "main.go",
		"line":      float64(5),
		"context":   float64(2),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result)
	}

	text := resultText(t, result)

	var out platform.SnippetOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if out.StartLine != 3 {
		t.Errorf("StartLine = %d, want 3", out.StartLine)
	}
	if out.EndLine != 7 {
		t.Errorf("EndLine = %d, want 7", out.EndLine)
	}
	if !strings.Contains(out.Content, "line5") {
		t.Errorf("snippet does not contain target line: %q", out.Content)
	}
	if !strings.Contains(out.Content, "line3") {
		t.Errorf("snippet missing context line 3: %q", out.Content)
	}
}

func TestHandleGetFileSnippet_DefaultContext(t *testing.T) {
	t.Parallel()

	fileContent := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	mock := &mockPlatform{fileContent: []byte(fileContent)}

	h := newTestHandler(mock)
	// Don't provide context — should default to 5.
	result, err := h.handleGetFileSnippet(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"commit":    "abc123",
		"path":      "main.go",
		"line":      float64(5),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error")
	}

	text := resultText(t, result)
	var out platform.SnippetOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	// With default context=5, line=5: start=1, end=10.
	if out.StartLine != 1 {
		t.Errorf("StartLine = %d, want 1", out.StartLine)
	}
	if out.EndLine != 10 {
		t.Errorf("EndLine = %d, want 10", out.EndLine)
	}
}

func TestHandleGetFileSnippet_MissingArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{name: "no args", args: map[string]any{}},
		{name: "missing commit", args: map[string]any{"workspace": "ws", "repo": "rp", "path": "f.go", "line": float64(1)}},
		{name: "missing line", args: map[string]any{"workspace": "ws", "repo": "rp", "commit": "abc", "path": "f.go"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newTestHandler(&mockPlatform{})
			result, err := h.handleGetFileSnippet(t.Context(), makeRequest(tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Error("expected error result for missing args")
			}
		})
	}
}

func TestHandleGetFileSnippet_LineOutOfRange(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{fileContent: []byte("line1\nline2\nline3\n")}

	h := newTestHandler(mock)
	result, err := h.handleGetFileSnippet(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"commit":    "abc123",
		"path":      "main.go",
		"line":      float64(100),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for out-of-range line")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "failed to extract snippet") {
		t.Errorf("error should contain sanitized message: %s", text)
	}
}

func TestHandleListBotComments_Success(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{
		botComments: []platform.Comment{
			{ID: "1", Path: "main.go", Line: 10, Body: "Fix this", Fingerprint: "fp1"},
			{ID: "2", Path: "util.go", Line: 20, Body: "Improve", Fingerprint: "fp2"},
		},
	}

	h := newTestHandler(mock)
	result, err := h.handleListBotComments(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"pr":        float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error")
	}

	text := resultText(t, result)

	var comments []platform.Comment
	if err := json.Unmarshal([]byte(text), &comments); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("got %d comments, want 2", len(comments))
	}
	if comments[0].Fingerprint != "fp1" {
		t.Errorf("comments[0].Fingerprint = %q, want %q", comments[0].Fingerprint, "fp1")
	}
}

func TestHandleListBotComments_EmptyResult(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{botComments: []platform.Comment{}}

	h := newTestHandler(mock)
	result, err := h.handleListBotComments(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"pr":        float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error")
	}

	text := resultText(t, result)
	var comments []platform.Comment
	if err := json.Unmarshal([]byte(text), &comments); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("got %d comments, want 0", len(comments))
	}
}

func TestHandleApplyReviewFindings_DryRun(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{
		prContext: &platform.PRContext{
			ID: 42,
			Files: []platform.ChangedFile{
				{Path: "main.go", Status: "modified"},
			},
			DiffHunks: []platform.DiffHunk{
				{Path: "main.go", NewStart: 1, NewLines: 10, Body: "+code"},
			},
		},
		botComments: []platform.Comment{},
	}

	h := newHandler(mock, config.Config{
		Review: config.ReviewConfig{
			MaxComments:       25,
			BotLabel:          "crobot",
			SeverityThreshold: "warning",
		},
	})

	findings := []any{
		map[string]any{
			"path":     "main.go",
			"line":     float64(5),
			"side":     "new",
			"severity": "warning",
			"category": "style",
			"message":  "Consider refactoring",
		},
	}

	result, err := h.handleApplyReviewFindings(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"pr":        float64(42),
		"findings":  findings,
		"dry_run":   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := resultText(t, result)
		t.Fatalf("expected success, got error: %s", text)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "dry-run") {
		t.Errorf("dry-run result should contain 'dry-run': %s", text)
	}
	if !strings.Contains(text, `"posted": 1`) {
		t.Errorf("should have posted 1 finding: %s", text)
	}
}

func TestHandleApplyReviewFindings_MissingFindings(t *testing.T) {
	t.Parallel()

	h := newTestHandler(&mockPlatform{})
	result, err := h.handleApplyReviewFindings(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"pr":        float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result when findings not provided")
	}
}

func TestHandleApplyReviewFindings_DefaultDryRun(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{
		prContext: &platform.PRContext{
			ID: 42,
			Files: []platform.ChangedFile{
				{Path: "main.go", Status: "modified"},
			},
			DiffHunks: []platform.DiffHunk{
				{Path: "main.go", NewStart: 1, NewLines: 10, Body: "+code"},
			},
		},
		botComments: []platform.Comment{},
	}

	h := newHandler(mock, config.Config{
		Review: config.ReviewConfig{
			MaxComments:       25,
			BotLabel:          "crobot",
			SeverityThreshold: "warning",
		},
	})

	findings := []any{
		map[string]any{
			"path":     "main.go",
			"line":     float64(5),
			"side":     "new",
			"severity": "warning",
			"category": "style",
			"message":  "Test finding",
		},
	}

	// Don't specify dry_run — should default to true.
	result, err := h.handleApplyReviewFindings(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"pr":        float64(42),
		"findings":  findings,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := resultText(t, result)
		t.Fatalf("expected success, got error: %s", text)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "dry-run") {
		t.Errorf("should default to dry-run: %s", text)
	}
}

// --- export_local_context handler tests (#14) ---

func TestHandleExportLocalContext_Success(t *testing.T) {
	// This test requires a real git repo because the local provider calls git.
	dir := setupLocalTestRepo(t)

	h := newTestHandler(&mockPlatform{})
	result, err := h.handleExportLocalContext(t.Context(), makeRequest(map[string]any{
		"base_branch": "master",
		"repo_dir":    dir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := resultText(t, result)
		t.Fatalf("expected success, got error: %s", text)
	}

	text := resultText(t, result)

	var prCtx platform.PRContext
	if err := json.Unmarshal([]byte(text), &prCtx); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if prCtx.State != "local" {
		t.Errorf("State = %q, want %q", prCtx.State, "local")
	}
	if prCtx.SourceBranch != "feature" {
		t.Errorf("SourceBranch = %q, want %q", prCtx.SourceBranch, "feature")
	}
	if prCtx.TargetBranch != "master" {
		t.Errorf("TargetBranch = %q, want %q", prCtx.TargetBranch, "master")
	}
	if len(prCtx.Files) == 0 {
		t.Error("expected at least one changed file")
	}
}

func TestHandleExportLocalContext_BadRepo(t *testing.T) {
	h := newTestHandler(&mockPlatform{})
	result, err := h.handleExportLocalContext(t.Context(), makeRequest(map[string]any{
		"repo_dir": t.TempDir(), // empty dir, not a git repo
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for non-repo directory")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "failed to export local context") {
		t.Errorf("error should contain sanitized message: %s", text)
	}
}

func TestHandleExportLocalContext_BadBranch(t *testing.T) {
	dir := setupLocalTestRepo(t)

	h := newTestHandler(&mockPlatform{})
	result, err := h.handleExportLocalContext(t.Context(), makeRequest(map[string]any{
		"base_branch": "nonexistent-branch",
		"repo_dir":    dir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for bad base branch")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "failed to export local context") {
		t.Errorf("error should contain sanitized message: %s", text)
	}
}

func TestHandleExportLocalContext_Defaults(t *testing.T) {
	dir := setupLocalTestRepo(t)

	h := newTestHandler(&mockPlatform{})
	// Don't provide base_branch or repo_dir — should use defaults.
	// repo_dir defaults to "." which won't work from test dir, so just test base_branch default.
	result, err := h.handleExportLocalContext(t.Context(), makeRequest(map[string]any{
		"repo_dir": dir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := resultText(t, result)
		t.Fatalf("expected success with default base_branch, got error: %s", text)
	}

	text := resultText(t, result)
	var prCtx platform.PRContext
	if err := json.Unmarshal([]byte(text), &prCtx); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	// Default base_branch is "master", which is what setupLocalTestRepo creates.
	if prCtx.TargetBranch != "master" {
		t.Errorf("TargetBranch = %q, want %q (default)", prCtx.TargetBranch, "master")
	}
}

// setupLocalTestRepo creates a temp git repo with a master branch and a feature
// branch with changes, suitable for testing the local context handler.
func setupLocalTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}

	run("init", "-b", "master")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@test.com")
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "hello.txt")
	run("commit", "-m", "initial")
	run("checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "hello.txt")
	run("commit", "-m", "modify hello")

	return dir
}

func TestDispatch_UnknownTool(t *testing.T) {
	t.Parallel()

	h := newTestHandler(&mockPlatform{})
	fn := h.dispatch("nonexistent_tool")
	result, err := fn(t.Context(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for unknown tool")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "unknown tool") {
		t.Errorf("error should mention unknown tool: %s", text)
	}
}

func TestDispatch_RoutesToCorrectHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		toolName string
		args     map[string]any
	}{
		{
			name:     "export_pr_context routes correctly",
			toolName: "export_pr_context",
			args:     map[string]any{"workspace": "ws", "repo": "rp", "pr": float64(1)},
		},
		{
			name:     "get_file_snippet routes correctly",
			toolName: "get_file_snippet",
			args:     map[string]any{"workspace": "ws", "repo": "rp", "commit": "abc", "path": "f.go", "line": float64(1)},
		},
		{
			name:     "list_bot_comments routes correctly",
			toolName: "list_bot_comments",
			args:     map[string]any{"workspace": "ws", "repo": "rp", "pr": float64(1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockPlatform{
				prContext:   &platform.PRContext{ID: 1, Files: []platform.ChangedFile{{Path: "f.go", Status: "modified"}}},
				fileContent: []byte("line1\n"),
				botComments: []platform.Comment{},
			}

			h := newTestHandler(mock)
			fn := h.dispatch(tt.toolName)
			result, err := fn(t.Context(), makeRequest(tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// We're just verifying routing works (no panic, proper call),
			// not detailed output — that's tested in individual handler tests.
			_ = result
		})
	}
}

func TestDispatch_ExportLocalContextRoutes(t *testing.T) {
	dir := setupLocalTestRepo(t)

	h := newTestHandler(&mockPlatform{})
	fn := h.dispatch("export_local_context")
	result, err := fn(t.Context(), makeRequest(map[string]any{
		"repo_dir": dir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := resultText(t, result)
		t.Fatalf("dispatch to export_local_context failed: %s", text)
	}
}

func TestHandleGetFileSnippet_PlatformError(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{
		fileContentErr: fmt.Errorf("file not found"),
	}

	h := newTestHandler(mock)
	result, err := h.handleGetFileSnippet(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"commit":    "abc123",
		"path":      "missing.go",
		"line":      float64(1),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for platform failure")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "failed to fetch file content") {
		t.Errorf("error should contain sanitized message: %s", text)
	}
}

func TestHandleListBotComments_PlatformError(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{
		botCommentsErr: fmt.Errorf("unauthorized"),
	}

	h := newTestHandler(mock)
	result, err := h.handleListBotComments(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"pr":        float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for platform failure")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "failed to list bot comments") {
		t.Errorf("error should contain sanitized message: %s", text)
	}
}

func TestHandleApplyReviewFindings_WriteMode(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{
		prContext: &platform.PRContext{
			ID: 42,
			Files: []platform.ChangedFile{
				{Path: "main.go", Status: "modified"},
			},
			DiffHunks: []platform.DiffHunk{
				{Path: "main.go", NewStart: 1, NewLines: 10, Body: "+code"},
			},
		},
		botComments: []platform.Comment{},
		createComment: &platform.Comment{
			ID:   "created-123",
			Path: "main.go",
			Line: 5,
		},
	}

	h := newHandler(mock, config.Config{
		Review: config.ReviewConfig{
			MaxComments:       25,
			BotLabel:          "crobot",
			SeverityThreshold: "warning",
		},
	})

	findings := []any{
		map[string]any{
			"path":     "main.go",
			"line":     float64(5),
			"side":     "new",
			"severity": "warning",
			"category": "style",
			"message":  "Consider refactoring",
		},
	}

	result, err := h.handleApplyReviewFindings(t.Context(), makeRequest(map[string]any{
		"workspace": "ws",
		"repo":      "rp",
		"pr":        float64(42),
		"findings":  findings,
		"dry_run":   false,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := resultText(t, result)
		t.Fatalf("expected success, got error: %s", text)
	}

	text := resultText(t, result)
	// In write mode, comment IDs should be real (not "dry-run").
	if strings.Contains(text, "dry-run") {
		t.Errorf("write mode should not contain 'dry-run': %s", text)
	}
	if !strings.Contains(text, "created-123") {
		t.Errorf("should contain posted comment ID: %s", text)
	}
}
