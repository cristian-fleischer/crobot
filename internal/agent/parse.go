package agent

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

// fencedJSONRe matches markdown-fenced JSON code blocks.
var fencedJSONRe = regexp.MustCompile("(?s)```(?:json)?\\s*\n(.*?)```")

// ExtractFindings parses agent output text and extracts ReviewFinding[] JSON.
// It handles raw JSON arrays, markdown-fenced JSON, and JSON embedded in
// surrounding text. It returns a descriptive error if no valid JSON array of
// findings can be extracted.
func ExtractFindings(agentOutput string) ([]platform.ReviewFinding, error) {
	text := strings.TrimSpace(agentOutput)
	if text == "" {
		return nil, fmt.Errorf("agent: extract findings: empty output")
	}

	// 1. Try parsing the entire output as a JSON array.
	if findings, err := platform.ParseFindings([]byte(text)); err == nil {
		return findings, nil
	}

	// 2. Look for markdown code fences and try each.
	matches := fencedJSONRe.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		content := strings.TrimSpace(match[1])
		if findings, err := platform.ParseFindings([]byte(content)); err == nil {
			return findings, nil
		}
	}

	// 3. Scan for bare JSON arrays at the top level.
	if findings, ok := extractBareArray(text); ok {
		return findings, nil
	}

	// Nothing worked — build a descriptive error.
	if len(matches) > 0 {
		return nil, fmt.Errorf("agent: extract findings: found %d code fence(s) but none contained valid ReviewFinding[] JSON", len(matches))
	}
	if strings.Contains(text, "[") {
		return nil, fmt.Errorf("agent: extract findings: found JSON-like content but could not parse as ReviewFinding[]")
	}
	return nil, fmt.Errorf("agent: extract findings: no JSON array found in agent output")
}

// extractBareArray scans the text for top-level bracket-delimited JSON arrays
// and attempts to parse each as ReviewFinding[].
func extractBareArray(text string) ([]platform.ReviewFinding, bool) {
	for i := 0; i < len(text); i++ {
		if text[i] != '[' {
			continue
		}
		// Find the matching closing bracket, tracking nesting depth.
		depth := 0
		inString := false
		escaped := false
		for j := i; j < len(text); j++ {
			ch := text[j]
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && inString {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if ch == '[' {
				depth++
			} else if ch == ']' {
				depth--
				if depth == 0 {
					candidate := text[i : j+1]
					if findings, err := platform.ParseFindings([]byte(candidate)); err == nil {
						return findings, true
					}
					break
				}
			}
		}
	}
	return nil, false
}
