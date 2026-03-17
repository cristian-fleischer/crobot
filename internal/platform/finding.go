package platform

import (
	"encoding/json"
	"fmt"
)

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
	if f.Category == "" {
		return fmt.Errorf("review finding: %w", ErrEmptyCategory)
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
