package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dizzyc/crobot/internal/platform"
)

func TestListBotComments_HappyPath(t *testing.T) {
	t.Parallel()

	response := `{
		"values": [
			{
				"id": 12345,
				"content": {"raw": "**warning** | security\n\nMessage\n\n[//]: # \"crobot:fp=abc123\""},
				"inline": {"path": "src/auth.ts", "to": 42},
				"user": {"display_name": "CRoBot"},
				"created_on": "2025-01-15T10:30:00Z"
			},
			{
				"id": 12346,
				"content": {"raw": "Some regular comment without fingerprint"},
				"inline": {"path": "src/auth.ts", "to": 10},
				"user": {"display_name": "Alice"},
				"created_on": "2025-01-15T11:00:00Z"
			},
			{
				"id": 12347,
				"content": {"raw": "Another bot comment\n\n[//]: # \"crobot:fp=def456\""},
				"inline": {"path": "src/main.ts", "to": 5},
				"user": {"display_name": "CRoBot"},
				"created_on": "2025-01-15T12:00:00Z"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	c := testClient(t, server)
	comments, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "ws",
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
	if comments[0].Author != "CRoBot" {
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
			resp := map[string]any{
				"values": []map[string]any{
					{
						"id":         22222,
						"content":    map[string]string{"raw": "msg\n[//]: # \"crobot:fp=page2fp\""},
						"inline":     map[string]any{"path": "b.go", "to": 20},
						"user":       map[string]string{"display_name": "CRoBot"},
						"created_on": "2025-01-15T13:00:00Z",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		resp := map[string]any{
			"values": []map[string]any{
				{
					"id":         11111,
					"content":    map[string]string{"raw": "msg\n[//]: # \"crobot:fp=page1fp\""},
					"inline":     map[string]any{"path": "a.go", "to": 10},
					"user":       map[string]string{"display_name": "CRoBot"},
					"created_on": "2025-01-15T12:00:00Z",
				},
			},
			"next": server.URL + "/2.0/repositories/ws/repo/pullrequests/1/comments?page=2",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := testClient(t, server)
	comments, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "ws",
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
		fmt.Fprint(w, `{"values": []}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	comments, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "ws",
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

	response := `{
		"values": [
			{
				"id": 1,
				"content": {"raw": "Regular comment without fingerprint"},
				"user": {"display_name": "Alice"},
				"created_on": "2025-01-15T10:00:00Z"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	c := testClient(t, server)
	comments, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "ws",
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

func TestListBotComments_CommentWithFromLine(t *testing.T) {
	t.Parallel()

	response := `{
		"values": [
			{
				"id": 100,
				"content": {"raw": "old side comment\n[//]: # \"crobot:fp=oldside\""},
				"inline": {"path": "old.go", "from": 15},
				"user": {"display_name": "CRoBot"},
				"created_on": "2025-01-15T10:00:00Z"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	c := testClient(t, server)
	comments, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "ws",
		Repo:      "repo",
		PRNumber:  1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Line != 15 {
		t.Errorf("Line: got %d, want 15", comments[0].Line)
	}
}

func TestListBotComments_Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.ListBotComments(context.Background(), platform.PRRequest{
		Workspace: "ws",
		Repo:      "repo",
		PRNumber:  1,
	})
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestCreateInlineComment_HappyPath_NewSide(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		var payload bbCommentCreatePayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}

		if payload.Content.Raw != "test body\n[//]: # \"crobot:fp=testfp\"" {
			t.Errorf("body: got %q", payload.Content.Raw)
		}
		if payload.Inline.Path != "src/auth.ts" {
			t.Errorf("inline path: got %q", payload.Inline.Path)
		}
		if payload.Inline.To == nil || *payload.Inline.To != 42 {
			t.Errorf("inline.to: got %v", payload.Inline.To)
		}
		if payload.Inline.From != nil {
			t.Error("inline.from should be nil for new side")
		}

		w.WriteHeader(http.StatusCreated)
		resp := `{
			"id": 99999,
			"content": {"raw": "test body\n[//]: # \"crobot:fp=testfp\""},
			"inline": {"path": "src/auth.ts", "to": 42},
			"user": {"display_name": "CRoBot"},
			"created_on": "2025-01-15T14:00:00Z"
		}`
		fmt.Fprint(w, resp)
	}))
	defer server.Close()

	c := testClient(t, server)
	comment, err := c.CreateInlineComment(context.Background(), platform.PRRequest{
		Workspace: "ws",
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

func TestCreateInlineComment_OldSide(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload bbCommentCreatePayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if payload.Inline.From == nil || *payload.Inline.From != 10 {
			t.Errorf("inline.from: got %v", payload.Inline.From)
		}
		if payload.Inline.To != nil {
			t.Error("inline.to should be nil for old side")
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{
			"id": 88888,
			"content": {"raw": "old side\n[//]: # \"crobot:fp=oldfp\""},
			"inline": {"path": "old.go", "from": 10},
			"user": {"display_name": "CRoBot"},
			"created_on": "2025-01-15T14:00:00Z"
		}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	comment, err := c.CreateInlineComment(context.Background(), platform.PRRequest{
		Workspace: "ws",
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.CreateInlineComment(context.Background(), platform.PRRequest{
		Workspace: "ws",
		Repo:      "repo",
		PRNumber:  1,
	}, platform.InlineComment{
		Path: "file.go",
		Line: 1,
		Side: "new",
		Body: "test",
	})
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestDeleteComment_HappyPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method: got %s, want DELETE", r.Method)
		}
		expected := "/2.0/repositories/ws/repo/pullrequests/42/comments/12345"
		if r.URL.Path != expected {
			t.Errorf("path: got %q, want %q", r.URL.Path, expected)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := testClient(t, server)
	err := c.DeleteComment(context.Background(), platform.PRRequest{
		Workspace: "ws",
		Repo:      "repo",
		PRNumber:  42,
	}, "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteComment_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	c := testClient(t, server)
	err := c.DeleteComment(context.Background(), platform.PRRequest{
		Workspace: "ws",
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
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	c := testClient(t, server)
	err := c.DeleteComment(context.Background(), platform.PRRequest{
		Workspace: "ws",
		Repo:      "repo",
		PRNumber:  42,
	}, "12345")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestExtractFingerprint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "present",
			body: "some comment\n[//]: # \"crobot:fp=abc123\"",
			want: "abc123",
		},
		{
			name: "complex fingerprint",
			body: "comment\n[//]: # \"crobot:fp=src/auth.ts:new:42:token-log\"",
			want: "src/auth.ts:new:42:token-log",
		},
		{
			name: "missing",
			body: "just a regular comment",
			want: "",
		},
		{
			name: "empty body",
			body: "",
			want: "",
		},
		{
			name: "middle of text",
			body: "start\n[//]: # \"crobot:fp=mid123\"\nend",
			want: "mid123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := platform.ExtractFingerprint(tt.body)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
