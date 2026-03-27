package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NewDiffDir creates a unique directory under baseDir for this run. The name
// includes a unix-nanosecond timestamp to avoid collisions between concurrent
// runs. Returns the path, e.g. ".crobot/diffs-1710936000000000000".
func NewDiffDir(baseDir string) string {
	return filepath.Join(baseDir, fmt.Sprintf("diffs-%d", time.Now().UnixNano()))
}

// CleanupStaleDiffDirs removes all .crobot/diffs-* directories under baseDir.
// Safe to call at startup to clean up after killed processes.
func CleanupStaleDiffDirs(baseDir string) error {
	pattern := filepath.Join(baseDir, "diffs-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("globbing stale diff dirs: %w", err)
	}
	for _, m := range matches {
		if err := os.RemoveAll(m); err != nil {
			return fmt.Errorf("removing stale diff dir %s: %w", m, err)
		}
	}
	return nil
}

// WriteDiffFiles writes per-file diff hunks and an index to outputDir. Each
// file at outputDir/<path> contains the formatted unified diff hunks. The
// index at outputDir/.crobot-index.md lists all files with stats.
func WriteDiffFiles(hunks []DiffHunk, stats DiffStats, outputDir string) error {
	// Ensure output directory exists.
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	// Group hunks by file.
	hunksByFile := make(map[string][]DiffHunk)
	for _, h := range hunks {
		hunksByFile[h.Path] = append(hunksByFile[h.Path], h)
	}

	// Write per-file diffs.
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolving output dir: %w", err)
	}
	for path, fileHunks := range hunksByFile {
		filePath := filepath.Join(absOutput, path)
		// Guard against path traversal from untrusted platform data.
		if rel, err := filepath.Rel(absOutput, filePath); err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("diff path %q escapes output directory", path)
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return fmt.Errorf("creating dir for %s: %w", path, err)
		}

		var b strings.Builder
		for i, h := range fileHunks {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n",
				h.OldStart, h.OldLines, h.NewStart, h.NewLines)
			b.WriteString(h.Body)
			if !strings.HasSuffix(h.Body, "\n") {
				b.WriteString("\n")
			}
		}

		if err := os.WriteFile(filePath, []byte(b.String()), 0o644); err != nil {
			return fmt.Errorf("writing diff for %s: %w", path, err)
		}
	}

	// Write index file.
	index := buildIndex(stats, outputDir)
	indexPath := filepath.Join(outputDir, ".crobot-index.md")
	if err := os.WriteFile(indexPath, []byte(index), 0o644); err != nil {
		return fmt.Errorf("writing index: %w", err)
	}

	return nil
}

// CleanupDiffDir removes the diff output directory.
func CleanupDiffDir(outputDir string) error {
	return os.RemoveAll(outputDir)
}

// buildIndex builds the markdown index file content.
func buildIndex(stats DiffStats, outputDir string) string {
	var b strings.Builder

	b.WriteString("# Diff Index\n\n")
	fmt.Fprintf(&b, "%d files changed, %d hunks total\n\n", stats.TotalFiles, stats.TotalHunks)
	b.WriteString("| File | Hunks | Size | Notes |\n")
	b.WriteString("|------|-------|------|-------|\n")

	for _, fs := range stats.FileStats {
		size := formatBytes(fs.BodyBytes)
		notes := lowValueNote(fs)
		fmt.Fprintf(&b, "| %s | %d | %s | %s |\n", fs.Path, fs.HunkCount, size, notes)
	}

	fmt.Fprintf(&b, "\nRead individual diffs at: %s/<file-path>\n", outputDir)

	return b.String()
}

// lowValueNote returns a short note for the index table if the file is low-value.
func lowValueNote(fs FileDiffStats) string {
	if !fs.IsLowValue {
		return ""
	}
	path := strings.ToLower(fs.Path)
	switch {
	case strings.Contains(path, "lock") || strings.HasSuffix(path, ".sum"):
		return "lock file"
	case strings.HasPrefix(path, "vendor/"):
		return "vendor"
	case strings.HasPrefix(path, "node_modules/"):
		return "node_modules"
	case strings.HasSuffix(path, ".pb.go"):
		return "protobuf generated"
	case strings.Contains(path, "_generated.") || strings.Contains(path, ".gen."):
		return "generated"
	case strings.HasSuffix(path, ".min.js") || strings.HasSuffix(path, ".min.css"):
		return "minified"
	default:
		return "low-value"
	}
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(b int) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	kb := float64(b) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.1f KB", kb)
	}
	mb := kb / 1024
	return fmt.Sprintf("%.1f MB", mb)
}
