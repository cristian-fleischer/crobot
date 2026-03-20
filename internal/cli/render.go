package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/review"
	"github.com/mkozhukh/mdterm"
)

// ANSI color codes for diff rendering.
const (
	ansiGreen  = "\033[32m"
	ansiRed    = "\033[31m"
	ansiBold   = "\033[1m"
)

// contextLine represents a single line extracted from a diff hunk with its
// computed line number and prefix.
type contextLine struct {
	OldNum   int    // line number on the old side (0 if not applicable)
	NewNum   int    // line number on the new side (0 if not applicable)
	Prefix   string // " ", "+", or "-"
	Text     string // line content without the prefix
	IsTarget bool   // true if this is the finding's target line
}

// RenderFindings prints review findings to w with diff context snippets above
// each comment, similar to how code review platforms display inline comments.
func RenderFindings(posted []review.PostedComment, hunks []platform.DiffHunk, w io.Writer, raw bool) {
	fmt.Fprintf(w, "\n%s--- REVIEW COMMENTS (%d) ---%s\n", ansiDim, len(posted), ansiReset)

	var mdParser *mdterm.Parser
	if !raw {
		mdParser = mdterm.New(w)
	}

	for i, p := range posted {
		// File path header.
		fmt.Fprintf(w, "\n%s%s%s\n", ansiDim, p.Finding.Path, ansiReset)

		// Diff context snippet.
		lines := extractDiffContext(p.Finding.Line, p.Finding.Side, p.Finding.Path, hunks, 3)
		if len(lines) > 0 {
			renderDiffSnippet(w, lines)
		}

		// Comment body.
		fmt.Fprintln(w)
		if raw || mdParser == nil {
			fmt.Fprintf(w, "%s\n", p.RenderedBody)
		} else {
			mdParser.Render(p.RenderedBody)
		}

		if i < len(posted)-1 {
			fmt.Fprintf(w, "\n%s───%s\n", ansiDim, ansiReset)
		}
	}
	fmt.Fprintf(w, "\n%s--- END REVIEW COMMENTS ---%s\n", ansiDim, ansiReset)
}

// renderDiffSnippet renders a few context lines from the diff with line numbers
// and ANSI coloring: green for additions, red for deletions, dim line numbers.
func renderDiffSnippet(w io.Writer, lines []contextLine) {
	// Determine the max line number width for alignment.
	maxNum := 0
	for _, l := range lines {
		if l.OldNum > maxNum {
			maxNum = l.OldNum
		}
		if l.NewNum > maxNum {
			maxNum = l.NewNum
		}
	}
	numWidth := len(fmt.Sprintf("%d", maxNum))
	if numWidth < 3 {
		numWidth = 3
	}

	for _, l := range lines {
		// Line numbers (old and new side, dimmed).
		oldStr := strings.Repeat(" ", numWidth)
		newStr := strings.Repeat(" ", numWidth)
		if l.OldNum > 0 {
			oldStr = fmt.Sprintf("%*d", numWidth, l.OldNum)
		}
		if l.NewNum > 0 {
			newStr = fmt.Sprintf("%*d", numWidth, l.NewNum)
		}

		// Color based on prefix.
		color := ""
		resetColor := ""
		switch l.Prefix {
		case "+":
			color = ansiGreen
			resetColor = ansiReset
		case "-":
			color = ansiRed
			resetColor = ansiReset
		}

		bold := ""
		resetBold := ""
		if l.IsTarget {
			bold = ansiBold
			resetBold = ansiReset + color // restore color after bold reset
			if color == "" {
				resetBold = ansiReset
			}
		}

		fmt.Fprintf(w, "%s%s %s%s %s%s%s%s%s%s\n",
			ansiDim, oldStr, newStr, ansiReset,
			color, bold, l.Prefix, " "+l.Text, resetBold, resetColor)
	}
}

// extractDiffContext finds the diff hunk containing the given line and returns
// a window of context lines around the target line.
func extractDiffContext(line int, side, path string, hunks []platform.DiffHunk, contextSize int) []contextLine {
	// Find the matching hunk.
	var hunk *platform.DiffHunk
	for i := range hunks {
		h := &hunks[i]
		if h.Path != path {
			continue
		}
		if side == "new" {
			if line >= h.NewStart && line < h.NewStart+h.NewLines {
				hunk = h
				break
			}
		} else {
			if line >= h.OldStart && line < h.OldStart+h.OldLines {
				hunk = h
				break
			}
		}
	}
	if hunk == nil {
		return nil
	}

	// Parse hunk body into context lines with line numbers.
	bodyLines := strings.Split(hunk.Body, "\n")
	var parsed []contextLine
	targetIdx := -1

	oldLine := hunk.OldStart
	newLine := hunk.NewStart

	for _, raw := range bodyLines {
		if raw == "" {
			continue
		}
		// Skip the @@ header if present in body.
		if strings.HasPrefix(raw, "@@") {
			continue
		}
		// Handle no-newline-at-end marker.
		if strings.HasPrefix(raw, "\\ No newline") {
			continue
		}

		prefix := string(raw[0])
		text := raw[1:]

		cl := contextLine{Prefix: prefix, Text: text}

		switch prefix {
		case "+":
			cl.NewNum = newLine
			if side == "new" && newLine == line {
				cl.IsTarget = true
				targetIdx = len(parsed)
			}
			newLine++
		case "-":
			cl.OldNum = oldLine
			if side == "old" && oldLine == line {
				cl.IsTarget = true
				targetIdx = len(parsed)
			}
			oldLine++
		default: // context line " "
			cl.OldNum = oldLine
			cl.NewNum = newLine
			if side == "new" && newLine == line {
				cl.IsTarget = true
				targetIdx = len(parsed)
			} else if side == "old" && oldLine == line {
				cl.IsTarget = true
				targetIdx = len(parsed)
			}
			oldLine++
			newLine++
		}

		parsed = append(parsed, cl)
	}

	if targetIdx < 0 || len(parsed) == 0 {
		return nil
	}

	// Extract a window around the target.
	start := targetIdx - contextSize
	if start < 0 {
		start = 0
	}
	end := targetIdx + contextSize + 1
	if end > len(parsed) {
		end = len(parsed)
	}
	return parsed[start:end]
}
