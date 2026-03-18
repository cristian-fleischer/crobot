package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

func TestGetPRContext_HappyPath(t *testing.T) {
	t.Parallel()

	prResponse := `{
		"number": 42,
		"title": "Add feature X",
		"body": "Implements feature X",
		"state": "open",
		"merged": false,
		"user": {"login": "octocat", "id": 1, "type": "User"},
		"head": {"ref": "feature/x", "sha": "abc123"},
		"base": {"ref": "main", "sha": "def456"}
	}`

	filesResponse := `[
		{
			"filename": "src/auth.ts",
			"status": "modified",
			"additions": 5,
			"deletions": 2
		},
		{
			"filename": "src/new_file.ts",
			"status": "added",
			"additions": 10,
			"deletions": 0
		}
	]`

	diffResponse := `diff --git a/src/auth.ts b/src/auth.ts
--- a/src/auth.ts
+++ b/src/auth.ts
@@ -10,5 +10,7 @@ function authenticate() {
   const token = getToken();
+  console.log(token);
+  validateToken(token);
   return token;
 }
diff --git a/src/new_file.ts b/src/new_file.ts
new file mode 100644
--- /dev/null
+++ b/src/new_file.ts
@@ -0,0 +1,3 @@
+export function newFunc() {
+  return true;
+}
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		switch {
		case strings.HasSuffix(r.URL.Path, "/files"):
			fmt.Fprint(w, filesResponse)
		case accept == "application/vnd.github.diff":
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, diffResponse)
		case strings.Contains(r.URL.Path, "/pulls/42"):
			fmt.Fprint(w, prResponse)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := testClient(t, server)
	prCtx, err := c.GetPRContext(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prCtx.ID != 42 {
		t.Errorf("ID: got %d, want 42", prCtx.ID)
	}
	if prCtx.Title != "Add feature X" {
		t.Errorf("Title: got %q", prCtx.Title)
	}
	if prCtx.Description != "Implements feature X" {
		t.Errorf("Description: got %q", prCtx.Description)
	}
	if prCtx.Author != "octocat" {
		t.Errorf("Author: got %q", prCtx.Author)
	}
	if prCtx.SourceBranch != "feature/x" {
		t.Errorf("SourceBranch: got %q", prCtx.SourceBranch)
	}
	if prCtx.TargetBranch != "main" {
		t.Errorf("TargetBranch: got %q", prCtx.TargetBranch)
	}
	if prCtx.State != "OPEN" {
		t.Errorf("State: got %q, want OPEN", prCtx.State)
	}
	if prCtx.HeadCommit != "abc123" {
		t.Errorf("HeadCommit: got %q", prCtx.HeadCommit)
	}
	if prCtx.BaseCommit != "def456" {
		t.Errorf("BaseCommit: got %q", prCtx.BaseCommit)
	}
	if len(prCtx.Files) != 2 {
		t.Fatalf("Files: got %d, want 2", len(prCtx.Files))
	}
	if prCtx.Files[0].Path != "src/auth.ts" {
		t.Errorf("Files[0].Path: got %q", prCtx.Files[0].Path)
	}
	if prCtx.Files[0].Status != "modified" {
		t.Errorf("Files[0].Status: got %q", prCtx.Files[0].Status)
	}
	if prCtx.Files[1].Path != "src/new_file.ts" {
		t.Errorf("Files[1].Path: got %q", prCtx.Files[1].Path)
	}
	if prCtx.Files[1].Status != "added" {
		t.Errorf("Files[1].Status: got %q", prCtx.Files[1].Status)
	}
	if len(prCtx.DiffHunks) != 2 {
		t.Fatalf("DiffHunks: got %d, want 2", len(prCtx.DiffHunks))
	}
}

func TestGetPRContext_FilesPagination(t *testing.T) {
	t.Parallel()

	prResponse := `{
		"number": 1, "title": "PR", "body": "",
		"state": "open", "merged": false,
		"user": {"login": "bob"},
		"head": {"ref": "feat", "sha": "aaa"},
		"base": {"ref": "main", "sha": "bbb"}
	}`

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		switch {
		case strings.HasSuffix(r.URL.Path, "/files") && r.URL.Query().Get("page") == "":
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/owner/repo/pulls/1/files?page=2&per_page=100>; rel="next"`, server.URL))
			fmt.Fprint(w, `[{"filename": "file1.go", "status": "modified"}]`)
		case strings.HasSuffix(r.URL.Path, "/files") && r.URL.Query().Get("page") == "2":
			fmt.Fprint(w, `[{"filename": "file2.go", "status": "added"}]`)
		case accept == "application/vnd.github.diff":
			fmt.Fprint(w, "")
		default:
			fmt.Fprint(w, prResponse)
		}
	}))
	defer server.Close()

	c := testClient(t, server)
	prCtx, err := c.GetPRContext(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prCtx.Files) != 2 {
		t.Fatalf("Files: got %d, want 2", len(prCtx.Files))
	}
	if prCtx.Files[0].Path != "file1.go" {
		t.Errorf("Files[0].Path: got %q", prCtx.Files[0].Path)
	}
	if prCtx.Files[1].Path != "file2.go" {
		t.Errorf("Files[1].Path: got %q", prCtx.Files[1].Path)
	}
}

