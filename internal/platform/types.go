package platform

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
	IsResolved  bool   `json:"is_resolved"`
	Fingerprint string `json:"fingerprint,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
}
