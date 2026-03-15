package bitbucket

import (
	"context"
	"encoding/json"
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
		"id": 42,
		"title": "Add feature X",
		"description": "Implements feature X",
		"author": {"display_name": "Alice"},
		"source": {"branch": {"name": "feature/x"}, "commit": {"hash": "abc123"}},
		"destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}},
		"state": "OPEN"
	}`

	diffstatResponse := `{
		"values": [
			{
				"new": {"path": "src/auth.ts", "type": "commit_file"},
				"old": {"path": "src/auth.ts", "type": "commit_file"},
				"status": "modified"
			},
			{
				"new": {"path": "src/new_file.ts", "type": "commit_file"},
				"old": {"path": "", "type": ""},
				"status": "added"
			}
		]
	}`

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
		switch {
		case strings.HasSuffix(r.URL.Path, "/diffstat"):
			fmt.Fprint(w, diffstatResponse)
		case strings.HasSuffix(r.URL.Path, "/diff"):
			fmt.Fprint(w, diffResponse)
		case strings.Contains(r.URL.Path, "/pullrequests/42"):
			fmt.Fprint(w, prResponse)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := testClient(t, server)
	ctx := context.Background()

	prCtx, err := c.GetPRContext(ctx, platform.PRRequest{
		Workspace: "myteam",
		Repo:      "my-service",
		PRNumber:  42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prCtx.ID != 42 {
		t.Errorf("ID: got %d, want 42", prCtx.ID)
	}
	if prCtx.Title != "Add feature X" {
		t.Errorf("Title: got %q, want %q", prCtx.Title, "Add feature X")
	}
	if prCtx.Description != "Implements feature X" {
		t.Errorf("Description: got %q", prCtx.Description)
	}
	if prCtx.Author != "Alice" {
		t.Errorf("Author: got %q, want %q", prCtx.Author, "Alice")
	}
	if prCtx.SourceBranch != "feature/x" {
		t.Errorf("SourceBranch: got %q", prCtx.SourceBranch)
	}
	if prCtx.TargetBranch != "main" {
		t.Errorf("TargetBranch: got %q", prCtx.TargetBranch)
	}
	if prCtx.State != "OPEN" {
		t.Errorf("State: got %q", prCtx.State)
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

func TestGetPRContext_DiffstatPagination(t *testing.T) {
	t.Parallel()

	prResponse := `{
		"id": 1,
		"title": "PR",
		"description": "",
		"author": {"display_name": "Bob"},
		"source": {"branch": {"name": "feat"}, "commit": {"hash": "aaa"}},
		"destination": {"branch": {"name": "main"}, "commit": {"hash": "bbb"}},
		"state": "OPEN"
	}`

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/2.0/repositories/ws/repo/pullrequests/1/diffstat" && r.URL.Query().Get("page") == "":
			// First page
			resp := map[string]any{
				"values": []map[string]any{
					{
						"new":    map[string]string{"path": "file1.go", "type": "commit_file"},
						"old":    map[string]string{"path": "file1.go", "type": "commit_file"},
						"status": "modified",
					},
				},
				"next": server.URL + "/2.0/repositories/ws/repo/pullrequests/1/diffstat?page=2",
			}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Query().Get("page") == "2":
			// Second page (no next)
			resp := map[string]any{
				"values": []map[string]any{
					{
						"new":    map[string]string{"path": "file2.go", "type": "commit_file"},
						"old":    map[string]string{"path": "", "type": ""},
						"status": "added",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/diff"):
			fmt.Fprint(w, "")
		case strings.Contains(r.URL.Path, "/pullrequests/1"):
			fmt.Fprint(w, prResponse)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := testClient(t, server)
	prCtx, err := c.GetPRContext(context.Background(), platform.PRRequest{
		Workspace: "ws",
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

func TestGetPRContext_DiffstatRename(t *testing.T) {
	t.Parallel()

	prResponse := `{
		"id": 1,
		"title": "PR",
		"description": "",
		"author": {"display_name": "Bob"},
		"source": {"branch": {"name": "feat"}, "commit": {"hash": "aaa"}},
		"destination": {"branch": {"name": "main"}, "commit": {"hash": "bbb"}},
		"state": "OPEN"
	}`

	diffstatResponse := `{
		"values": [
			{
				"new": {"path": "new_name.go", "type": "commit_file"},
				"old": {"path": "old_name.go", "type": "commit_file"},
				"status": "renamed"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/diffstat"):
			fmt.Fprint(w, diffstatResponse)
		case strings.HasSuffix(r.URL.Path, "/diff"):
			fmt.Fprint(w, "")
		default:
			fmt.Fprint(w, prResponse)
		}
	}))
	defer server.Close()

	c := testClient(t, server)
	prCtx, err := c.GetPRContext(context.Background(), platform.PRRequest{
		Workspace: "ws",
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

func TestGetPRContext_DiffstatDeletedFile(t *testing.T) {
	t.Parallel()

	prResponse := `{
		"id": 1,
		"title": "PR",
		"description": "",
		"author": {"display_name": "Bob"},
		"source": {"branch": {"name": "feat"}, "commit": {"hash": "aaa"}},
		"destination": {"branch": {"name": "main"}, "commit": {"hash": "bbb"}},
		"state": "OPEN"
	}`

	diffstatResponse := `{
		"values": [
			{
				"new": {"path": "", "type": ""},
				"old": {"path": "deleted.go", "type": "commit_file"},
				"status": "removed"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/diffstat"):
			fmt.Fprint(w, diffstatResponse)
		case strings.HasSuffix(r.URL.Path, "/diff"):
			fmt.Fprint(w, "")
		default:
			fmt.Fprint(w, prResponse)
		}
	}))
	defer server.Close()

	c := testClient(t, server)
	prCtx, err := c.GetPRContext(context.Background(), platform.PRRequest{
		Workspace: "ws",
		Repo:      "repo",
		PRNumber:  1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prCtx.Files) != 1 {
		t.Fatalf("Files: got %d, want 1", len(prCtx.Files))
	}
	if prCtx.Files[0].Path != "deleted.go" {
		t.Errorf("Path: got %q", prCtx.Files[0].Path)
	}
	if prCtx.Files[0].Status != "deleted" {
		t.Errorf("Status: got %q", prCtx.Files[0].Status)
	}
}

func TestGetPRContext_PRNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.GetPRContext(context.Background(), platform.PRRequest{
		Workspace: "ws",
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
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.GetPRContext(context.Background(), platform.PRRequest{
		Workspace: "ws",
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

func TestGetPRContext_UsesDefaultWorkspace(t *testing.T) {
	t.Parallel()

	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capturedPath == "" {
			capturedPath = r.URL.Path
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/diffstat"):
			fmt.Fprint(w, `{"values":[]}`)
		case strings.HasSuffix(r.URL.Path, "/diff"):
			fmt.Fprint(w, "")
		default:
			fmt.Fprint(w, `{
				"id": 1, "title": "PR", "description": "",
				"author": {"display_name": "Bob"},
				"source": {"branch": {"name": "f"}, "commit": {"hash": "a"}},
				"destination": {"branch": {"name": "m"}, "commit": {"hash": "b"}},
				"state": "OPEN"
			}`)
		}
	}))
	defer server.Close()

	c, err := NewClient(&Config{
		User:       "testuser",
		Token:      "testtoken",
		BaseURL:    server.URL,
		Workspace:  "default-ws",
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

	if !strings.Contains(capturedPath, "default-ws") {
		t.Errorf("expected default workspace in path, got: %q", capturedPath)
	}
}

func TestNormalizeDiffStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"added", "added"},
		{"removed", "deleted"},
		{"modified", "modified"},
		{"renamed", "renamed"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := normalizeDiffStatus(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
