package platform

import (
	"testing"
)

func TestComputeDiffStats_Basic(t *testing.T) {
	t.Parallel()

	hunks := []DiffHunk{
		{Path: "src/auth.go", Body: "+package auth\n+func Verify() {}\n"},
		{Path: "src/auth.go", Body: "+func Refresh() {}\n"},
		{Path: "src/main.go", Body: " func main() {\n-old()\n+new()\n"},
	}

	stats := ComputeDiffStats(hunks)

	if stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", stats.TotalFiles)
	}
	if stats.TotalHunks != 3 {
		t.Errorf("TotalHunks = %d, want 3", stats.TotalHunks)
	}
	if len(stats.FileStats) != 2 {
		t.Fatalf("FileStats length = %d, want 2", len(stats.FileStats))
	}

	// First file: auth.go with 2 hunks.
	auth := stats.FileStats[0]
	if auth.Path != "src/auth.go" {
		t.Errorf("FileStats[0].Path = %q, want %q", auth.Path, "src/auth.go")
	}
	if auth.HunkCount != 2 {
		t.Errorf("FileStats[0].HunkCount = %d, want 2", auth.HunkCount)
	}
	if auth.IsLowValue {
		t.Error("FileStats[0].IsLowValue = true, want false")
	}

	// Second file: main.go with 1 hunk.
	main := stats.FileStats[1]
	if main.Path != "src/main.go" {
		t.Errorf("FileStats[1].Path = %q, want %q", main.Path, "src/main.go")
	}
	if main.HunkCount != 1 {
		t.Errorf("FileStats[1].HunkCount = %d, want 1", main.HunkCount)
	}

	// TotalBodyBytes should be sum of all body bytes.
	expectedBytes := len(hunks[0].Body) + len(hunks[1].Body) + len(hunks[2].Body)
	if stats.TotalBodyBytes != expectedBytes {
		t.Errorf("TotalBodyBytes = %d, want %d", stats.TotalBodyBytes, expectedBytes)
	}
}

func TestComputeDiffStats_Empty(t *testing.T) {
	t.Parallel()

	stats := ComputeDiffStats(nil)

	if stats.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0", stats.TotalFiles)
	}
	if stats.TotalHunks != 0 {
		t.Errorf("TotalHunks = %d, want 0", stats.TotalHunks)
	}
	if len(stats.FileStats) != 0 {
		t.Errorf("FileStats length = %d, want 0", len(stats.FileStats))
	}
}

func TestComputeDiffStats_LowValueFiles(t *testing.T) {
	t.Parallel()

	hunks := []DiffHunk{
		{Path: "go.sum", Body: "hash\n"},
		{Path: "package-lock.json", Body: "lock data\n"},
		{Path: "vendor/lib/x.go", Body: "+package x\n"},
		{Path: "src/api.pb.go", Body: "+generated\n"},
		{Path: "ui/bundle.min.js", Body: "+minified\n"},
		{Path: "ui/styles.min.css", Body: "+minified\n"},
		{Path: "src/schema_generated.go", Body: "+gen\n"},
		{Path: "src/types.gen.ts", Body: "+gen\n"},
		{Path: "node_modules/foo/index.js", Body: "+module\n"},
		{Path: "src/real.go", Body: "+real code\n"},
	}

	stats := ComputeDiffStats(hunks)

	for _, fs := range stats.FileStats {
		switch fs.Path {
		case "src/real.go":
			if fs.IsLowValue {
				t.Errorf("%s: IsLowValue = true, want false", fs.Path)
			}
		default:
			if !fs.IsLowValue {
				t.Errorf("%s: IsLowValue = false, want true", fs.Path)
			}
		}
	}
}

func TestComputeDiffStats_PreservesOrder(t *testing.T) {
	t.Parallel()

	hunks := []DiffHunk{
		{Path: "c.go", Body: "c"},
		{Path: "a.go", Body: "a"},
		{Path: "b.go", Body: "b"},
		{Path: "a.go", Body: "a2"},
	}

	stats := ComputeDiffStats(hunks)

	if len(stats.FileStats) != 3 {
		t.Fatalf("FileStats length = %d, want 3", len(stats.FileStats))
	}
	want := []string{"c.go", "a.go", "b.go"}
	for i, w := range want {
		if stats.FileStats[i].Path != w {
			t.Errorf("FileStats[%d].Path = %q, want %q", i, stats.FileStats[i].Path, w)
		}
	}
	// a.go should have 2 hunks.
	if stats.FileStats[1].HunkCount != 2 {
		t.Errorf("a.go HunkCount = %d, want 2", stats.FileStats[1].HunkCount)
	}
}

func TestIsLowValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{"go.sum", true},
		{"package-lock.json", true},
		{"yarn.lock", true},
		{"Cargo.lock", true},
		{"vendor/pkg/foo.go", true},
		{"node_modules/bar/index.js", true},
		{"api.pb.go", true},
		{"schema_generated.go", true},
		{"types.gen.ts", true},
		{"app.min.js", true},
		{"style.min.css", true},
		{"src/auth.go", false},
		{"README.md", false},
		{"cmd/main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			if got := isLowValue(tt.path); got != tt.want {
				t.Errorf("isLowValue(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
