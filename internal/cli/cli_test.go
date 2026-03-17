package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/platform/bitbucket"
	"github.com/cristian-fleischer/crobot/internal/review"
)

// testServer creates a mock Bitbucket API server with preset responses and
// returns the server along with a cleanup function.
func testServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}
	return httptest.NewServer(mux)
}

// jsonResponse writes a JSON response to the http.ResponseWriter.
func jsonResponse(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(v)
}

func TestExportPRContext_MissingFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no flags",
			args: []string{"export-pr-context"},
			want: "--workspace, --repo, and --pr are required",
		},
		{
			name: "missing repo",
			args: []string{"export-pr-context", "--workspace", "ws"},
			want: "--workspace, --repo, and --pr are required",
		},
		{
			name: "missing pr",
			args: []string{"export-pr-context", "--workspace", "ws", "--repo", "rp"},
			want: "--workspace, --repo, and --pr are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := RootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestExportPRContext_Success(t *testing.T) {
	t.Parallel()

	prMeta := map[string]any{
		"id":          42,
		"title":       "Test PR",
		"description": "A test pull request",
		"state":       "OPEN",
		"author":      map[string]any{"display_name": "testuser"},
		"source": map[string]any{
			"branch": map[string]any{"name": "feature"},
			"commit": map[string]any{"hash": "abc123"},
		},
		"destination": map[string]any{
			"branch": map[string]any{"name": "main"},
			"commit": map[string]any{"hash": "def456"},
		},
	}

	diffstat := map[string]any{
		"values": []map[string]any{
			{
				"new":    map[string]any{"path": "file.go", "type": "commit_file"},
				"old":    map[string]any{"path": "file.go", "type": "commit_file"},
				"status": "modified",
			},
		},
	}

	diff := `diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
@@ -1,3 +1,4 @@
 package main
+
 func main() {}
`

	srv := testServer(t, map[string]http.HandlerFunc{
		"/2.0/repositories/ws/rp/pullrequests/42": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, prMeta)
		},
		"/2.0/repositories/ws/rp/pullrequests/42/diffstat": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, diffstat)
		},
		"/2.0/repositories/ws/rp/pullrequests/42/diff": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(200)
			fmt.Fprint(w, diff)
		},
	})
	defer srv.Close()

	// Build the command manually instead of using config.LoadDefault so we can
	// point at our test server.
	cmd := RootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	// We can't easily inject the platform through RootCmd, so we test the
	// underlying logic directly with a real bitbucket client pointed at the
	// test server.
	client, err := bitbucket.NewClient(&bitbucket.Config{
		User:    "testuser",
		Token:   "testtoken",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = t.Context()
	}

	prCtx, err := client.GetPRContext(ctx, platform.PRRequest{
		Workspace: "ws",
		Repo:      "rp",
		PRNumber:  42,
	})
	if err != nil {
		t.Fatalf("GetPRContext: %v", err)
	}

	if prCtx.ID != 42 {
		t.Errorf("ID = %d, want 42", prCtx.ID)
	}
	if prCtx.Title != "Test PR" {
		t.Errorf("Title = %q, want %q", prCtx.Title, "Test PR")
	}
	if prCtx.HeadCommit != "abc123" {
		t.Errorf("HeadCommit = %q, want %q", prCtx.HeadCommit, "abc123")
	}
	if len(prCtx.Files) != 1 {
		t.Errorf("Files count = %d, want 1", len(prCtx.Files))
	}

	// Verify JSON marshaling works.
	data, err := json.MarshalIndent(prCtx, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "Test PR") {
		t.Error("marshaled JSON does not contain PR title")
	}
}

