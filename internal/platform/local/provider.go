// Package local provides a platform.Platform implementation that works
// entirely from the local git repository, without requiring a remote platform
// or credentials. It is used for pre-push local code reviews.
package local

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

// Provider implements platform.Platform using local git commands.
// It diffs the working tree (including staged and unstaged changes) against
// a base branch to produce a PRContext for review.
type Provider struct {
	baseBranch  string
	repoDir     string
	uncommitted bool
}

// New creates a local provider that diffs against the given base branch.
// repoDir should be the root of the git repository (typically ".").
func New(baseBranch, repoDir string) *Provider {
	return &Provider{
		baseBranch: baseBranch,
		repoDir:    repoDir,
	}
}

// NewUncommitted creates a local provider that diffs only uncommitted changes
// (staged and unstaged) against HEAD.
func NewUncommitted(repoDir string) *Provider {
	return &Provider{
		repoDir:     repoDir,
		uncommitted: true,
	}
}

// GetPRContext builds a PRContext from local git state. By default it diffs the
// working tree against the merge-base of the base branch, capturing all
// committed, staged, and unstaged changes. When uncommitted mode is enabled, it
// diffs only uncommitted changes (staged + unstaged) against HEAD.
func (p *Provider) GetPRContext(ctx context.Context, _ platform.PRRequest) (*platform.PRContext, error) {
	if err := p.validate(ctx); err != nil {
		return nil, err
	}

	head, err := p.git(ctx, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolving HEAD: %w", err)
	}

	branch, err := p.git(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		branch = "HEAD"
	}

	author, _ := p.git(ctx, "config", "user.name")

	// Determine the diff base: HEAD for uncommitted mode, merge-base otherwise.
	diffBase := head
	targetBranch := branch
	title := "Local review (uncommitted): " + branch
	if !p.uncommitted {
		mergeBase, err := p.git(ctx, "merge-base", p.baseBranch, "HEAD")
		if err != nil {
			return nil, fmt.Errorf("finding merge-base with %s: %w (is %q a valid branch?)", p.baseBranch, err, p.baseBranch)
		}
		diffBase = mergeBase
		targetBranch = p.baseBranch
		title = "Local review: " + branch
	}

	nameStatus, err := p.git(ctx, "diff", "--name-status", diffBase)
	if err != nil {
		return nil, fmt.Errorf("getting changed files: %w", err)
	}
	files := parseNameStatus(nameStatus)

	rawDiff, err := p.git(ctx, "diff", diffBase)
	if err != nil {
		return nil, fmt.Errorf("getting diff: %w", err)
	}

	hunks, err := platform.ParseDiff(rawDiff)
	if err != nil {
		return nil, fmt.Errorf("parsing diff: %w", err)
	}

	return &platform.PRContext{
		ID:           0,
		Title:        title,
		Author:       author,
		SourceBranch: branch,
		TargetBranch: targetBranch,
		State:        "local",
		HeadCommit:   head,
		BaseCommit:   diffBase,
		Files:        files,
		DiffHunks:    hunks,
	}, nil
}

// GetFileContent returns file content at a specific commit via git show.
func (p *Provider) GetFileContent(ctx context.Context, opts platform.FileRequest) ([]byte, error) {
	content, err := p.git(ctx, "show", opts.Commit+":"+opts.Path)
	if err != nil {
		return nil, fmt.Errorf("reading %s at %s: %w", opts.Path, opts.Commit, err)
	}
	return []byte(content), nil
}

// ListBotComments returns an empty slice — there are no existing comments in local mode.
func (p *Provider) ListBotComments(_ context.Context, _ platform.PRRequest) ([]platform.Comment, error) {
	return nil, nil
}

// ListPRComments returns an empty slice — there are no existing comments in local mode.
func (p *Provider) ListPRComments(_ context.Context, _ platform.PRRequest) ([]platform.Comment, error) {
	return nil, nil
}

// CreateInlineComment is not supported in local mode.
func (p *Provider) CreateInlineComment(_ context.Context, _ platform.PRRequest, _ platform.InlineComment) (*platform.Comment, error) {
	return nil, fmt.Errorf("local mode does not support posting comments")
}

// DeleteComment is not supported in local mode.
func (p *Provider) DeleteComment(_ context.Context, _ platform.PRRequest, _ string) error {
	return fmt.Errorf("local mode does not support deleting comments")
}

// validate checks that we're in a git repository and the base branch exists.
func (p *Provider) validate(ctx context.Context) error {
	if _, err := p.git(ctx, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}
	// Verify the base branch is resolvable (not needed in uncommitted mode).
	if !p.uncommitted {
		if _, err := p.git(ctx, "rev-parse", "--verify", p.baseBranch); err != nil {
			return fmt.Errorf("base branch %q not found: %w", p.baseBranch, err)
		}
	}
	return nil
}

// git runs a git command and returns trimmed stdout.
func (p *Provider) git(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = p.repoDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// parseNameStatus parses `git diff --name-status` output into ChangedFile values.
func parseNameStatus(output string) []platform.ChangedFile {
	if output == "" {
		return nil
	}
	var files []platform.ChangedFile
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		status := parts[0]
		path := parts[len(parts)-1]

		cf := platform.ChangedFile{Path: path}
		switch {
		case status == "A":
			cf.Status = "added"
		case status == "D":
			cf.Status = "deleted"
		case status == "M":
			cf.Status = "modified"
		case strings.HasPrefix(status, "R"):
			cf.Status = "renamed"
			if len(parts) >= 3 {
				cf.OldPath = parts[1]
				cf.Path = parts[2]
			}
		default:
			cf.Status = "modified"
		}
		files = append(files, cf)
	}
	return files
}

// RepoName returns the basename of the repository directory.
func (p *Provider) RepoName() string {
	abs, err := filepath.Abs(p.repoDir)
	if err != nil {
		return "local"
	}
	return filepath.Base(abs)
}
