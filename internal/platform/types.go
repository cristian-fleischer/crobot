// Package platform defines the abstractions and shared types for interacting
// with code-hosting platforms (Bitbucket, GitHub, GitLab, etc.).
package platform

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// PRRequest identifies a pull request on a platform.
type PRRequest struct {
	Workspace string `json:"workspace"`
	Repo      string `json:"repo"`
	PRNumber  int    `json:"pr_number"`
}

// FileRequest identifies a specific file at a specific commit.
type FileRequest struct {
	Workspace string `json:"workspace"`
	Repo      string `json:"repo"`
	Commit    string `json:"commit"`
	Path      string `json:"path"`
}

// PRContext contains normalized metadata, changed files, and diff hunks for a
// pull request.
type PRContext struct {
	ID           int           `json:"id"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	Author       string        `json:"author"`
	SourceBranch string        `json:"source_branch"`
	TargetBranch string        `json:"target_branch"`
	State        string        `json:"state"`
	HeadCommit   string        `json:"head_commit"`
	BaseCommit   string        `json:"base_commit"`
	Files        []ChangedFile `json:"files"`
	DiffHunks    []DiffHunk    `json:"diff_hunks"`
}

// ChangedFile represents a file that was modified in a pull request.
type ChangedFile struct {
	Path    string `json:"path"`
	OldPath string `json:"old_path,omitempty"`
	Status  string `json:"status"`
}

// DiffHunk represents a single hunk from a unified diff.
type DiffHunk struct {
	Path     string `json:"path"`
	OldStart int    `json:"old_start"`
	OldLines int    `json:"old_lines"`
	NewStart int    `json:"new_start"`
	NewLines int    `json:"new_lines"`
	Body     string `json:"body"`
}

// ReviewFinding represents a single review finding produced by the AI reviewer.
type ReviewFinding struct {
	Path          string   `json:"path"`
	Line          int      `json:"line"`
	Side          string   `json:"side"`
	Severity      string   `json:"severity"`
	SeverityScore int      `json:"severity_score,omitempty"`
	Category      string   `json:"category"`
	Criteria      []string `json:"criteria,omitempty"`
	Message       string   `json:"message"`
	Suggestion    string   `json:"suggestion,omitempty"`
	Fingerprint   string   `json:"fingerprint"`
}

// validSides is the set of allowed values for ReviewFinding.Side.
var validSides = map[string]bool{
	"new": true,
	"old": true,
}

// validSeverities is the set of allowed values for ReviewFinding.Severity.
var validSeverities = map[string]bool{
	"info":    true,
	"warning": true,
	"error":   true,
}

// Validate checks that the ReviewFinding has all required fields set to
// acceptable values. It returns an error describing the first validation
// failure encountered, or nil if the finding is valid.
func (f ReviewFinding) Validate() error {
	if f.Path == "" {
		return fmt.Errorf("review finding: %w", ErrEmptyPath)
	}
	if f.Line <= 0 {
		return fmt.Errorf("review finding: %w: got %d", ErrInvalidLine, f.Line)
	}
	if !validSides[f.Side] {
		return fmt.Errorf("review finding: %w: got %q", ErrInvalidSide, f.Side)
	}
	if !validSeverities[f.Severity] {
		return fmt.Errorf("review finding: %w: got %q", ErrInvalidSeverity, f.Severity)
	}
	if f.SeverityScore < 0 || f.SeverityScore > 10 {
		return fmt.Errorf("review finding: %w: got %d", ErrInvalidSeverityScore, f.SeverityScore)
	}
	if f.Message == "" {
		return fmt.Errorf("review finding: %w", ErrEmptyMessage)
	}
	return nil
}

// ParseFindings parses a JSON-encoded array of ReviewFinding values from data.
// It returns an error if the JSON is malformed or cannot be decoded into
// []ReviewFinding.
func ParseFindings(data []byte) ([]ReviewFinding, error) {
	var findings []ReviewFinding
	if err := json.Unmarshal(data, &findings); err != nil {
		return nil, fmt.Errorf("parsing findings: %w", err)
	}
	return findings, nil
}

// InlineComment represents a comment to be posted on a specific line of a file
// in a pull request.
type InlineComment struct {
	Path        string `json:"path"`
	Line        int    `json:"line"`
	Side        string `json:"side"`
	Body        string `json:"body"`
	Fingerprint string `json:"fingerprint"`
}

// Comment represents an existing comment on a pull request.
type Comment struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Line        int    `json:"line"`
	Body        string `json:"body"`
	Author      string `json:"author"`
	CreatedAt   string `json:"created_at"`
	IsBot       bool   `json:"is_bot"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// fingerprintRe matches the hidden CRoBot fingerprint in the markdown
// reference-link comment format: [//]: # "crobot:fp=VALUE"
// This format is invisible in rendered markdown across all major platforms.
var fingerprintRe = regexp.MustCompile(`\[//\]: # "crobot:fp=(.+?)"`)

// ExtractFingerprint extracts the CRoBot fingerprint from a comment body. It
// returns an empty string if no fingerprint marker is found.
func ExtractFingerprint(body string) string {
	matches := fingerprintRe.FindStringSubmatch(body)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}
