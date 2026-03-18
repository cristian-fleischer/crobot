package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

func TestExtractDiffContext(t *testing.T) {
	hunks := []platform.DiffHunk{
		{
			Path:     "main.go",
			OldStart: 10,
			OldLines: 7,
			NewStart: 10,
			NewLines: 8,
			Body:     " line10\n line11\n line12\n-old13\n+new13\n+new14\n line15\n line16\n",
		},
	}

	tests := []struct {
		name       string
		line       int
		side       string
		wantTarget string
		wantCount  int
	}{
		{
			name:       "new side line in middle",
			line:       13,
			side:       "new",
			wantTarget: "new13",
			wantCount:  7, // 3 before + target + 3 after (all lines of hunk)
		},
		{
			name:       "old side deleted line",
			line:       13,
			side:       "old",
			wantTarget: "old13",
			wantCount:  7, // context window
		},
		{
			name:       "context line",
			line:       10,
			side:       "new",
			wantTarget: "line10",
			wantCount:  4, // target + 3 after (at start of hunk)
		},
		{
			name:      "line not in any hunk",
			line:      99,
			side:      "new",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := extractDiffContext(tt.line, tt.side, "main.go", hunks, 3)
			if len(lines) != tt.wantCount {
				t.Errorf("got %d lines, want %d", len(lines), tt.wantCount)
				for i, l := range lines {
					t.Logf("  [%d] old=%d new=%d prefix=%q text=%q target=%v",
						i, l.OldNum, l.NewNum, l.Prefix, l.Text, l.IsTarget)
				}
				return
			}
			if tt.wantTarget != "" {
				found := false
				for _, l := range lines {
					if l.IsTarget && l.Text == tt.wantTarget {
						found = true
					}
				}
				if !found {
					t.Errorf("target line %q not found", tt.wantTarget)
					for i, l := range lines {
						t.Logf("  [%d] old=%d new=%d prefix=%q text=%q target=%v",
							i, l.OldNum, l.NewNum, l.Prefix, l.Text, l.IsTarget)
					}
				}
			}
		})
	}
}

func TestExtractDiffContext_WrongPath(t *testing.T) {
	hunks := []platform.DiffHunk{
		{Path: "foo.go", NewStart: 1, NewLines: 3, Body: " a\n+b\n c\n"},
	}
	lines := extractDiffContext(2, "new", "bar.go", hunks, 3)
	if len(lines) != 0 {
		t.Errorf("expected empty for wrong path, got %d lines", len(lines))
	}
}

func TestExtractDiffContext_LineNumbers(t *testing.T) {
	hunks := []platform.DiffHunk{
		{
			Path:     "test.go",
			OldStart: 5,
			OldLines: 3,
			NewStart: 5,
			NewLines: 4,
			Body:     " line5\n-old6\n+new6\n+new7\n line8\n",
		},
	}

	lines := extractDiffContext(6, "new", "test.go", hunks, 1)

	// Should have: line5, old6 (removed), new6 (target), +/- 1 context
	for _, l := range lines {
		if l.IsTarget {
			if l.NewNum != 6 {
				t.Errorf("target NewNum = %d, want 6", l.NewNum)
			}
			if l.Prefix != "+" {
				t.Errorf("target Prefix = %q, want %q", l.Prefix, "+")
			}
		}
	}
}

func TestRenderDiffSnippet(t *testing.T) {
	lines := []contextLine{
		{OldNum: 10, NewNum: 10, Prefix: " ", Text: "context"},
		{OldNum: 11, Prefix: "-", Text: "removed"},
		{NewNum: 11, Prefix: "+", Text: "added", IsTarget: true},
		{OldNum: 12, NewNum: 12, Prefix: " ", Text: "more context"},
	}

	var buf bytes.Buffer
	renderDiffSnippet(&buf, lines)
	out := buf.String()

	// Should contain line numbers and prefixes.
	if !strings.Contains(out, "10") {
		t.Error("output should contain line number 10")
	}
	if !strings.Contains(out, "context") {
		t.Error("output should contain 'context'")
	}
	if !strings.Contains(out, "removed") {
		t.Error("output should contain 'removed'")
	}
	if !strings.Contains(out, "added") {
		t.Error("output should contain 'added'")
	}
}
