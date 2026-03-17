package agent

import (
	"strings"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

func TestBuildSystemPrompt(t *testing.T) {
	t.Parallel()

	prompt := BuildSystemPrompt()

	required := []string{
		"ReviewFinding",
		"severity",
		"path",
		"line",
		"message",
		"JSON array",
		"Review Philosophy",
		"What to comment on",
		"What to skip",
		"Output Format",
		"You do NOT need to fetch any data",
	}

	for _, s := range required {
		if !strings.Contains(prompt, s) {
			t.Errorf("BuildSystemPrompt() missing required content: %q", s)
		}
	}
}

func TestBuildReviewPrompt_FullPR(t *testing.T) {
	t.Parallel()

	prCtx := &platform.PRContext{
		ID:           42,
		Title:        "Add authentication middleware",
		Description:  "Implements JWT-based auth for API routes.",
		Author:       "jdoe",
		SourceBranch: "feature/auth",
		TargetBranch: "main",
		State:        "OPEN",
		HeadCommit:   "abc123",
		BaseCommit:   "def456",
		Files: []platform.ChangedFile{
			{Path: "src/auth.go", Status: "added"},
			{Path: "src/middleware.go", Status: "modified"},
			{Path: "src/old.go", OldPath: "src/legacy.go", Status: "renamed"},
		},
		DiffHunks: []platform.DiffHunk{
			{
				Path:     "src/auth.go",
				OldStart: 0, OldLines: 0,
				NewStart: 1, NewLines: 5,
				Body: "+package auth\n+\n+func Verify(token string) bool {\n+\treturn token != \"\"\n+}\n",
			},
			{
				Path:     "src/middleware.go",
				OldStart: 10, OldLines: 3,
				NewStart: 10, NewLines: 5,
				Body: " func handler(w http.ResponseWriter, r *http.Request) {\n-\t// no auth\n+\tif !auth.Verify(r.Header.Get(\"Authorization\")) {\n+\t\thttp.Error(w, \"unauthorized\", 401)\n+\t\treturn\n+\t}\n",
			},
		},
	}

	ref := &platform.PRRequest{Workspace: "myteam", Repo: "myrepo", PRNumber: 42}
	prompt := BuildReviewPrompt(prCtx, ref)

	// Check PR metadata
	for _, s := range []string{
		"Add authentication middleware",
		"jdoe",
		"feature/auth",
		"main",
		"OPEN",
		"JWT-based auth",
		"myteam",
		"myrepo",
		"42",
	} {
		if !strings.Contains(prompt, s) {
			t.Errorf("BuildReviewPrompt() missing metadata: %q", s)
		}
	}

	// Check changed files
	for _, s := range []string{
		"src/auth.go",
		"added",
		"src/middleware.go",
		"modified",
		"src/old.go",
		"renamed",
		"src/legacy.go",
	} {
		if !strings.Contains(prompt, s) {
			t.Errorf("BuildReviewPrompt() missing file info: %q", s)
		}
	}

	// Check diff content
	for _, s := range []string{
		"@@ -0,0 +1,5 @@",
		"+func Verify(token string) bool",
		"@@ -10,3 +10,5 @@",
		"+\tif !auth.Verify",
	} {
		if !strings.Contains(prompt, s) {
			t.Errorf("BuildReviewPrompt() missing diff content: %q", s)
		}
	}

	// Check instructions
	if !strings.Contains(prompt, "fingerprint") {
		t.Error("BuildReviewPrompt() missing fingerprint instruction")
	}
}

func TestBuildReviewPrompt_EmptyPR(t *testing.T) {
	t.Parallel()

	prCtx := &platform.PRContext{
		Title:        "Empty PR",
		Author:       "nobody",
		SourceBranch: "fix/empty",
		TargetBranch: "main",
		State:        "OPEN",
	}

	prompt := BuildReviewPrompt(prCtx, nil)

	if !strings.Contains(prompt, "No files changed") {
		t.Error("BuildReviewPrompt() should indicate no files changed for empty PR")
	}
	if !strings.Contains(prompt, "No diff hunks available") {
		t.Error("BuildReviewPrompt() should indicate no diff hunks for empty PR")
	}
	if !strings.Contains(prompt, "Empty PR") {
		t.Error("BuildReviewPrompt() should include PR title")
	}
}

func TestBuildReviewPrompt_MultipleHunksSameFile(t *testing.T) {
	t.Parallel()

	prCtx := &platform.PRContext{
		Title:        "Multi-hunk change",
		Author:       "dev",
		SourceBranch: "fix/multi",
		TargetBranch: "main",
		State:        "OPEN",
		Files: []platform.ChangedFile{
			{Path: "main.go", Status: "modified"},
		},
		DiffHunks: []platform.DiffHunk{
			{
				Path:     "main.go",
				OldStart: 1, OldLines: 3,
				NewStart: 1, NewLines: 3,
				Body: " package main\n-var old = 1\n+var new = 2\n",
			},
			{
				Path:     "main.go",
				OldStart: 20, OldLines: 2,
				NewStart: 20, NewLines: 3,
				Body: " func run() {\n+\tlog.Println(\"start\")\n \treturn\n",
			},
		},
	}

	prompt := BuildReviewPrompt(prCtx, nil)

	// File header should appear exactly once
	count := strings.Count(prompt, "### main.go")
	if count != 1 {
		t.Errorf("expected file header once, got %d times", count)
	}

	// Both hunks should be present
	if !strings.Contains(prompt, "@@ -1,3 +1,3 @@") {
		t.Error("missing first hunk header")
	}
	if !strings.Contains(prompt, "@@ -20,2 +20,3 @@") {
		t.Error("missing second hunk header")
	}
}

func TestBuildFullPrompt(t *testing.T) {
	t.Parallel()

	prCtx := &platform.PRContext{
		Title:        "Test PR",
		Author:       "tester",
		SourceBranch: "test",
		TargetBranch: "main",
		State:        "OPEN",
	}

	full := BuildFullPrompt(prCtx, nil)

	// Should contain both system and review parts
	if !strings.Contains(full, "Review Philosophy") {
		t.Error("BuildFullPrompt() missing system prompt content")
	}
	if !strings.Contains(full, "Test PR") {
		t.Error("BuildFullPrompt() missing review prompt content")
	}
	if !strings.Contains(full, "---") {
		t.Error("BuildFullPrompt() missing separator between system and review prompts")
	}
}
