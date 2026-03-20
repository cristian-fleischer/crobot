package agent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFSHandlerReadTextFile(t *testing.T) {
	t.Parallel()

	// Get the current HEAD commit.
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Skip("not in a git repository")
	}
	commit := strings.TrimSpace(string(out))

	// Get the repo root.
	rootOut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not in a git repository")
	}
	repoDir := strings.TrimSpace(string(rootOut))

	handler, err := NewFSHandler(commit, repoDir)
	if err != nil {
		t.Fatalf("NewFSHandler: %v", err)
	}

	// Read a file that should exist (go.mod).
	params, _ := json.Marshal(readTextFileParams{Path: "go.mod"})
	result, err := handler.HandleRequest(context.Background(), "fs/read_text_file", params)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	data, _ := json.Marshal(result)
	var resp map[string]string
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !strings.Contains(resp["content"], "module") {
		t.Errorf("expected go.mod content to contain 'module', got: %s", resp["content"])
	}
}

func TestFSHandlerWriteDenied(t *testing.T) {
	t.Parallel()

	handler, err := NewFSHandler("abc123", "/tmp")
	if err != nil {
		t.Fatalf("NewFSHandler: %v", err)
	}

	params, _ := json.Marshal(map[string]string{"path": "test.go", "content": "package main"})
	_, err = handler.HandleRequest(context.Background(), "fs/write_text_file", params)
	if err == nil {
		t.Fatal("expected error for write operation, got nil")
	}
	if !strings.Contains(err.Error(), "not permitted") {
		t.Errorf("expected 'not permitted' error, got: %v", err)
	}
}

func TestFSHandlerTerminalDenied(t *testing.T) {
	t.Parallel()

	handler, err := NewFSHandler("abc123", "/tmp")
	if err != nil {
		t.Fatalf("NewFSHandler: %v", err)
	}

	params, _ := json.Marshal(map[string]string{"command": "ls"})
	_, err = handler.HandleRequest(context.Background(), "terminal/run", params)
	if err == nil {
		t.Fatal("expected error for terminal operation, got nil")
	}
	if !strings.Contains(err.Error(), "not permitted") {
		t.Errorf("expected 'not permitted' error, got: %v", err)
	}
}

func TestFSHandlerUnknownMethod(t *testing.T) {
	t.Parallel()

	handler, err := NewFSHandler("abc123", "/tmp")
	if err != nil {
		t.Fatalf("NewFSHandler: %v", err)
	}

	params, _ := json.Marshal(map[string]string{})
	_, err = handler.HandleRequest(context.Background(), "fs/unknown", params)
	if err == nil {
		t.Fatal("expected error for unknown method, got nil")
	}
	if !strings.Contains(err.Error(), "unknown method") {
		t.Errorf("expected 'unknown method' error, got: %v", err)
	}
}

func TestFSHandlerEmptyPath(t *testing.T) {
	t.Parallel()

	handler, err := NewFSHandler("abc123", "/tmp")
	if err != nil {
		t.Fatalf("NewFSHandler: %v", err)
	}

	params, _ := json.Marshal(readTextFileParams{Path: ""})
	_, err = handler.HandleRequest(context.Background(), "fs/read_text_file", params)
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("expected 'must not be empty' error, got: %v", err)
	}
}

func TestFSHandlerDirectoryTraversal(t *testing.T) {
	t.Parallel()

	handler, err := NewFSHandler("abc123", "/tmp")
	if err != nil {
		t.Fatalf("NewFSHandler: %v", err)
	}

	params, _ := json.Marshal(readTextFileParams{Path: "../../../etc/passwd"})
	_, err = handler.HandleRequest(context.Background(), "fs/read_text_file", params)
	if err == nil {
		t.Fatal("expected error for directory traversal, got nil")
	}
	if !strings.Contains(err.Error(), "..") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

func TestFSHandlerNonexistentFile(t *testing.T) {
	t.Parallel()

	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Skip("not in a git repository")
	}
	commit := strings.TrimSpace(string(out))

	rootOut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not in a git repository")
	}
	repoDir := strings.TrimSpace(string(rootOut))

	handler, err := NewFSHandler(commit, repoDir)
	if err != nil {
		t.Fatalf("NewFSHandler: %v", err)
	}

	params, _ := json.Marshal(readTextFileParams{Path: "nonexistent_file_xyz.go"})
	_, err = handler.HandleRequest(context.Background(), "fs/read_text_file", params)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestNewFSHandler(t *testing.T) {
	t.Parallel()

	handler, err := NewFSHandler("abc123", "/repo")
	if err != nil {
		t.Fatalf("NewFSHandler: %v", err)
	}
	if handler.headCommit != "abc123" {
		t.Errorf("expected headCommit=abc123, got %q", handler.headCommit)
	}
	if handler.repoDir != "/repo" {
		t.Errorf("expected repoDir=/repo, got %q", handler.repoDir)
	}
}