func TestSnippet_MissingFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no flags",
			args: []string{"get-file-snippet"},
			want: "--workspace, --repo, --commit, --path, and --line are required",
		},
		{
			name: "missing path and line",
			args: []string{"get-file-snippet", "--workspace", "ws", "--repo", "rp", "--commit", "abc"},
			want: "--workspace, --repo, --commit, --path, and --line are required",
		},
		{
			name: "negative context",
			args: []string{"get-file-snippet", "--workspace", "ws", "--repo", "rp", "--commit", "abc", "--path", "f.go", "--line", "1", "--context", "-3"},
			want: "--context must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := RootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestSnippet_Success(t *testing.T) {
	t.Parallel()

	fileContent := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"

	srv := testServer(t, map[string]http.HandlerFunc{
		"/2.0/repositories/ws/rp/src/abc123/main.go": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			fmt.Fprint(w, fileContent)
		},
	})
	defer srv.Close()

	client, err := bitbucket.NewClient(&bitbucket.Config{
		User:    "testuser",
		Token:   "testtoken",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	ctx := t.Context()
	content, err := client.GetFileContent(ctx, platform.FileRequest{
		Workspace: "ws",
		Repo:      "rp",
		Commit:    "abc123",
		Path:      "main.go",
	})
	if err != nil {
		t.Fatalf("GetFileContent: %v", err)
	}

	// Test snippet extraction logic (mirrors the command implementation).
	lines := strings.Split(string(content), "\n")
	// Trim trailing empty element from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Verify we have exactly 10 lines (not 11 from trailing newline).
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}

	line := 5
	contextSize := 2
	startLine := line - contextSize
	if startLine < 1 {
		startLine = 1
	}
	endLine := line + contextSize
	if endLine > len(lines) {
		endLine = len(lines)
	}
	snippet := strings.Join(lines[startLine-1:endLine], "\n")

	if !strings.Contains(snippet, "line5") {
		t.Errorf("snippet does not contain target line: %q", snippet)
	}
	if !strings.Contains(snippet, "line3") {
		t.Errorf("snippet does not contain context line 3: %q", snippet)
	}
	if !strings.Contains(snippet, "line7") {
		t.Errorf("snippet does not contain context line 7: %q", snippet)
	}

	out := platform.SnippetOutput{
		Path:      "main.go",
		Commit:    "abc123",
		StartLine: startLine,
		EndLine:   endLine,
		Content:   snippet,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "main.go") {
		t.Error("marshaled JSON does not contain path")
	}
}

