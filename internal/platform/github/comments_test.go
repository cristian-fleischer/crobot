package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

func TestListBotComments_HappyPath(t *testing.T) {
	t.Parallel()

	response := `[
		{
			"id": 12345,
			"body": "**warning** | security\n\nMessage\n\n[//]: # \"crobot:fp=abc123\"",
			"path": "src/auth.ts",
			"line": 42,
			"side": "RIGHT",
			"commit_id": "abc123",
			"user": {"login": "crobot[bot]", "id": 100, "type": "Bot"},
			"created_at": "2025-01-15T10:30:00Z"
		},
		{
			"id": 12346,
			"body": "Some regular comment without fingerprint",
			"path": "src/auth.ts",
			"line": 10,
			"side": "RIGHT",
			"commit_id": "abc123",
			"user": {"login": "alice", "id": 200, "type": "User"},
			"created_at": "2025-01-15T11:00:00Z"
		},
		{
			"id": 12347,
			"body": "Another bot comment\n\n[//]: # \"crobot:fp=def456\"",
			"path": "src/main.ts",
			"line": 5,
			"side": "LEFT",
			"commit_id": "abc123",
			"user": {"login": "crobot[bot]", "id": 100, "type": "Bot"},
			"created_at": "2025-01-15T12:00:00Z"
		}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	c := testClient(t, server)
	comments, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only return comments with fingerprints
	if len(comments) != 2 {
		t.Fatalf("expected 2 bot comments, got %d", len(comments))
	}

	// First bot comment
	if comments[0].ID != "12345" {
		t.Errorf("ID: got %q, want %q", comments[0].ID, "12345")
	}
	if comments[0].Fingerprint != "abc123" {
		t.Errorf("Fingerprint: got %q, want %q", comments[0].Fingerprint, "abc123")
	}
	if comments[0].Path != "src/auth.ts" {
		t.Errorf("Path: got %q", comments[0].Path)
	}
	if comments[0].Line != 42 {
		t.Errorf("Line: got %d, want 42", comments[0].Line)
	}
	if !comments[0].IsBot {
		t.Error("expected IsBot=true")
	}
	if comments[0].Author != "crobot[bot]" {
		t.Errorf("Author: got %q", comments[0].Author)
	}
	if comments[0].CreatedAt != "2025-01-15T10:30:00Z" {
		t.Errorf("CreatedAt: got %q", comments[0].CreatedAt)
	}

	// Second bot comment
	if comments[1].ID != "12347" {
		t.Errorf("ID: got %q, want %q", comments[1].ID, "12347")
	}
	if comments[1].Fingerprint != "def456" {
		t.Errorf("Fingerprint: got %q, want %q", comments[1].Fingerprint, "def456")
	}
}

func TestListBotComments_Pagination(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `[{
				"id": 22222,
				"body": "msg\n[//]: # \"crobot:fp=page2fp\"",
				"path": "b.go",
				"line": 20,
				"side": "RIGHT",
				"commit_id": "sha",
				"user": {"login": "bot", "type": "Bot"},
				"created_at": "2025-01-15T13:00:00Z"
			}]`)
			return
		}

		w.Header().Set("Link", fmt.Sprintf(`<%s/repos/owner/repo/pulls/1/comments?page=2&per_page=100>; rel="next"`, server.URL))
		fmt.Fprint(w, `[{
			"id": 11111,
			"body": "msg\n[//]: # \"crobot:fp=page1fp\"",
			"path": "a.go",
			"line": 10,
			"side": "RIGHT",
			"commit_id": "sha",
			"user": {"login": "bot", "type": "Bot"},
			"created_at": "2025-01-15T12:00:00Z"
		}]`)
	}))
	defer server.Close()

	c := testClient(t, server)
	comments, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments across pages, got %d", len(comments))
	}
	if comments[0].Fingerprint != "page1fp" {
		t.Errorf("first comment fingerprint: got %q", comments[0].Fingerprint)
	}
	if comments[1].Fingerprint != "page2fp" {
		t.Errorf("second comment fingerprint: got %q", comments[1].Fingerprint)
	}
}

func TestListBotComments_NoComments(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[]`)
	}))
	defer server.Close()

	c := testClient(t, server)
	comments, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(comments))
	}
}

