package platform_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
)

// ---------------------------------------------------------------------------
// JSON round-trip tests
// ---------------------------------------------------------------------------

func TestPRContextJSON(t *testing.T) {
	t.Parallel()

	original := platform.PRContext{
		ID:           42,
		Title:        "Add feature X",
		Description:  "Implements feature X as described in PROJ-123.",
		Author:       "alice",
		SourceBranch: "feature/x",
		TargetBranch: "main",
		State:        "OPEN",
		HeadCommit:   "abc123",
		BaseCommit:   "def456",
		Files: []platform.ChangedFile{
			{Path: "foo.go", Status: "modified"},
			{Path: "bar.go", OldPath: "baz.go", Status: "renamed"},
		},
		DiffHunks: []platform.DiffHunk{
			{
				Path:     "foo.go",
				OldStart: 10,
				OldLines: 5,
				NewStart: 10,
				NewLines: 7,
				Body:     "@@ -10,5 +10,7 @@\n+added line",
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded platform.PRContext
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %d, want %d", decoded.ID, original.ID)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title: got %q, want %q", decoded.Title, original.Title)
	}
	if decoded.SourceBranch != original.SourceBranch {
		t.Errorf("SourceBranch: got %q, want %q", decoded.SourceBranch, original.SourceBranch)
	}
	if len(decoded.Files) != len(original.Files) {
		t.Fatalf("Files length: got %d, want %d", len(decoded.Files), len(original.Files))
	}
	if decoded.Files[1].OldPath != "baz.go" {
		t.Errorf("Files[1].OldPath: got %q, want %q", decoded.Files[1].OldPath, "baz.go")
	}
	if len(decoded.DiffHunks) != len(original.DiffHunks) {
		t.Fatalf("DiffHunks length: got %d, want %d", len(decoded.DiffHunks), len(original.DiffHunks))
	}
	if decoded.DiffHunks[0].Body != original.DiffHunks[0].Body {
		t.Errorf("DiffHunks[0].Body: got %q, want %q", decoded.DiffHunks[0].Body, original.DiffHunks[0].Body)
	}
}

func TestChangedFileJSON_OmitEmpty(t *testing.T) {
	t.Parallel()

	cf := platform.ChangedFile{Path: "main.go", Status: "added"}
	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// OldPath should be omitted when empty.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["old_path"]; ok {
		t.Error("old_path should be omitted when empty")
	}
}

func TestReviewFindingJSON(t *testing.T) {
	t.Parallel()

	original := platform.ReviewFinding{
		Path:        "pkg/handler.go",
		Line:        15,
		Side:        "new",
		Severity:    "warning",
		Category:    "error-handling",
		Message:     "Error is not checked.",
		Suggestion:  "if err != nil { return err }",
		Fingerprint: "abc123",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded platform.ReviewFinding
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("round-trip mismatch:\n  got  %+v\n  want %+v", decoded, original)
	}
}

