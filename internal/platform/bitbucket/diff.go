package bitbucket

import "github.com/cristian-fleischer/crobot/internal/platform"

// parseDiff delegates to the shared unified diff parser. Both Bitbucket and
// GitHub produce standard unified diffs.
func parseDiff(raw string) ([]platform.DiffHunk, error) {
	return platform.ParseDiff(raw)
}
