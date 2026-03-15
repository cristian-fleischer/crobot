package review_test

import (
	"testing"

	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/review"
)

// testPRContext returns a PRContext with two changed files and multiple hunks
// for use across validation tests.
func testPRContext() *platform.PRContext {
	return &platform.PRContext{
		ID:           1,
		Title:        "Test PR",
		SourceBranch: "feature",
		TargetBranch: "main",
		HeadCommit:   "abc123",
		BaseCommit:   "def456",
		Files: []platform.ChangedFile{
			{Path: "src/main.go", Status: "modified"},
			{Path: "src/util.go", Status: "added"},
			{Path: "src/old.go", Status: "deleted"},
		},
		DiffHunks: []platform.DiffHunk{
			{
				Path:     "src/main.go",
				OldStart: 10,
				OldLines: 5,
				NewStart: 10,
				NewLines: 7,
			},
			{
				Path:     "src/main.go",
				OldStart: 50,
				OldLines: 3,
				NewStart: 52,
				NewLines: 4,
			},
			{
				Path:     "src/util.go",
				OldStart: 0,
				OldLines: 0,
				NewStart: 1,
				NewLines: 20,
			},
			{
				Path:     "src/old.go",
				OldStart: 1,
				OldLines: 10,
				NewStart: 0,
				NewLines: 0,
			},
		},
	}
}

func validFinding() platform.ReviewFinding {
	return platform.ReviewFinding{
		Path:        "src/main.go",
		Line:        12,
		Side:        "new",
		Severity:    "warning",
		Category:    "style",
		Message:     "Consider renaming this variable.",
		Fingerprint: "fp-1",
	}
}