func TestGetPRContext_RenamedFile(t *testing.T) {
	t.Parallel()

	prResponse := `{
		"number": 1, "title": "PR", "body": "",
		"state": "open", "merged": false,
		"user": {"login": "bob"},
		"head": {"ref": "feat", "sha": "aaa"},
		"base": {"ref": "main", "sha": "bbb"}
	}`

	filesResponse := `[{
		"filename": "new_name.go",
		"previous_filename": "old_name.go",
		"status": "renamed"
	}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		switch {
		case strings.HasSuffix(r.URL.Path, "/files"):
			fmt.Fprint(w, filesResponse)
		case accept == "application/vnd.github.diff":
			fmt.Fprint(w, "")
		default:
			fmt.Fprint(w, prResponse)
		}
	}))
	defer server.Close()

	c := testClient(t, server)
	prCtx, err := c.GetPRContext(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prCtx.Files) != 1 {
		t.Fatalf("Files: got %d, want 1", len(prCtx.Files))
	}
	if prCtx.Files[0].Path != "new_name.go" {
		t.Errorf("Path: got %q", prCtx.Files[0].Path)
	}
	if prCtx.Files[0].OldPath != "old_name.go" {
		t.Errorf("OldPath: got %q", prCtx.Files[0].OldPath)
	}
	if prCtx.Files[0].Status != "renamed" {
		t.Errorf("Status: got %q", prCtx.Files[0].Status)
	}
}

func TestGetPRContext_PRNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.GetPRContext(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  999,
	})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

func TestGetPRContext_Unauthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.GetPRContext(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  1,
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestGetPRContext_UsesDefaultOwner(t *testing.T) {
	t.Parallel()

	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capturedPath == "" {
			capturedPath = r.URL.Path
		}
		accept := r.Header.Get("Accept")
		switch {
		case strings.HasSuffix(r.URL.Path, "/files"):
			fmt.Fprint(w, `[]`)
		case accept == "application/vnd.github.diff":
			fmt.Fprint(w, "")
		default:
			fmt.Fprint(w, `{
				"number": 1, "title": "PR", "body": "",
				"state": "open", "merged": false,
				"user": {"login": "bob"},
				"head": {"ref": "f", "sha": "a"},
				"base": {"ref": "m", "sha": "b"}
			}`)
		}
	}))
	defer server.Close()

	c, err := NewClient(&Config{
		Token:      "ghp_test",
		Owner:      "default-owner",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = c.GetPRContext(context.Background(), platform.PRRequest{
		Repo:     "repo",
		PRNumber: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedPath, "default-owner") {
		t.Errorf("expected default owner in path, got: %q", capturedPath)
	}
}

func TestMapPRState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state  string
		merged bool
		want   string
	}{
		{"open", false, "OPEN"},
		{"closed", false, "CLOSED"},
		{"closed", true, "MERGED"},
		{"unknown", false, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.state+fmt.Sprintf("_merged=%v", tt.merged), func(t *testing.T) {
			t.Parallel()
			got := mapPRState(tt.state, tt.merged)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeFileStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"added", "added"},
		{"removed", "deleted"},
		{"modified", "modified"},
		{"renamed", "renamed"},
		{"copied", "added"},
		{"changed", "modified"},
		{"unchanged", ""},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := normalizeFileStatus(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
