package review_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/review"
)

func TestRenderComment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		finding    platform.ReviewFinding
		botLabel   string
		goldenFile string
	}{
		{
			name: "warning with suggestion",
			finding: platform.ReviewFinding{
				Path:          "src/auth.ts",
				Line:          42,
				Side:          "new",
				Severity:      "warning",
				SeverityScore: 7,
				Category:      "security",
				Criteria:      []string{"Security", "Maintainability"},
				Message:       "Logging the raw token can leak credentials.",
				Suggestion:    "log.Info(\"auth completed\")",
				Fingerprint:   "src/auth.ts:new:42:token-log",
			},
			botLabel:   "crobot",
			goldenFile: "render_warning_with_suggestion.golden",
		},
		{
			name: "error without suggestion",
			finding: platform.ReviewFinding{
				Path:          "main.go",
				Line:          15,
				Side:          "new",
				Severity:      "error",
				SeverityScore: 9,
				Category:      "bug",
				Criteria:      []string{"Reliability"},
				Message:       "Nil pointer dereference on line 15.",
				Fingerprint:   "main.go:new:15:nil-deref",
			},
			botLabel:   "crobot",
			goldenFile: "render_error_no_suggestion.golden",
		},
		{
			name: "info without suggestion",
			finding: platform.ReviewFinding{
				Path:          "util.go",
				Line:          8,
				Side:          "new",
				Severity:      "info",
				SeverityScore: 3,
				Category:      "style",
				Criteria:      []string{"Readability"},
				Message:       "Consider using a constant here.",
				Fingerprint:   "util.go:new:8:use-const",
			},
			botLabel:   "crobot",
			goldenFile: "render_info_no_suggestion.golden",
		},
		{
			name: "special characters in message",
			finding: platform.ReviewFinding{
				Path:          "special.go",
				Line:          10,
				Side:          "new",
				Severity:      "warning",
				SeverityScore: 5,
				Category:      "performance",
				Criteria:      []string{"Performance", "Maintainability"},
				Message:       "String contains \"special\" <chars> & entities.",
				Suggestion:    "use &amp; properly",
				Fingerprint:   "special.go:new:10:special-fp",
			},
			botLabel:   "crobot",
			goldenFile: "render_special_chars.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := review.RenderComment(tt.finding, tt.botLabel)

			goldenPath := filepath.Join("testdata", tt.goldenFile)
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("reading golden file %s: %v", goldenPath, err)
			}

			if got != string(want) {
				t.Errorf("output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", tt.goldenFile, got, string(want))
			}
		})
	}
}

func TestRenderComment_NoFingerprint(t *testing.T) {
	t.Parallel()

	f := platform.ReviewFinding{
		Path:     "a.go",
		Line:     1,
		Side:     "new",
		Severity: "info",
		Category: "style",
		Message:  "test msg",
	}

	got := review.RenderComment(f, "crobot")

	// Should contain an auto-generated fingerprint.
	if !containsSubstring(got, "[//]: # \"crobot:fp=") {
		t.Error("rendered comment should contain a fingerprint tag")
	}
}

func TestRenderComment_NoCategory(t *testing.T) {
	t.Parallel()

	f := platform.ReviewFinding{
		Path:        "a.go",
		Line:        1,
		Side:        "new",
		Severity:    "warning",
		Category:    "",
		Message:     "no category here",
		Fingerprint: "fp-1",
	}

	got := review.RenderComment(f, "crobot")

	// Should NOT contain a pipe separator when category is empty.
	if containsSubstring(got, " | ") {
		t.Error("rendered comment should not contain ' | ' when category is empty")
	}

	// Should still contain severity with icon.
	if !containsSubstring(got, "**warning**") {
		t.Error("rendered comment should contain severity badge")
	}
}