func TestReviewFindingJSON_SuggestionOmitted(t *testing.T) {
	t.Parallel()

	f := platform.ReviewFinding{
		Path:     "a.go",
		Line:     1,
		Side:     "new",
		Severity: "info",
		Message:  "note",
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["suggestion"]; ok {
		t.Error("suggestion should be omitted when empty")
	}
}

func TestInlineCommentJSON(t *testing.T) {
	t.Parallel()

	original := platform.InlineComment{
		Path:        "main.go",
		Line:        42,
		Side:        "new",
		Body:        "Consider using a constant here.",
		Fingerprint: "fp1",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded platform.InlineComment
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n  got  %+v\n  want %+v", decoded, original)
	}
}

func TestCommentJSON(t *testing.T) {
	t.Parallel()

	original := platform.Comment{
		ID:          "12345",
		Path:        "handler.go",
		Line:        7,
		Body:        "LGTM",
		Author:      "bot",
		CreatedAt:   "2025-01-15T10:30:00Z",
		IsBot:       true,
		Fingerprint: "fp-xyz",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded platform.Comment
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n  got  %+v\n  want %+v", decoded, original)
	}
}

func TestCommentJSON_FingerprintOmitted(t *testing.T) {
	t.Parallel()

	c := platform.Comment{ID: "1", Body: "hi", Author: "bot"}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["fingerprint"]; ok {
		t.Error("fingerprint should be omitted when empty")
	}
}

// ---------------------------------------------------------------------------
// ReviewFinding.Validate tests
// ---------------------------------------------------------------------------

func TestReviewFinding_Validate(t *testing.T) {
	t.Parallel()

	validFinding := func() platform.ReviewFinding {
		return platform.ReviewFinding{
			Path:        "file.go",
			Line:        10,
			Side:        "new",
			Severity:    "warning",
			Category:    "style",
			Message:     "Use consistent casing.",
			Fingerprint: "abc",
		}
	}

	tests := []struct {
		name    string
		modify  func(*platform.ReviewFinding)
		wantErr error
	}{
		{
			name:    "valid finding",
			modify:  func(_ *platform.ReviewFinding) {},
			wantErr: nil,
		},
		{
			name:    "empty path",
			modify:  func(f *platform.ReviewFinding) { f.Path = "" },
			wantErr: platform.ErrEmptyPath,
		},
		{
			name:    "zero line",
			modify:  func(f *platform.ReviewFinding) { f.Line = 0 },
			wantErr: platform.ErrInvalidLine,
		},
		{
			name:    "negative line",
			modify:  func(f *platform.ReviewFinding) { f.Line = -5 },
			wantErr: platform.ErrInvalidLine,
		},
		{
			name:    "invalid side",
			modify:  func(f *platform.ReviewFinding) { f.Side = "left" },
			wantErr: platform.ErrInvalidSide,
		},
		{
			name:    "empty side",
			modify:  func(f *platform.ReviewFinding) { f.Side = "" },
			wantErr: platform.ErrInvalidSide,
		},
		{
			name:    "invalid severity",
			modify:  func(f *platform.ReviewFinding) { f.Severity = "critical" },
			wantErr: platform.ErrInvalidSeverity,
		},
		{
			name:    "empty severity",
			modify:  func(f *platform.ReviewFinding) { f.Severity = "" },
			wantErr: platform.ErrInvalidSeverity,
		},
		{
			name:    "empty message",
			modify:  func(f *platform.ReviewFinding) { f.Message = "" },
			wantErr: platform.ErrEmptyMessage,
		},
		{
			name:    "side old is valid",
			modify:  func(f *platform.ReviewFinding) { f.Side = "old" },
			wantErr: nil,
		},
		{
			name:    "severity info is valid",
			modify:  func(f *platform.ReviewFinding) { f.Severity = "info" },
			wantErr: nil,
		},
		{
			name:    "severity error is valid",
			modify:  func(f *platform.ReviewFinding) { f.Severity = "error" },
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := validFinding()
			tt.modify(&f)
			err := f.Validate()

			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error wrapping %v, got nil", tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error wrapping %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ParseFindings tests
// ---------------------------------------------------------------------------

func TestParseFindings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantCount int
		wantErr   bool
	}{
		{
			name: "valid array with one finding",
			input: `[{
				"path": "main.go",
				"line": 5,
				"side": "new",
				"severity": "error",
				"category": "bug",
				"message": "nil pointer dereference",
				"fingerprint": "fp1"
			}]`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "empty array",
			input:     `[]`,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:    "invalid JSON",
			input:   `{not valid json`,
			wantErr: true,
		},
		{
			name: "single object instead of array",
			input: `{
				"path": "main.go",
				"line": 5,
				"side": "new",
				"severity": "error",
				"category": "bug",
				"message": "oops",
				"fingerprint": "fp1"
			}`,
			wantErr: true,
		},
		{
			name: "valid array with multiple findings",
			input: `[
				{"path":"a.go","line":1,"side":"new","severity":"info","category":"style","message":"m1","fingerprint":"f1"},
				{"path":"b.go","line":2,"side":"old","severity":"warning","category":"perf","message":"m2","fingerprint":"f2"}
			]`,
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			findings, err := platform.ParseFindings([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(findings) != tt.wantCount {
				t.Errorf("findings count: got %d, want %d", len(findings), tt.wantCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Factory tests
// ---------------------------------------------------------------------------

func TestNewPlatform_UnknownPlatform(t *testing.T) {
	t.Parallel()

	_, err := platform.NewPlatform("nosuchplatform", config.Config{})
	if err == nil {
		t.Fatal("expected error for unknown platform, got nil")
	}
	if !errors.Is(err, platform.ErrUnknownPlatform) {
		t.Errorf("expected error wrapping ErrUnknownPlatform, got: %v", err)
	}
}

func TestNewPlatform_RegisteredPlatform(t *testing.T) {
	t.Parallel()

	// Register a stub platform for the test.
	platform.Register("stub", func(cfg config.Config) (platform.Platform, error) {
		return stubPlatform{}, nil
	})

	p, err := platform.NewPlatform("stub", config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil platform")
	}
}

// stubPlatform is a minimal Platform implementation for factory tests.
type stubPlatform struct{}

func (stubPlatform) GetPRContext(_ context.Context, _ platform.PRRequest) (*platform.PRContext, error) {
	return nil, nil
}
func (stubPlatform) GetFileContent(_ context.Context, _ platform.FileRequest) ([]byte, error) {
	return nil, nil
}
func (stubPlatform) ListBotComments(_ context.Context, _ platform.PRRequest) ([]platform.Comment, error) {
	return nil, nil
}
func (stubPlatform) CreateInlineComment(_ context.Context, _ platform.PRRequest, _ platform.InlineComment) (*platform.Comment, error) {
	return nil, nil
}
func (stubPlatform) DeleteComment(_ context.Context, _ platform.PRRequest, _ string) error {
	return nil
}
