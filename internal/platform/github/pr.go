package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

// ghPullRequest is the GitHub API representation of a pull request.
type ghPullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"` // "open" or "closed"
	Merged bool   `json:"merged"`
	User   ghUser `json:"user"`
	Head   ghRef  `json:"head"`
	Base   ghRef  `json:"base"`
}

// ghUser is the GitHub API representation of a user.
type ghUser struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
	Type  string `json:"type"` // "User", "Bot", "Organization"
}

// ghRef is a branch reference in a pull request.
type ghRef struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

// ghDiffEntry is a single entry in the GitHub pull request files response.
type ghDiffEntry struct {
	Filename         string `json:"filename"`
	Status           string `json:"status"`
	PreviousFilename string `json:"previous_filename,omitempty"`
}

// GetPRContext fetches pull request metadata, changed files, and raw diff from
// the GitHub REST API and returns a normalized PRContext.
func (c *Client) GetPRContext(ctx context.Context, opts platform.PRRequest) (*platform.PRContext, error) {
	owner := c.resolveOwner(opts.Workspace)

	basePath := fmt.Sprintf("/repos/%s/%s/pulls/%d",
		url.PathEscape(owner), url.PathEscape(opts.Repo), opts.PRNumber)

	// Fetch PR metadata.
	prBody, _, err := c.do(ctx, "GET", basePath, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching PR metadata: %w", err)
	}

	var pr ghPullRequest
	if err := json.Unmarshal(prBody, &pr); err != nil {
		return nil, fmt.Errorf("decoding PR metadata: %w", err)
	}

	// Fetch changed files (paginated).
	files, err := c.fetchFiles(ctx, basePath+"/files")
	if err != nil {
		return nil, fmt.Errorf("fetching changed files: %w", err)
	}

	// Fetch raw diff using Accept header.
	diffHunks, err := c.fetchDiff(ctx, basePath)
	if err != nil {
		return nil, fmt.Errorf("fetching diff: %w", err)
	}

	return &platform.PRContext{
		ID:           pr.Number,
		Title:        pr.Title,
		Description:  pr.Body,
		Author:       pr.User.Login,
		SourceBranch: pr.Head.Ref,
		TargetBranch: pr.Base.Ref,
		State:        mapPRState(pr.State, pr.Merged),
		HeadCommit:   pr.Head.SHA,
		BaseCommit:   pr.Base.SHA,
		Files:        files,
		DiffHunks:    diffHunks,
	}, nil
}

// fetchFiles retrieves the changed files for a pull request, following Link
// header pagination.
func (c *Client) fetchFiles(ctx context.Context, path string) ([]platform.ChangedFile, error) {
	var allFiles []platform.ChangedFile

	data, headers, err := c.do(ctx, "GET", formatPerPage(path), nil)
	if err != nil {
		return nil, err
	}

	for {
		var entries []ghDiffEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, fmt.Errorf("decoding files page: %w", err)
		}

		for _, e := range entries {
			status := normalizeFileStatus(e.Status)
			if status == "" {
				continue // skip "unchanged" entries
			}
			cf := platform.ChangedFile{
				Path:   e.Filename,
				Status: status,
			}
			if e.PreviousFilename != "" && e.PreviousFilename != e.Filename {
				cf.OldPath = e.PreviousFilename
			}
			allFiles = append(allFiles, cf)
		}

		next := parseLinkNext(headers)
		if next == "" {
			break
		}

		data, headers, err = c.doURL(ctx, next)
		if err != nil {
			return nil, fmt.Errorf("fetching files next page: %w", err)
		}
	}

	return allFiles, nil
}

// fetchDiff retrieves the raw unified diff for a pull request using the
// Accept: application/vnd.github.diff header on the PR endpoint.
func (c *Client) fetchDiff(ctx context.Context, path string) ([]platform.DiffHunk, error) {
	resp, err := c.doRaw(ctx, "GET", path, "application/vnd.github.diff")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return nil, fmt.Errorf("reading diff body: %w", err)
	}

	return platform.ParseDiff(string(raw))
}

// mapPRState maps GitHub's state + merged boolean to a normalized state string.
func mapPRState(state string, merged bool) string {
	switch state {
	case "open":
		return "OPEN"
	case "closed":
		if merged {
			return "MERGED"
		}
		return "CLOSED"
	default:
		return state
	}
}

// normalizeFileStatus maps GitHub file status values to the CRoBot standard
// set. Returns "" for statuses that should be skipped.
func normalizeFileStatus(status string) string {
	switch status {
	case "added":
		return "added"
	case "removed":
		return "deleted"
	case "modified":
		return "modified"
	case "renamed":
		return "renamed"
	case "copied":
		return "added"
	case "changed":
		return "modified"
	case "unchanged":
		return "" // skip
	default:
		return status
	}
}
