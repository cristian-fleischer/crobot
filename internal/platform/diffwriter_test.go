package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDiffDir(t *testing.T) {
	t.Parallel()

	dir := NewDiffDir(".crobot")
	if !strings.HasPrefix(dir, ".crobot/diffs-") {
		t.Errorf("NewDiffDir() = %q, want prefix .crobot/diffs-", dir)
	}

	// Two calls should produce different directories.
	dir2 := NewDiffDir(".crobot")
	if dir == dir2 {
		t.Errorf("NewDiffDir() returned same dir twice: %q", dir)
	}
}

func TestWriteDiffFiles(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()

	hunks := []DiffHunk{
		{
			Path:     "src/auth.go",
			OldStart: 0, OldLines: 0,
			NewStart: 1, NewLines: 3,
			Body: "+package auth\n+\n+func Verify() {}\n",
		},
		{
			Path:     "src/auth.go",
			OldStart: 10, OldLines: 2,
			NewStart: 10, NewLines: 3,
			Body: " existing\n-old\n+new\n+added\n",
		},
		{
			Path:     "pkg/db/conn.go",
			OldStart: 5, OldLines: 3,
			NewStart: 5, NewLines: 4,
			Body: " func Open() {\n-\treturn nil\n+\tdb, err := sql.Open()\n+\treturn db\n",
		},
	}

	stats := ComputeDiffStats(hunks)
	err := WriteDiffFiles(hunks, stats, outputDir)
	if err != nil {
		t.Fatalf("WriteDiffFiles: %v", err)
	}

	// Check auth.go diff file exists and contains both hunks.
	authContent, err := os.ReadFile(filepath.Join(outputDir, "src", "auth.go"))
	if err != nil {
		t.Fatalf("reading auth.go diff: %v", err)
	}
	authStr := string(authContent)
	if !strings.Contains(authStr, "@@ -0,0 +1,3 @@") {
		t.Error("auth.go diff missing first hunk header")
	}
	if !strings.Contains(authStr, "@@ -10,2 +10,3 @@") {
		t.Error("auth.go diff missing second hunk header")
	}
	if !strings.Contains(authStr, "+func Verify()") {
		t.Error("auth.go diff missing body content")
	}

	// Check conn.go diff file exists.
	connContent, err := os.ReadFile(filepath.Join(outputDir, "pkg", "db", "conn.go"))
	if err != nil {
		t.Fatalf("reading conn.go diff: %v", err)
	}
	if !strings.Contains(string(connContent), "sql.Open") {
		t.Error("conn.go diff missing body content")
	}

	// Check index file.
	indexContent, err := os.ReadFile(filepath.Join(outputDir, ".crobot-index.md"))
	if err != nil {
		t.Fatalf("reading .crobot-index.md: %v", err)
	}
	indexStr := string(indexContent)
	if !strings.Contains(indexStr, "2 files changed") {
		t.Error("index missing file count")
	}
	if !strings.Contains(indexStr, "3 hunks total") {
		t.Error("index missing hunk count")
	}
	if !strings.Contains(indexStr, "src/auth.go") {
		t.Error("index missing auth.go entry")
	}
	if !strings.Contains(indexStr, "pkg/db/conn.go") {
		t.Error("index missing conn.go entry")
	}
}

func TestWriteDiffFiles_Empty(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	stats := ComputeDiffStats(nil)
	err := WriteDiffFiles(nil, stats, outputDir)
	if err != nil {
		t.Fatalf("WriteDiffFiles: %v", err)
	}

	// Index should still exist.
	indexContent, err := os.ReadFile(filepath.Join(outputDir, ".crobot-index.md"))
	if err != nil {
		t.Fatalf("reading .crobot-index.md: %v", err)
	}
	if !strings.Contains(string(indexContent), "0 files changed") {
		t.Error("index should report 0 files changed")
	}
}

func TestWriteDiffFiles_LowValueNotes(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()

	hunks := []DiffHunk{
		{Path: "go.sum", Body: "hash data\n"},
		{Path: "vendor/lib/x.go", Body: "+package x\n"},
		{Path: "src/real.go", Body: "+real code\n"},
	}

	stats := ComputeDiffStats(hunks)
	err := WriteDiffFiles(hunks, stats, outputDir)
	if err != nil {
		t.Fatalf("WriteDiffFiles: %v", err)
	}

	indexContent, err := os.ReadFile(filepath.Join(outputDir, ".crobot-index.md"))
	if err != nil {
		t.Fatalf("reading .crobot-index.md: %v", err)
	}
	indexStr := string(indexContent)

	if !strings.Contains(indexStr, "lock file") {
		t.Error("index should note go.sum as lock file")
	}
	if !strings.Contains(indexStr, "vendor") {
		t.Error("index should note vendor file")
	}
}

func TestCleanupStaleDiffDirs(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()

	// Old diff dir: mtime backdated well before the cutoff.
	oldDir := filepath.Join(baseDir, "diffs-111")
	// Fresh diff dir: represents a concurrent review that should be skipped.
	freshDir := filepath.Join(baseDir, "diffs-222")
	other := filepath.Join(baseDir, "config.yaml")

	os.MkdirAll(oldDir, 0o755)
	os.MkdirAll(freshDir, 0o755)
	os.WriteFile(filepath.Join(oldDir, "test.go"), []byte("test"), 0o644)
	os.WriteFile(filepath.Join(freshDir, "test.go"), []byte("test"), 0o644)
	os.WriteFile(other, []byte("keep"), 0o644)

	// Backdate the old dir so it falls outside the 1h window.
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := CleanupStaleDiffDirs(baseDir, time.Hour); err != nil {
		t.Fatalf("CleanupStaleDiffDirs: %v", err)
	}

	// Old dir should be gone.
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("oldDir should have been removed")
	}
	// Fresh dir should be preserved (concurrent-run protection).
	if _, err := os.Stat(freshDir); err != nil {
		t.Errorf("freshDir should have been preserved: %v", err)
	}
	// Other file should remain.
	if _, err := os.Stat(other); err != nil {
		t.Error("config.yaml should not have been removed")
	}
}

func TestCleanupDiffDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	subDir := filepath.Join(dir, "diffs-123")
	os.MkdirAll(filepath.Join(subDir, "src"), 0o755)
	os.WriteFile(filepath.Join(subDir, "src", "test.go"), []byte("test"), 0o644)

	err := CleanupDiffDir(subDir)
	if err != nil {
		t.Fatalf("CleanupDiffDir: %v", err)
	}

	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Error("diff dir should have been removed")
	}
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input int
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{2560, "2.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := formatBytes(tt.input); got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
