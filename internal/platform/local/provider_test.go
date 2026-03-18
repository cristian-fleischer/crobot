package local

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

// setupTestRepo creates a temporary git repo with a base branch and some
// committed + uncommitted changes for testing.
func setupTestRepo(t *testing.T) string {
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

	// Initialize repo with a base branch.
	run("init", "-b", "master")
	run("config", "user.name", "Test User")
	run("config", "user.email", "test@test.com")

	// Create initial file on master.
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "hello.txt")
	run("commit", "-m", "initial commit")

	// Create a feature branch with changes.
	run("checkout", "-b", "feature")

	// Committed change: modify hello.txt.
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "hello.txt")
	run("commit", "-m", "modify hello")

	// Committed change: add new file.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new file\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "new.txt")
	run("commit", "-m", "add new file")

	// Unstaged change: modify hello.txt further (not committed).
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world!!!\n"), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestGetPRContext(t *testing.T) {
	dir := setupTestRepo(t)
	p := New("master", dir)

	ctx := context.Background()
	prCtx, err := p.GetPRContext(ctx, prRequest())
	if err != nil {
		t.Fatalf("GetPRContext failed: %v", err)
	}

	if prCtx.SourceBranch != "feature" {
		t.Errorf("SourceBranch = %q, want %q", prCtx.SourceBranch, "feature")
	}
	if prCtx.TargetBranch != "master" {
		t.Errorf("TargetBranch = %q, want %q", prCtx.TargetBranch, "master")
	}
	if prCtx.State != "local" {
		t.Errorf("State = %q, want %q", prCtx.State, "local")
	}
	if prCtx.Author != "Test User" {
		t.Errorf("Author = %q, want %q", prCtx.Author, "Test User")
	}
	if prCtx.Title != "Local review: feature" {
		t.Errorf("Title = %q, want %q", prCtx.Title, "Local review: feature")
	}

	// Should have 2 files: hello.txt (modified) and new.txt (added).
	if len(prCtx.Files) != 2 {
		t.Fatalf("Files count = %d, want 2", len(prCtx.Files))
	}

	fileMap := map[string]string{}
	for _, f := range prCtx.Files {
		fileMap[f.Path] = f.Status
	}
	if fileMap["hello.txt"] != "modified" {
		t.Errorf("hello.txt status = %q, want %q", fileMap["hello.txt"], "modified")
	}
	if fileMap["new.txt"] != "added" {
		t.Errorf("new.txt status = %q, want %q", fileMap["new.txt"], "added")
	}

	// Should have diff hunks.
	if len(prCtx.DiffHunks) == 0 {
		t.Error("DiffHunks is empty, want at least 1")
	}

	// The diff should include the unstaged change ("hello world!!!" not just "hello world").
	found := false
	for _, h := range prCtx.DiffHunks {
		if h.Path == "hello.txt" && containsString(h.Body, "hello world!!!") {
			found = true
		}
	}
	if !found {
		t.Error("diff should include unstaged changes (hello world!!!)")
	}
}

func TestGetPRContext_NoChanges(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "f.txt")
	run("commit", "-m", "init")

	p := New("master", dir)
	prCtx, err := p.GetPRContext(context.Background(), prRequest())
	if err != nil {
		t.Fatalf("GetPRContext failed: %v", err)
	}
	if len(prCtx.Files) != 0 {
		t.Errorf("Files count = %d, want 0 (no changes)", len(prCtx.Files))
	}
}

func TestGetPRContext_BadBaseBranch(t *testing.T) {
	dir := setupTestRepo(t)
	p := New("nonexistent-branch", dir)

	_, err := p.GetPRContext(context.Background(), prRequest())
	if err == nil {
		t.Fatal("expected error for bad base branch")
	}
	if !containsString(err.Error(), "not found") {
		t.Errorf("error = %q, want it to mention 'not found'", err)
	}
}

func TestGetPRContext_NotARepo(t *testing.T) {
	p := New("master", t.TempDir())
	_, err := p.GetPRContext(context.Background(), prRequest())
	if err == nil {
		t.Fatal("expected error for non-repo directory")
	}
	if !containsString(err.Error(), "not a git repository") {
		t.Errorf("error = %q, want it to mention 'not a git repository'", err)
	}
}

func TestListBotComments_Empty(t *testing.T) {
	p := New("master", ".")
	comments, err := p.ListBotComments(context.Background(), prRequest())
	if err != nil {
		t.Fatalf("ListBotComments failed: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected empty comments, got %d", len(comments))
	}
}

func TestCreateInlineComment_Error(t *testing.T) {
	p := New("master", ".")
	_, err := p.CreateInlineComment(context.Background(), prRequest(), platform.InlineComment{})
	if err == nil {
		t.Fatal("expected error from CreateInlineComment in local mode")
	}
}

func TestParseNameStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   int
		status map[string]string
	}{
		{
			name:   "mixed changes",
			input:  "M\tfoo.go\nA\tbar.go\nD\told.go",
			want:   3,
			status: map[string]string{"foo.go": "modified", "bar.go": "added", "old.go": "deleted"},
		},
		{
			name:  "rename",
			input: "R100\told.go\tnew.go",
			want:  1,
			status: map[string]string{"new.go": "renamed"},
		},
		{
			name:  "empty",
			input: "",
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := parseNameStatus(tt.input)
			if len(files) != tt.want {
				t.Fatalf("got %d files, want %d", len(files), tt.want)
			}
			for _, f := range files {
				if expected, ok := tt.status[f.Path]; ok {
					if f.Status != expected {
						t.Errorf("%s: status = %q, want %q", f.Path, f.Status, expected)
					}
				}
			}
		})
	}
}

func TestRepoName(t *testing.T) {
	p := New("master", "/home/user/projects/my-repo")
	if name := p.RepoName(); name != "my-repo" {
		t.Errorf("RepoName = %q, want %q", name, "my-repo")
	}
}

// helpers

func prRequest() platform.PRRequest {
	return platform.PRRequest{Workspace: "local", Repo: "test", PRNumber: 0}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && strings.Contains(s, substr))
}