func TestNewFSHandlerInvalidCommit(t *testing.T) {
	t.Parallel()

	_, err := NewFSHandler("not-a-hash!", "/repo")
	if err == nil {
		t.Fatal("expected error for invalid commit hash, got nil")
	}
	if !strings.Contains(err.Error(), "invalid commit hash") {
		t.Errorf("expected 'invalid commit hash' error, got: %v", err)
	}
}

func TestFSHandlerDiskFallback(t *testing.T) {
	t.Parallel()

	// Create a temp dir to act as repo root with a .crobot/ dir.
	repoDir := t.TempDir()
	diffDir := filepath.Join(repoDir, ".crobot", "diffs-123")
	os.MkdirAll(filepath.Join(diffDir, "src"), 0o755)
	os.WriteFile(filepath.Join(diffDir, "src", "auth.go"), []byte("diff content here"), 0o644)
	os.WriteFile(filepath.Join(diffDir, ".crobot-index.md"), []byte("# Index\n"), 0o644)

	handler, err := NewFSHandler("abcd1234", repoDir)
	if err != nil {
		t.Fatalf("NewFSHandler: %v", err)
	}

	// Read a file under .crobot/ -- should use disk, not git show.
	params, _ := json.Marshal(readTextFileParams{Path: ".crobot/diffs-123/src/auth.go"})
	result, err := handler.HandleRequest(context.Background(), "fs/read_text_file", params)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	data, _ := json.Marshal(result)
	var resp map[string]string
	json.Unmarshal(data, &resp)

	if resp["content"] != "diff content here" {
		t.Errorf("expected disk content, got: %q", resp["content"])
	}

	// Read the index file.
	params, _ = json.Marshal(readTextFileParams{Path: ".crobot/diffs-123/.crobot-index.md"})
	result, err = handler.HandleRequest(context.Background(), "fs/read_text_file", params)
	if err != nil {
		t.Fatalf("HandleRequest for index: %v", err)
	}

	data, _ = json.Marshal(result)
	json.Unmarshal(data, &resp)

	if resp["content"] != "# Index\n" {
		t.Errorf("expected index content, got: %q", resp["content"])
	}
}

func TestFSHandlerDiskFallback_NotFound(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	handler, err := NewFSHandler("abcd1234", repoDir)
	if err != nil {
		t.Fatalf("NewFSHandler: %v", err)
	}

	params, _ := json.Marshal(readTextFileParams{Path: ".crobot/diffs-123/nonexistent.go"})
	_, err = handler.HandleRequest(context.Background(), "fs/read_text_file", params)
	if err == nil {
		t.Fatal("expected error for nonexistent .crobot file, got nil")
	}
}

// TestNewFSHandler_CommitHashValidation exercises the commit hash regex with
// various valid and invalid inputs.
func TestNewFSHandler_CommitHashValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		commit  string
		wantErr bool
	}{
		// Valid hashes
		{name: "full 40-char hash", commit: "abcdef1234567890abcdef1234567890abcdef12", wantErr: false},
		{name: "short 7-char hash", commit: "abc1234", wantErr: false},
		{name: "minimum 4-char hash", commit: "abcd", wantErr: false},
		{name: "all digits", commit: "1234567890", wantErr: false},
		{name: "all lowercase hex letters", commit: "abcdef", wantErr: false},
		{name: "mixed hex", commit: "a1b2c3d4e5f6", wantErr: false},

		// Invalid hashes
		{name: "uppercase hex rejected", commit: "ABCDEF", wantErr: true},
		{name: "mixed case rejected", commit: "aBcDeF", wantErr: true},
		{name: "too short 3 chars", commit: "abc", wantErr: true},
		{name: "too short 1 char", commit: "a", wantErr: true},
		{name: "empty string", commit: "", wantErr: true},
		{name: "too long 41 chars", commit: "abcdef1234567890abcdef1234567890abcdef123", wantErr: true},
		{name: "contains non-hex letter g", commit: "abcdefg", wantErr: true},
		{name: "contains dash", commit: "abc-123", wantErr: true},
		{name: "contains space", commit: "abc 123", wantErr: true},
		{name: "contains special chars", commit: "abc!@#$", wantErr: true},
		{name: "hex with leading space", commit: " abcdef", wantErr: true},
		{name: "hex with trailing space", commit: "abcdef ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, err := NewFSHandler(tt.commit, "/repo")

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewFSHandler(%q) expected error, got nil", tt.commit)
				}
				if err != nil && !strings.Contains(err.Error(), "invalid commit hash") {
					t.Errorf("NewFSHandler(%q) error = %q, want 'invalid commit hash'", tt.commit, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("NewFSHandler(%q) unexpected error: %v", tt.commit, err)
			}
			if handler.headCommit != tt.commit {
				t.Errorf("headCommit = %q, want %q", handler.headCommit, tt.commit)
			}
		})
	}
}
