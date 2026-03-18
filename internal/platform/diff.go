package platform

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseDiff parses a unified diff string into a slice of DiffHunk values. It
// handles regular hunks, file renames, binary files, new files, and deleted
// files. This function is shared across platform adapters since GitHub and
// Bitbucket both produce standard unified diffs.
func ParseDiff(raw string) ([]DiffHunk, error) {
	var hunks []DiffHunk
	lines := strings.Split(raw, "\n")

	var currentPath string
	i := 0

	for i < len(lines) {
		line := lines[i]

		// Detect file header: "diff --git a/path b/path"
		if strings.HasPrefix(line, "diff --git ") {
			currentPath = parseDiffFilePath(line)
			i++

			// Scan file-level headers (index, old mode, new mode, etc.)
			for i < len(lines) && !strings.HasPrefix(lines[i], "diff --git ") {
				if strings.HasPrefix(lines[i], "@@") {
					break
				}

				// Handle renames: look for "rename to" or "+++ b/path"
				if strings.HasPrefix(lines[i], "rename to ") {
					currentPath = strings.TrimPrefix(lines[i], "rename to ")
				} else if strings.HasPrefix(lines[i], "+++ b/") {
					currentPath = strings.TrimPrefix(lines[i], "+++ b/")
				} else if strings.HasPrefix(lines[i], "+++ /dev/null") {
					// deleted file -- keep original path
				}

				// Binary files
				if strings.HasPrefix(lines[i], "Binary files") || strings.HasPrefix(lines[i], "GIT binary patch") {
					hunks = append(hunks, DiffHunk{
						Path: currentPath,
						Body: lines[i],
					})
				}

				i++
			}
			continue
		}

		// Parse hunk header: "@@ -old_start,old_lines +new_start,new_lines @@"
		if strings.HasPrefix(line, "@@") {
			hunk, consumed, err := parseDiffHunk(currentPath, lines, i)
			if err != nil {
				return nil, fmt.Errorf("parsing hunk at line %d: %w", i, err)
			}
			hunks = append(hunks, hunk)
			i = consumed
			continue
		}

		i++
	}

	return hunks, nil
}

// parseDiffFilePath extracts the file path from a "diff --git a/path b/path"
// line. It splits from the right to handle paths that contain " b/" as a
// directory component (e.g., "lib/build/foo.go").
func parseDiffFilePath(diffLine string) string {
	idx := strings.LastIndex(diffLine, " b/")
	if idx >= 0 {
		return diffLine[idx+3:]
	}
	return ""
}

// parseDiffHunk parses a single hunk starting at the @@ line and returns the
// hunk, the index of the next line after the hunk, and any error.
func parseDiffHunk(path string, lines []string, start int) (DiffHunk, int, error) {
	header := lines[start]

	oldStart, oldLines, newStart, newLines, err := parseDiffHunkHeader(header)
	if err != nil {
		return DiffHunk{}, 0, err
	}

	// Collect hunk body lines
	var bodyLines []string
	bodyLines = append(bodyLines, header)

	i := start + 1
	for i < len(lines) {
		l := lines[i]
		// Stop at next hunk header or next file diff
		if strings.HasPrefix(l, "@@") || strings.HasPrefix(l, "diff --git ") {
			break
		}
		bodyLines = append(bodyLines, l)
		i++
	}

	// Trim trailing empty line that comes from the final \n in the split
	if len(bodyLines) > 1 && bodyLines[len(bodyLines)-1] == "" {
		bodyLines = bodyLines[:len(bodyLines)-1]
	}

	return DiffHunk{
		Path:     path,
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
		Body:     strings.Join(bodyLines, "\n"),
	}, i, nil
}

// parseDiffHunkHeader parses the "@@ -old_start,old_lines +new_start,new_lines @@"
// header line and returns the four integer values.
func parseDiffHunkHeader(header string) (oldStart, oldLines, newStart, newLines int, err error) {
	if !strings.HasPrefix(header, "@@") {
		return 0, 0, 0, 0, fmt.Errorf("not a hunk header: %q", header)
	}

	end := strings.Index(header[2:], "@@")
	if end < 0 {
		return 0, 0, 0, 0, fmt.Errorf("malformed hunk header (no closing @@): %q", header)
	}
	rangePart := strings.TrimSpace(header[2 : 2+end])

	parts := strings.Fields(rangePart)
	if len(parts) < 2 {
		return 0, 0, 0, 0, fmt.Errorf("malformed hunk header (need 2 range parts): %q", header)
	}

	oldStart, oldLines, err = parseDiffRange(parts[0], "-")
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("parsing old range: %w", err)
	}

	newStart, newLines, err = parseDiffRange(parts[1], "+")
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("parsing new range: %w", err)
	}

	return oldStart, oldLines, newStart, newLines, nil
}

// parseDiffRange parses a range like "-10,5" or "+10,7" (prefix is "-" or "+").
// If the count is omitted (e.g. "-10"), it defaults to 1.
func parseDiffRange(s, prefix string) (start, count int, err error) {
	s = strings.TrimPrefix(s, prefix)
	parts := strings.SplitN(s, ",", 2)

	start, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing start: %w", err)
	}

	if len(parts) == 2 {
		count, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("parsing count: %w", err)
		}
	} else {
		count = 1
	}

	return start, count, nil
}
