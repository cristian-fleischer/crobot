package platform

import (
	"path/filepath"
	"strings"
)

// DiffStats contains aggregate statistics about a set of diff hunks.
type DiffStats struct {
	TotalFiles     int             `json:"total_files"`
	TotalHunks     int             `json:"total_hunks"`
	TotalBodyBytes int             `json:"total_body_bytes"`
	FileStats      []FileDiffStats `json:"file_stats"`
}

// FileDiffStats contains per-file statistics for a set of diff hunks.
type FileDiffStats struct {
	Path       string `json:"path"`
	HunkCount  int    `json:"hunk_count"`
	BodyBytes  int    `json:"body_bytes"`
	IsLowValue bool   `json:"is_low_value"`
}

// lowValueGlobs are filename-level glob patterns that match files unlikely to
// contain meaningful review findings (lock files, generated code, etc.).
var lowValueGlobs = []string{
	"*lock*",
	"*.sum",
	"*.pb.go",
	"*_generated.*",
	"*.gen.*",
	"*.min.js",
	"*.min.css",
}

// lowValuePrefixes are directory prefixes that match vendored or dependency files.
var lowValuePrefixes = []string{
	"vendor/",
	"node_modules/",
}

// ComputeDiffStats computes aggregate and per-file statistics from a set of
// diff hunks. Files whose paths match low-value patterns (lock files, generated
// code, vendored dependencies) are flagged.
func ComputeDiffStats(hunks []DiffHunk) DiffStats {
	type fileAcc struct {
		hunkCount int
		bodyBytes int
	}

	// Accumulate per-file stats preserving first-seen order.
	var order []string
	acc := make(map[string]*fileAcc)

	for _, h := range hunks {
		a, ok := acc[h.Path]
		if !ok {
			a = &fileAcc{}
			acc[h.Path] = a
			order = append(order, h.Path)
		}
		a.hunkCount++
		a.bodyBytes += len(h.Body)
	}

	stats := DiffStats{
		TotalFiles: len(order),
		TotalHunks: len(hunks),
	}

	for _, path := range order {
		a := acc[path]
		fs := FileDiffStats{
			Path:       path,
			HunkCount:  a.hunkCount,
			BodyBytes:  a.bodyBytes,
			IsLowValue: isLowValue(path),
		}
		stats.FileStats = append(stats.FileStats, fs)
		stats.TotalBodyBytes += a.bodyBytes
	}

	return stats
}

// isLowValue returns true if the file path matches any low-value pattern.
func isLowValue(path string) bool {
	// Check directory prefixes (vendor/, node_modules/).
	for _, prefix := range lowValuePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	// Check filename-level globs against the base name.
	base := filepath.Base(path)
	for _, pattern := range lowValueGlobs {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}
