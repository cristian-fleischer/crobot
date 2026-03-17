package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

// FSHandler handles filesystem requests from agent subprocesses. It provides
// read-only access to files at a specific commit using git.
type FSHandler struct {
	headCommit string
	repoDir    string
}

// validCommitHash matches a lowercase hex string of 4–40 characters, which
// covers both short and full Git commit hashes.
var validCommitHash = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

// NewFSHandler creates a new FSHandler that reads files at the given commit
// from the given repository directory. It returns an error if the commit hash
// is not a valid hex string.
func NewFSHandler(headCommit, repoDir string) (*FSHandler, error) {
	if !validCommitHash.MatchString(headCommit) {
		return nil, fmt.Errorf("agent: fs: invalid commit hash %q: must be 4-40 lowercase hex characters", headCommit)
	}
	return &FSHandler{
		headCommit: headCommit,
		repoDir:    repoDir,
	}, nil
}

// HandleRequest routes incoming filesystem requests from the agent.
func (h *FSHandler) HandleRequest(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case "fs/read_text_file":
		return h.readTextFile(ctx, params)
	case "fs/write_text_file":
		return nil, fmt.Errorf("agent: fs: write operations are not permitted")
	case "terminal/run":
		return nil, fmt.Errorf("agent: fs: terminal operations are not permitted")
	default:
		return nil, fmt.Errorf("agent: fs: unknown method: %s", method)
	}
}

// readTextFileParams are the parameters for the fs/read_text_file method.
type readTextFileParams struct {
	Path string `json:"path"`
}

// readTextFile reads a file at the configured commit using git show.
func (h *FSHandler) readTextFile(ctx context.Context, params json.RawMessage) (any, error) {
	var p readTextFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("agent: fs: parsing read params: %w", err)
	}

	if p.Path == "" {
		return nil, fmt.Errorf("agent: fs: path must not be empty")
	}

	// Sanitize the path to prevent directory traversal.
	cleaned := path.Clean(p.Path)
	if cleaned == ".." || cleaned == "." || path.IsAbs(cleaned) || strings.HasPrefix(cleaned, "../") {
		return nil, fmt.Errorf("agent: fs: invalid path %q", p.Path)
	}
	p.Path = cleaned

	// Use git show to read the file at the specific commit.
	ref := fmt.Sprintf("%s:%s", h.headCommit, p.Path)
	cmd := exec.CommandContext(ctx, "git", "show", ref)
	cmd.Dir = h.repoDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("agent: fs: reading %s: %w", p.Path, err)
	}

	return map[string]string{
		"content": string(output),
	}, nil
}