func TestValidateFindings(t *testing.T) {
	t.Parallel()

	ctx := testPRContext()

	tests := []struct {
		name              string
		findings          []platform.ReviewFinding
		threshold         string
		wantValidCount    int
		wantRejectedCount int
		wantReasons       []string // substring match on rejection reasons
	}{
		{
			name:              "valid finding in hunk range",
			findings:          []platform.ReviewFinding{validFinding()},
			threshold:         "info",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name: "path not in changed files",
			findings: []platform.ReviewFinding{
				{
					Path: "src/missing.go", Line: 5, Side: "new",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    0,
			wantRejectedCount: 1,
			wantReasons:       []string{"not in changed files"},
		},
		{
			name: "line outside hunk range - new side",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 5, Side: "new",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    0,
			wantRejectedCount: 1,
			wantReasons:       []string{"not within any diff hunk"},
		},
		{
			name: "line at hunk boundary start - new side (inclusive)",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 10, Side: "new",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name: "line at hunk boundary end - new side (exclusive, last valid)",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 16, Side: "new",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name: "line just past hunk boundary - new side (exclusive)",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 17, Side: "new",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    0,
			wantRejectedCount: 1,
			wantReasons:       []string{"not within any diff hunk"},
		},
		{
			name: "line in second hunk - new side",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 53, Side: "new",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name: "old side - line in old hunk range",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 12, Side: "old",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name: "old side - line outside old hunk range",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 15, Side: "old",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    0,
			wantRejectedCount: 1,
			wantReasons:       []string{"not within any diff hunk"},
		},
		{
			name: "old side - boundary start (inclusive)",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 10, Side: "old",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name: "old side - boundary end exclusive",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 14, Side: "old",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name: "old side - one past end",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 15, Side: "old",
					Severity: "warning", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    0,
			wantRejectedCount: 1,
		},
		{
			name: "severity threshold - info finding with warning threshold",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 12, Side: "new",
					Severity: "info", Category: "style", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "warning",
			wantValidCount:    0,
			wantRejectedCount: 1,
			wantReasons:       []string{"below threshold"},
		},
		{
			name: "severity threshold - warning finding with warning threshold",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 12, Side: "new",
					Severity: "warning", Category: "style", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "warning",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name: "severity threshold - error finding with warning threshold",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 12, Side: "new",
					Severity: "error", Category: "bug", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "warning",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name: "severity threshold - info finding with error threshold",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 12, Side: "new",
					Severity: "info", Category: "style", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "error",
			wantValidCount:    0,
			wantRejectedCount: 1,
			wantReasons:       []string{"below threshold"},
		},
		{
			name: "severity threshold - warning finding with error threshold",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 12, Side: "new",
					Severity: "warning", Category: "style", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "error",
			wantValidCount:    0,
			wantRejectedCount: 1,
			wantReasons:       []string{"below threshold"},
		},
		{
			name: "invalid finding - empty path",
			findings: []platform.ReviewFinding{
				{
					Path: "", Line: 12, Side: "new",
					Severity: "warning", Category: "style", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    0,
			wantRejectedCount: 1,
			wantReasons:       []string{"validation error"},
		},
		{
			name: "invalid finding - bad side",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 12, Side: "left",
					Severity: "warning", Category: "style", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    0,
			wantRejectedCount: 1,
			wantReasons:       []string{"validation error"},
		},
		{
			name: "multiple findings mixed valid and invalid",
			findings: []platform.ReviewFinding{
				{
					Path: "src/main.go", Line: 12, Side: "new",
					Severity: "warning", Category: "style", Message: "good", Fingerprint: "fp-1",
				},
				{
					Path: "src/missing.go", Line: 1, Side: "new",
					Severity: "warning", Category: "bug", Message: "bad path", Fingerprint: "fp-2",
				},
				{
					Path: "src/main.go", Line: 53, Side: "new",
					Severity: "error", Category: "bug", Message: "also good", Fingerprint: "fp-3",
				},
			},
			threshold:         "info",
			wantValidCount:    2,
			wantRejectedCount: 1,
		},
		{
			name: "new file - line in new hunk range",
			findings: []platform.ReviewFinding{
				{
					Path: "src/util.go", Line: 10, Side: "new",
					Severity: "warning", Category: "style", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name: "deleted file - line in old hunk range",
			findings: []platform.ReviewFinding{
				{
					Path: "src/old.go", Line: 5, Side: "old",
					Severity: "warning", Category: "style", Message: "msg", Fingerprint: "fp",
				},
			},
			threshold:         "info",
			wantValidCount:    1,
			wantRejectedCount: 0,
		},
		{
			name:              "empty findings",
			findings:          []platform.ReviewFinding{},
			threshold:         "info",
			wantValidCount:    0,
			wantRejectedCount: 0,
		},
		{
			name:              "nil findings",
			findings:          nil,
			threshold:         "info",
			wantValidCount:    0,
			wantRejectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			valid, rejected := review.ValidateFindings(tt.findings, ctx, tt.threshold)

			if len(valid) != tt.wantValidCount {
				t.Errorf("valid count: got %d, want %d", len(valid), tt.wantValidCount)
			}
			if len(rejected) != tt.wantRejectedCount {
				t.Errorf("rejected count: got %d, want %d", len(rejected), tt.wantRejectedCount)
			}

			// Check rejection reasons if specified.
			for i, reason := range tt.wantReasons {
				if i >= len(rejected) {
					t.Errorf("expected reason %d (%q) but only got %d rejections", i, reason, len(rejected))
					continue
				}
				if !containsSubstring(rejected[i].Reason, reason) {
					t.Errorf("rejected[%d].Reason = %q, want substring %q", i, rejected[i].Reason, reason)
				}
			}
		})
	}
}

func TestValidateFindings_MultipleHunksPerFile(t *testing.T) {
	t.Parallel()

	ctx := testPRContext()

	// Line 12 is in first hunk [10, 17), line 53 is in second hunk [52, 56).
	// Line 20 is between hunks (gap).
	findings := []platform.ReviewFinding{
		{Path: "src/main.go", Line: 12, Side: "new", Severity: "warning", Category: "a", Message: "in hunk 1", Fingerprint: "fp1"},
		{Path: "src/main.go", Line: 20, Side: "new", Severity: "warning", Category: "a", Message: "between hunks", Fingerprint: "fp2"},
		{Path: "src/main.go", Line: 53, Side: "new", Severity: "warning", Category: "a", Message: "in hunk 2", Fingerprint: "fp3"},
	}

	valid, rejected := review.ValidateFindings(findings, ctx, "info")
	if len(valid) != 2 {
		t.Errorf("valid count: got %d, want 2", len(valid))
	}
	if len(rejected) != 1 {
		t.Errorf("rejected count: got %d, want 1", len(rejected))
	}
	if len(rejected) > 0 && rejected[0].Finding.Line != 20 {
		t.Errorf("expected rejected finding at line 20, got line %d", rejected[0].Finding.Line)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
