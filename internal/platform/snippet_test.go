package platform

import (
	"strings"
	"testing"
)

func TestExtractSnippet(t *testing.T) {
	t.Parallel()

	// A small file with 10 numbered lines for easy reasoning.
	tenLines := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"

	tests := []struct {
		name        string
		content     []byte
		path        string
		commit      string
		line        int
		contextSize int
		wantStart   int
		wantEnd     int
		wantContent string // substring that must appear in Content
		wantErr     bool
		errSubstr   string
	}{
		{
			name:        "line in middle with context",
			content:     []byte(tenLines),
			path:        "file.go",
			commit:      "abc",
			line:        5,
			contextSize: 2,
			wantStart:   3,
			wantEnd:     7,
			wantContent: "line5",
		},
		{
			name:        "line at start clamps to 1",
			content:     []byte(tenLines),
			path:        "file.go",
			commit:      "abc",
			line:        1,
			contextSize: 3,
			wantStart:   1,
			wantEnd:     4,
			wantContent: "line1",
		},
		{
			name:        "line at end clamps to file length",
			content:     []byte(tenLines),
			path:        "file.go",
			commit:      "abc",
			line:        10,
			contextSize: 3,
			wantStart:   7,
			wantEnd:     10,
			wantContent: "line10",
		},
		{
			name:        "context zero returns only the target line",
			content:     []byte(tenLines),
			path:        "file.go",
			commit:      "abc",
			line:        5,
			contextSize: 0,
			wantStart:   5,
			wantEnd:     5,
			wantContent: "line5",
		},
		{
			name:        "context larger than file covers whole file",
			content:     []byte(tenLines),
			path:        "file.go",
			commit:      "abc",
			line:        5,
			contextSize: 100,
			wantStart:   1,
			wantEnd:     10,
			wantContent: "line1",
		},
		{
			name:        "no trailing newline still works",
			content:     []byte("alpha\nbeta\ngamma"),
			path:        "file.go",
			commit:      "abc",
			line:        2,
			contextSize: 1,
			wantStart:   1,
			wantEnd:     3,
			wantContent: "beta",
		},
		{
			name:      "empty file returns error",
			content:   []byte(""),
			path:      "empty.go",
			commit:    "abc",
			line:      1,
			wantErr:   true,
			errSubstr: "empty",
		},
		{
			name:      "line beyond file length returns error",
			content:   []byte(tenLines),
			path:      "file.go",
			commit:    "abc",
			line:      99,
			wantErr:   true,
			errSubstr: "out of range",
		},
		{
			name:        "single-line file",
			content:     []byte("only-line\n"),
			path:        "single.go",
			commit:      "abc",
			line:        1,
			contextSize: 5,
			wantStart:   1,
			wantEnd:     1,
			wantContent: "only-line",
		},
		{
			name:        "single-line file without trailing newline",
			content:     []byte("only-line"),
			path:        "single.go",
			commit:      "abc",
			line:        1,
			contextSize: 0,
			wantStart:   1,
			wantEnd:     1,
			wantContent: "only-line",
		},
		{
			name:        "line 1 with large context",
			content:     []byte(tenLines),
			path:        "file.go",
			commit:      "abc",
			line:        1,
			contextSize: 50,
			wantStart:   1,
			wantEnd:     10,
			wantContent: "line1",
		},
		{
			name:        "last line with large context",
			content:     []byte(tenLines),
			path:        "file.go",
			commit:      "abc",
			line:        10,
			contextSize: 50,
			wantStart:   1,
			wantEnd:     10,
			wantContent: "line10",
		},
		{
			name:        "newline-only file has one empty line",
			content:     []byte("\n"),
			path:        "blank.go",
			commit:      "abc",
			line:        1,
			contextSize: 0,
			wantStart:   1,
			wantEnd:     1,
			wantContent: "",
		},
		{
			name:      "line exactly at boundary (line == len)",
			content:   []byte(tenLines),
			path:      "file.go",
			commit:    "abc",
			line:      10,
			contextSize: 0,
			wantStart:   10,
			wantEnd:     10,
			wantContent: "line10",
		},
		{
			name:      "line one past boundary returns error",
			content:   []byte(tenLines),
			path:      "file.go",
			commit:    "abc",
			line:      11,
			wantErr:   true,
			errSubstr: "out of range",
		},
		{
			name:        "two-line file first line",
			content:     []byte("first\nsecond\n"),
			path:        "two.go",
			commit:      "abc",
			line:        1,
			contextSize: 1,
			wantStart:   1,
			wantEnd:     2,
			wantContent: "first",
		},
		{
			name:        "two-line file second line",
			content:     []byte("first\nsecond\n"),
			path:        "two.go",
			commit:      "abc",
			line:        2,
			contextSize: 1,
			wantStart:   1,
			wantEnd:     2,
			wantContent: "second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ExtractSnippet(tt.content, tt.path, tt.commit, tt.line, tt.contextSize)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Path != tt.path {
				t.Errorf("Path = %q, want %q", got.Path, tt.path)
			}
			if got.Commit != tt.commit {
				t.Errorf("Commit = %q, want %q", got.Commit, tt.commit)
			}
			if got.StartLine != tt.wantStart {
				t.Errorf("StartLine = %d, want %d", got.StartLine, tt.wantStart)
			}
			if got.EndLine != tt.wantEnd {
				t.Errorf("EndLine = %d, want %d", got.EndLine, tt.wantEnd)
			}
			if tt.wantContent != "" && !strings.Contains(got.Content, tt.wantContent) {
				t.Errorf("Content %q does not contain %q", got.Content, tt.wantContent)
			}
		})
	}
}

// TestExtractSnippet_ExactContent verifies the exact content returned for
// specific scenarios, not just substring matching.
func TestExtractSnippet_ExactContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      []byte
		line         int
		contextSize  int
		wantExact    string
	}{
		{
			name:        "context zero returns exactly the target line",
			content:     []byte("aaa\nbbb\nccc\nddd\neee\n"),
			line:        3,
			contextSize: 0,
			wantExact:   "ccc",
		},
		{
			name:        "context 1 returns three lines joined",
			content:     []byte("aaa\nbbb\nccc\nddd\neee\n"),
			line:        3,
			contextSize: 1,
			wantExact:   "bbb\nccc\nddd",
		},
		{
			name:        "single-line file returns just that line",
			content:     []byte("only\n"),
			line:        1,
			contextSize: 10,
			wantExact:   "only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ExtractSnippet(tt.content, "test.go", "abc", tt.line, tt.contextSize)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Content != tt.wantExact {
				t.Errorf("Content = %q, want exact %q", got.Content, tt.wantExact)
			}
		})
	}
}
