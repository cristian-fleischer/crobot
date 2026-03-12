package bitbucket

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dizzyc/crobot/internal/platform"
)

func TestGetFileContent_HappyPath(t *testing.T) {
	t.Parallel()

	fileContent := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/ws/repo/src/abc123/main.go" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		fmt.Fprint(w, fileContent)
	}))
	defer server.Close()

	c := testClient(t, server)

	data, err := c.GetFileContent(context.Background(), platform.FileRequest{
		Workspace: "ws",
		Repo:      "repo",
		Commit:    "abc123",
		Path:      "main.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != fileContent {
		t.Errorf("content mismatch:\n  got  %q\n  want %q", string(data), fileContent)
	}
}

func TestGetFileContent_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.GetFileContent(context.Background(), platform.FileRequest{
		Workspace: "ws",
		Repo:      "repo",
		Commit:    "abc123",
		Path:      "nonexistent.go",
	})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

func TestGetFileContent_Unauthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.GetFileContent(context.Background(), platform.FileRequest{
		Workspace: "ws",
		Repo:      "repo",
		Commit:    "abc123",
		Path:      "secret.go",
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestGetFileContent_Forbidden(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.GetFileContent(context.Background(), platform.FileRequest{
		Workspace: "ws",
		Repo:      "repo",
		Commit:    "abc123",
		Path:      "restricted.go",
	})
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 in error, got: %v", err)
	}
}

func TestGetFileContent_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.GetFileContent(context.Background(), platform.FileRequest{
		Workspace: "ws",
		Repo:      "repo",
		Commit:    "abc123",
		Path:      "file.go",
	})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got: %v", err)
	}
}

func TestGetFileContent_BinaryFile(t *testing.T) {
	t.Parallel()

	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(binaryData)
	}))
	defer server.Close()

	c := testClient(t, server)
	data, err := c.GetFileContent(context.Background(), platform.FileRequest{
		Workspace: "ws",
		Repo:      "repo",
		Commit:    "abc123",
		Path:      "image.png",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != len(binaryData) {
		t.Errorf("length mismatch: got %d, want %d", len(data), len(binaryData))
	}
}

func TestGetFileContent_UsesDefaultWorkspace(t *testing.T) {
	t.Parallel()

	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		fmt.Fprint(w, "content")
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
		t.Fatalf("creating client: %v", err)
	}

	_, err = c.GetFileContent(context.Background(), platform.FileRequest{
		Repo:   "repo",
		Commit: "abc123",
		Path:   "file.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedPath, "default-ws") {
		t.Errorf("expected default workspace in path, got: %q", capturedPath)
	}
}

func TestGetFileContent_NestedPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/2.0/repositories/ws/repo/src/abc123/pkg/internal/handler.go"
		if r.URL.Path != expected {
			t.Errorf("path: got %q, want %q", r.URL.Path, expected)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		fmt.Fprint(w, "package internal")
	}))
	defer server.Close()

	c := testClient(t, server)
	data, err := c.GetFileContent(context.Background(), platform.FileRequest{
		Workspace: "ws",
		Repo:      "repo",
		Commit:    "abc123",
		Path:      "pkg/internal/handler.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "package internal" {
		t.Errorf("content: got %q", string(data))
	}
}

func TestGetFileContent_LargeFile(t *testing.T) {
	t.Parallel()

	largeContent := strings.Repeat("a", 1024*1024) // 1MB

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, largeContent)
	}))
	defer server.Close()

	c := testClient(t, server)
	data, err := c.GetFileContent(context.Background(), platform.FileRequest{
		Workspace: "ws",
		Repo:      "repo",
		Commit:    "abc123",
		Path:      "big.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != len(largeContent) {
		t.Errorf("length: got %d, want %d", len(data), len(largeContent))
	}
}
