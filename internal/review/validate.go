// Package review implements the review engine: validate, dedupe, render, and
// post review findings as inline comments on pull requests.
package review

import (
	"fmt"
	"strings"

	"github.com/dizzyc/crobot/internal/platform"
)

// severityRank maps severity strings to numeric ranks for threshold comparison.
// Higher rank means more severe.
var severityRank = map[string]int{
	"info":    0,
	"warning": 1,
	"error":   2,
}

// RejectedFinding pairs a finding with the reason it was rejected.
type RejectedFinding struct {
	Finding platform.ReviewFinding
	Reason  string
}

// ValidateFindings checks findings against the PR context and returns validated
// and rejected findings. A finding is valid if:
//  1. It passes basic field validation (ReviewFinding.Validate)
//  2. Its severity meets or exceeds the configured threshold
//  3. Its path matches a file in the PR's changed files
//  4. Its line falls within a diff hunk range for that path and side
func ValidateFindings(findings []platform.ReviewFinding, ctx *platform.PRContext, severityThreshold string) (valid []platform.ReviewFinding, rejected []RejectedFinding) {
	// Build a set of changed file paths for fast lookup.
	changedFiles := make(map[string]bool, len(ctx.Files))
	for _, f := range ctx.Files {
		changedFiles[f.Path] = true
	}

	// Build a map from path to its diff hunks.
	hunksByPath := make(map[string][]platform.DiffHunk)
	for _, h := range ctx.DiffHunks {
		hunksByPath[h.Path] = append(hunksByPath[h.Path], h)
	}

	thresholdRank := severityRank[strings.ToLower(severityThreshold)]

	for _, f := range findings {
		if err := f.Validate(); err != nil {
			rejected = append(rejected, RejectedFinding{
				Finding: f,
				Reason:  fmt.Sprintf("validation error: %v", err),
			})
			continue
		}

		// Check severity threshold.
		rank, ok := severityRank[f.Severity]
		if !ok {
			rejected = append(rejected, RejectedFinding{
				Finding: f,
				Reason:  fmt.Sprintf("unknown severity: %q", f.Severity),
			})
			continue
		}
		if rank < thresholdRank {
			rejected = append(rejected, RejectedFinding{
				Finding: f,
				Reason:  fmt.Sprintf("severity %q below threshold %q", f.Severity, severityThreshold),
			})
			continue
		}

		// Check path exists in changed files.
		if !changedFiles[f.Path] {
			rejected = append(rejected, RejectedFinding{
				Finding: f,
				Reason:  fmt.Sprintf("path %q not in changed files", f.Path),
			})
			continue
		}

		// Check line falls within a diff hunk for this path and side.
		hunks := hunksByPath[f.Path]
		if !lineInHunks(f.Line, f.Side, hunks) {
			rejected = append(rejected, RejectedFinding{
				Finding: f,
				Reason:  fmt.Sprintf("line %d (side %q) not within any diff hunk for %q", f.Line, f.Side, f.Path),
			})
			continue
		}

		valid = append(valid, f)
	}

	return valid, rejected
}

// lineInHunks reports whether the given line number falls within any of the
// provided diff hunks for the specified side.
//
// For side="new": line must be in [NewStart, NewStart+NewLines).
// For side="old": line must be in [OldStart, OldStart+OldLines).
func lineInHunks(line int, side string, hunks []platform.DiffHunk) bool {
	for _, h := range hunks {
		switch side {
		case "new":
			if line >= h.NewStart && line < h.NewStart+h.NewLines {
				return true
			}
		case "old":
			if line >= h.OldStart && line < h.OldStart+h.OldLines {
				return true
			}
		}
	}
	return false
}
