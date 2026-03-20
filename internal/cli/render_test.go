package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/review"
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

func TestExtractDiffContext_TargetNotInHunk(t *testing.T) {
	// Hunk covers new lines 10-17, but we ask for line 15 which is within
	// the hunk range yet might not match any parsed line (e.g., the hunk body
	// doesn't have enough lines). After the #7 fix, this should return nil
	// instead of all parsed lines.
	hunks := []platform.DiffHunk{
		{
			Path:     "main.go",
			OldStart: 10,
			OldLines: 3,
			NewStart: 10,
			NewLines: 5,
			Body:     " line10\n+line11\n+line12\n",
		},
	}

	// Line 15 is within NewStart+NewLines range (10+5=15) but there's no
	// line 15 in the hunk body. The function should find the hunk but not
	// the target line, and return nil.
	lines := extractDiffContext(14, "new", "main.go", hunks, 3)
	if lines != nil {
		t.Errorf("expected nil when target line not found in hunk body, got %d lines", len(lines))
	}
}

func TestRenderFindings(t *testing.T) {
	posted := []review.PostedComment{
		{
			Finding: platform.ReviewFinding{
				Path:     "main.go",
				Line:     13,
				Side:     "new",
				Severity: "warning",
				Category: "bug",
				Message:  "potential nil pointer",
			},
			CommentID:    "dry-run-1",
			RenderedBody: "**warning** | bug\n\npotential nil pointer",
		},
		{
			Finding: platform.ReviewFinding{
				Path:     "util.go",
				Line:     5,
				Side:     "new",
				Severity: "info",
				Category: "style",
				Message:  "consider renaming",
			},
			CommentID:    "dry-run-2",
			RenderedBody: "**info** | style\n\nconsider renaming",
		},
	}

	hunks := []platform.DiffHunk{
		{
			Path:     "main.go",
			OldStart: 10,
			OldLines: 7,
			NewStart: 10,
			NewLines: 8,
			Body:     " line10\n line11\n line12\n-old13\n+new13\n+new14\n line15\n line16\n",
		},
		{
			Path:     "util.go",
			OldStart: 1,
			OldLines: 5,
			NewStart: 1,
			NewLines: 6,
			Body:     " line1\n line2\n line3\n line4\n+line5\n line6\n",
		},
	}

	var buf bytes.Buffer
	RenderFindings(posted, hunks, &buf, true) // raw=true to skip mdterm
	out := buf.String()

	// Should contain the header with count.
	if !strings.Contains(out, "REVIEW COMMENTS (2)") {
		t.Errorf("expected header with count 2, got:\n%s", out)
	}

	// Should contain both file paths.
	if !strings.Contains(out, "main.go") {
		t.Error("output should contain main.go")
	}
	if !strings.Contains(out, "util.go") {
		t.Error("output should contain util.go")
	}

	// Should contain the rendered bodies.
	if !strings.Contains(out, "potential nil pointer") {
		t.Error("output should contain first finding's body")
	}
	if !strings.Contains(out, "consider renaming") {
		t.Error("output should contain second finding's body")
	}

	// Should contain separator between findings.
	if !strings.Contains(out, "───") {
		t.Error("output should contain separator between findings")
	}

	// Should contain footer.
	if !strings.Contains(out, "END REVIEW COMMENTS") {
		t.Error("output should contain footer")
	}
}

func TestRenderFindings_Empty(t *testing.T) {
	var buf bytes.Buffer
	RenderFindings(nil, nil, &buf, true)
	out := buf.String()

	if !strings.Contains(out, "REVIEW COMMENTS (0)") {
		t.Errorf("expected header with count 0, got:\n%s", out)
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