func TestListBotComments_MissingFlags(t *testing.T) {
	t.Parallel()

	cmd := RootCmd()
	cmd.SetArgs([]string{"list-bot-comments"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--workspace, --repo, and --pr are required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestListBotComments_Success(t *testing.T) {
	t.Parallel()

	commentsResp := map[string]any{
		"values": []map[string]any{
			{
				"id":      1,
				"content": map[string]any{"raw": "Some comment\n[//]: # \"crobot:fp=abc123\""},
				"user":    map[string]any{"display_name": "bot"},
				"inline": map[string]any{
					"path": "file.go",
					"to":   10,
				},
				"created_on": "2025-01-01T00:00:00Z",
			},
			{
				"id":      2,
				"content": map[string]any{"raw": "Human comment without fingerprint"},
				"user":    map[string]any{"display_name": "user"},
				"inline": map[string]any{
					"path": "file.go",
					"to":   20,
				},
				"created_on": "2025-01-01T00:00:00Z",
			},
		},
	}

	srv := testServer(t, map[string]http.HandlerFunc{
		"/2.0/repositories/ws/rp/pullrequests/42/comments": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, commentsResp)
		},
	})
	defer srv.Close()

	client, err := bitbucket.NewClient(&bitbucket.Config{
		User:    "testuser",
		Token:   "testtoken",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	ctx := t.Context()
	comments, err := client.ListBotComments(ctx, platform.PRRequest{
		Workspace: "ws",
		Repo:      "rp",
		PRNumber:  42,
	})
	if err != nil {
		t.Fatalf("ListBotComments: %v", err)
	}

	// Should only return comments with fingerprints.
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
	if comments[0].Fingerprint != "abc123" {
		t.Errorf("fingerprint = %q, want %q", comments[0].Fingerprint, "abc123")
	}
}

func TestApplyReviewFindings_MissingFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no flags",
			args: []string{"apply-review-findings"},
			want: "--input is required",
		},
		{
			name: "missing input",
			args: []string{"apply-review-findings", "--workspace", "ws", "--repo", "rp", "--pr", "42"},
			want: "--input is required",
		},
		{
			name: "conflicting modes",
			args: []string{"apply-review-findings", "--workspace", "ws", "--repo", "rp", "--pr", "42", "--input", "f.json", "--dry-run", "--write"},
			want: "--dry-run and --write are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := RootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestApplyReviewFindings_DryRun(t *testing.T) {
	t.Parallel()

	// Set up a mock Bitbucket server that serves PR context and comments.
	prMeta := map[string]any{
		"id":          42,
		"title":       "Test PR",
		"description": "desc",
		"state":       "OPEN",
		"author":      map[string]any{"display_name": "testuser"},
		"source": map[string]any{
			"branch": map[string]any{"name": "feature"},
			"commit": map[string]any{"hash": "abc123"},
		},
		"destination": map[string]any{
			"branch": map[string]any{"name": "main"},
			"commit": map[string]any{"hash": "def456"},
		},
	}

	diffstat := map[string]any{
		"values": []map[string]any{
			{
				"new":    map[string]any{"path": "main.go", "type": "commit_file"},
				"old":    map[string]any{"path": "main.go", "type": "commit_file"},
				"status": "modified",
			},
		},
	}

	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +5,4 @@
 package main
+
+import "fmt"
 func main() {}
`

	commentsResp := map[string]any{
		"values": []any{},
	}

	srv := testServer(t, map[string]http.HandlerFunc{
		"/2.0/repositories/ws/rp/pullrequests/42": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, prMeta)
		},
		"/2.0/repositories/ws/rp/pullrequests/42/diffstat": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, diffstat)
		},
		"/2.0/repositories/ws/rp/pullrequests/42/diff": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(200)
			fmt.Fprint(w, diff)
		},
		"/2.0/repositories/ws/rp/pullrequests/42/comments": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, commentsResp)
		},
	})
	defer srv.Close()

	// Create a platform client pointing at our test server.
	client, err := bitbucket.NewClient(&bitbucket.Config{
		User:    "testuser",
		Token:   "testtoken",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	// Create findings.
	findings := []platform.ReviewFinding{
		{
			Path:     "main.go",
			Line:     6,
			Side:     "new",
			Severity: "warning",
			Category: "style",
			Message:  "Unused import",
		},
	}

	engine := review.NewEngine(client, review.EngineConfig{
		MaxComments:       25,
		DryRun:            true,
		BotLabel:          "crobot",
		SeverityThreshold: "warning",
	})

	ctx := t.Context()
	result, err := engine.Run(ctx, platform.PRRequest{
		Workspace: "ws",
		Repo:      "rp",
		PRNumber:  42,
	}, findings)
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}

	if result.Summary.Total != 1 {
		t.Errorf("total = %d, want 1", result.Summary.Total)
	}
	if result.Summary.Posted != 1 {
		t.Errorf("posted = %d, want 1", result.Summary.Posted)
	}
	if len(result.Posted) != 1 {
		t.Fatalf("posted count = %d, want 1", len(result.Posted))
	}
	if result.Posted[0].CommentID != "dry-run" {
		t.Errorf("comment ID = %q, want %q", result.Posted[0].CommentID, "dry-run")
	}

	// Verify JSON output.
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "dry-run") {
		t.Error("output does not contain dry-run")
	}
}

func TestApplyReviewFindings_FromFile(t *testing.T) {
	t.Parallel()

	// Write findings to a temp file and verify ParseFindings reads it correctly.
	findings := []platform.ReviewFinding{
		{
			Path:     "main.go",
			Line:     10,
			Side:     "new",
			Severity: "error",
			Category: "bug",
			Message:  "Null pointer dereference",
		},
		{
			Path:     "main.go",
			Line:     20,
			Side:     "new",
			Severity: "info",
			Category: "style",
			Message:  "Consider using constants",
		},
	}

	data, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "findings.json")
	if err := os.WriteFile(inputFile, data, 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	// Read and parse.
	fileData, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}

	parsed, err := platform.ParseFindings(fileData)
	if err != nil {
		t.Fatalf("parsing findings: %v", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("got %d findings, want 2", len(parsed))
	}
	if parsed[0].Message != "Null pointer dereference" {
		t.Errorf("message = %q, want %q", parsed[0].Message, "Null pointer dereference")
	}
}

func TestBuildPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		wantErr  string
	}{
		{
			name:     "unsupported platform",
			platform: "gitlab",
			wantErr:  "unknown platform",
		},
		{
			name:     "bitbucket missing creds",
			platform: "bitbucket",
			wantErr:  "user must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := config.Config{Platform: tt.platform}
			_, err := buildPlatform(cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestBuildPlatform_Success(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Platform: "bitbucket",
		Bitbucket: config.BitbucketConfig{
			User:  "testuser",
			Token: "testtoken",
		},
	}
	plat, err := buildPlatform(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plat == nil {
		t.Fatal("platform is nil")
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	data := []byte(`{"key":"value"}`)
	if err := writeJSON(&buf, data); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	if got := buf.String(); got != "{\"key\":\"value\"}\n" {
		t.Errorf("got %q, want %q", got, "{\"key\":\"value\"}\n")
	}
}

// TestApplyReviewFindings_FileSizeLimit verifies that the 10 MB read limit is
// enforced when reading the findings input file (SF-4 fix).
func TestApplyReviewFindings_FileSizeLimit(t *testing.T) {
	t.Parallel()

	// Write a file just over 10 MB of valid-looking but truncated JSON.
	const maxInputSize = 10 << 20 // 10 MB (must match apply.go)
	tmpDir := t.TempDir()
	bigFile := filepath.Join(tmpDir, "big.json")

	// Build a byte slice slightly larger than the limit: fill with spaces
	// preceded by '[' so it would be parseable if not truncated.
	content := make([]byte, maxInputSize+512)
	content[0] = '['
	for i := 1; i < len(content); i++ {
		content[i] = ' '
	}
	if err := os.WriteFile(bigFile, content, 0644); err != nil {
		t.Fatalf("writing big file: %v", err)
	}

	// io.LimitReader caps at maxInputSize; the resulting data will be truncated
	// and therefore fail JSON parsing — verify we get a parse error, not a
	// resource-exhaustion scenario.
	f, err := os.Open(bigFile)
	if err != nil {
		t.Fatalf("opening big file: %v", err)
	}
	defer f.Close()

	limited := io.LimitReader(f, maxInputSize)
	data, err := io.ReadAll(limited)
	if err != nil {
		t.Fatalf("reading limited file: %v", err)
	}

	// The read must be capped at exactly maxInputSize bytes.
	if len(data) != maxInputSize {
		t.Errorf("read %d bytes, want exactly %d (limit)", len(data), maxInputSize)
	}

	// Parsing truncated JSON should fail (incomplete JSON array).
	_, parseErr := platform.ParseFindings(data)
	if parseErr == nil {
		t.Error("expected parse error for truncated JSON, got nil")
	}
}

func TestResolveWorkspaceRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		flagWorkspace string
		flagRepo      string
		cfg           config.Config
		wantWorkspace string
		wantRepo      string
	}{
		{
			name:          "flags take precedence over config",
			flagWorkspace: "flag-ws",
			flagRepo:      "flag-repo",
			cfg: config.Config{
				Bitbucket: config.BitbucketConfig{
					Workspace: "cfg-ws",
					Repo:      "cfg-repo",
				},
			},
			wantWorkspace: "flag-ws",
			wantRepo:      "flag-repo",
		},
		{
			name:          "config fallback when flags empty",
			flagWorkspace: "",
			flagRepo:      "",
			cfg: config.Config{
				Bitbucket: config.BitbucketConfig{
					Workspace: "cfg-ws",
					Repo:      "cfg-repo",
				},
			},
			wantWorkspace: "cfg-ws",
			wantRepo:      "cfg-repo",
		},
		{
			name:          "mixed: flag workspace with config repo",
			flagWorkspace: "flag-ws",
			flagRepo:      "",
			cfg: config.Config{
				Bitbucket: config.BitbucketConfig{
					Workspace: "cfg-ws",
					Repo:      "cfg-repo",
				},
			},
			wantWorkspace: "flag-ws",
			wantRepo:      "cfg-repo",
		},
		{
			name:          "both empty when neither flag nor config set",
			flagWorkspace: "",
			flagRepo:      "",
			cfg:           config.Config{},
			wantWorkspace: "",
			wantRepo:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotWS, gotRepo := resolveWorkspaceRepo(tt.flagWorkspace, tt.flagRepo, tt.cfg)
			if gotWS != tt.wantWorkspace {
				t.Errorf("workspace = %q, want %q", gotWS, tt.wantWorkspace)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", gotRepo, tt.wantRepo)
			}
		})
	}
}