func TestListBotComments_NoBotComments(t *testing.T) {
	t.Parallel()

	response := `[
		{
			"id": 1,
			"body": "Regular comment without fingerprint",
			"path": "file.go",
			"line": 5,
			"side": "RIGHT",
			"user": {"login": "alice", "type": "User"},
			"created_at": "2025-01-15T10:00:00Z"
		}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	c := testClient(t, server)
	comments, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected 0 bot comments, got %d", len(comments))
	}
}

func TestListBotComments_Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  1,
	})
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestCreateInlineComment_HappyPath_RightSide(t *testing.T) {
	t.Parallel()

	prFetched := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The GET to /repos/.../pulls/42 fetches the head commit.
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pulls/42") && !strings.Contains(r.URL.Path, "/comments") {
			prFetched = true
			fmt.Fprint(w, `{
				"number": 42, "title": "PR", "body": "",
				"state": "open", "merged": false,
				"user": {"login": "bob"},
				"head": {"ref": "feat", "sha": "head-sha-123"},
				"base": {"ref": "main", "sha": "base-sha"}
			}`)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, _ := io.ReadAll(r.Body)
		var payload ghCreateComment
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}

		if payload.CommitID != "head-sha-123" {
			t.Errorf("commit_id: got %q, want %q", payload.CommitID, "head-sha-123")
		}
		if payload.Path != "src/auth.ts" {
			t.Errorf("path: got %q", payload.Path)
		}
		if payload.Line != 42 {
			t.Errorf("line: got %d", payload.Line)
		}
		if payload.Side != "RIGHT" {
			t.Errorf("side: got %q, want RIGHT", payload.Side)
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{
			"id": 99999,
			"body": "test body\n[//]: # \"crobot:fp=testfp\"",
			"path": "src/auth.ts",
			"line": 42,
			"side": "RIGHT",
			"commit_id": "head-sha-123",
			"user": {"login": "crobot[bot]", "type": "Bot"},
			"created_at": "2025-01-15T14:00:00Z"
		}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	comment, err := c.CreateInlineComment(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  42,
	}, platform.InlineComment{
		Path:        "src/auth.ts",
		Line:        42,
		Side:        "new",
		Body:        "test body\n[//]: # \"crobot:fp=testfp\"",
		Fingerprint: "testfp",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prFetched {
		t.Error("expected PR to be fetched for head commit")
	}
	if comment.ID != "99999" {
		t.Errorf("ID: got %q", comment.ID)
	}
	if comment.Path != "src/auth.ts" {
		t.Errorf("Path: got %q", comment.Path)
	}
	if comment.Line != 42 {
		t.Errorf("Line: got %d", comment.Line)
	}
	if comment.Fingerprint != "testfp" {
		t.Errorf("Fingerprint: got %q", comment.Fingerprint)
	}
}

func TestCreateInlineComment_LeftSide(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Head commit fetch
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{
				"number": 1, "title": "PR", "body": "",
				"state": "open", "merged": false,
				"user": {"login": "bob"},
				"head": {"ref": "feat", "sha": "sha123"},
				"base": {"ref": "main", "sha": "base"}
			}`)
			return
		}

		body, _ := io.ReadAll(r.Body)
		var payload ghCreateComment
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if payload.Side != "LEFT" {
			t.Errorf("side: got %q, want LEFT", payload.Side)
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{
			"id": 88888,
			"body": "old side\n[//]: # \"crobot:fp=oldfp\"",
			"path": "old.go",
			"line": 10,
			"side": "LEFT",
			"commit_id": "sha123",
			"user": {"login": "crobot[bot]", "type": "Bot"},
			"created_at": "2025-01-15T14:00:00Z"
		}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	comment, err := c.CreateInlineComment(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  1,
	}, platform.InlineComment{
		Path: "old.go",
		Line: 10,
		Side: "old",
		Body: "old side\n[//]: # \"crobot:fp=oldfp\"",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comment.Line != 10 {
		t.Errorf("Line: got %d", comment.Line)
	}
}

func TestCreateInlineComment_Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Head commit fetch succeeds
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{
				"number": 1, "title": "PR", "body": "",
				"state": "open", "merged": false,
				"user": {"login": "bob"},
				"head": {"ref": "feat", "sha": "sha123"},
				"base": {"ref": "main", "sha": "base"}
			}`)
			return
		}
		http.Error(w, `{"message":"Validation Failed"}`, http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.CreateInlineComment(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  1,
	}, platform.InlineComment{
		Path: "file.go",
		Line: 1,
		Side: "new",
		Body: "test",
	})
	if err == nil {
		t.Fatal("expected error for 422")
	}
}

func TestCreateInlineComment_HeadCommitFetchFails(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.CreateInlineComment(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  999,
	}, platform.InlineComment{
		Path: "file.go",
		Line: 1,
		Side: "new",
		Body: "test",
	})
	if err == nil {
		t.Fatal("expected error when head commit fetch fails")
	}
}

func TestDeleteComment_HappyPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method: got %s, want DELETE", r.Method)
		}
		// GitHub delete path does NOT include PR number
		expected := "/repos/testowner/repo/pulls/comments/12345"
		if r.URL.Path != expected {
			t.Errorf("path: got %q, want %q", r.URL.Path, expected)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := testClient(t, server)
	err := c.DeleteComment(context.Background(), platform.PRRequest{
		Workspace: "testowner",
		Repo:      "repo",
		PRNumber:  42, // Should not appear in the delete path
	}, "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteComment_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	c := testClient(t, server)
	err := c.DeleteComment(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  42,
	}, "99999")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

func TestDeleteComment_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Internal Server Error"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	c := testClient(t, server)
	err := c.DeleteComment(context.Background(), platform.PRRequest{
		Workspace: "owner",
		Repo:      "repo",
		PRNumber:  42,
	}, "12345")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestDeleteComment_UsesDefaultOwner(t *testing.T) {
	t.Parallel()

	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c, err := NewClient(&Config{
		Token:      "ghp_test",
		Owner:      "default-org",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	err = c.DeleteComment(context.Background(), platform.PRRequest{
		Repo:     "repo",
		PRNumber: 1,
	}, "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedPath, "default-org") {
		t.Errorf("expected default owner in path, got: %q", capturedPath)
	}
}
