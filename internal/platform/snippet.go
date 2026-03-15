package platform

import (
	"fmt"
	"strings"
)

// SnippetOutput represents a file snippet with surrounding context.
type SnippetOutput struct {
	Path      string `json:"path"`
	Commit    string `json:"commit"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
}

// ExtractSnippet extracts a snippet from file content centered on the given
// line with contextSize lines above and below. Line numbers are 1-based.
func ExtractSnippet(content []byte, path, commit string, line, contextSize int) (*SnippetOutput, error) {
	lines := strings.Split(string(content), "\n")
	// Trim trailing empty element from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("file %q is empty", path)
	}
	if line > len(lines) {
		return nil, fmt.Errorf("line %d is out of range (file has %d lines)", line, len(lines))
	}

	startLine := line - contextSize
	if startLine < 1 {
		startLine = 1
	}
	endLine := line + contextSize
	if endLine > len(lines) {
		endLine = len(lines)
	}

	snippet := strings.Join(lines[startLine-1:endLine], "\n")

	return &SnippetOutput{
		Path:      path,
		Commit:    commit,
		StartLine: startLine,
		EndLine:   endLine,
		Content:   snippet,
	}, nil
}
