package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

// bbPR is the Bitbucket API representation of a pull request.
type bbPR struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	Author      struct {
		DisplayName string `json:"display_name"`
	} `json:"author"`
	Source struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
	} `json:"source"`
	Destination struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
	} `json:"destination"`
}

// bbDiffStatEntry is a single entry in the Bitbucket diffstat response.
type bbDiffStatEntry struct {
	New struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"new"`
	Old struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"old"`
	Status string `json:"status"`
}

// GetPRContext fetches pull request metadata, diffstat, and raw diff from the
// Bitbucket Cloud API and returns a normalized PRContext.
func (c *Client) GetPRContext(ctx context.Context, opts platform.PRRequest) (*platform.PRContext, error) {
	workspace := opts.Workspace
	if workspace == "" {
		workspace = c.workspace
	}

	basePath := fmt.Sprintf("/2.0/repositories/%s/%s/pullrequests/%d", workspace, opts.Repo, opts.PRNumber)

	// Fetch PR metadata.
	prBody, err := c.do(ctx, "GET", basePath, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching PR metadata: %w", err)
	}

	var pr bbPR
	if err := json.Unmarshal(prBody, &pr); err != nil {
		return nil, fmt.Errorf("decoding PR metadata: %w", err)
	}

	// Fetch diffstat (paginated).
	files, err := c.fetchDiffstat(ctx, basePath+"/diffstat")
	if err != nil {
		return nil, fmt.Errorf("fetching diffstat: %w", err)
	}

	// Fetch raw diff.
	diffHunks, err := c.fetchDiff(ctx, basePath+"/diff")
	if err != nil {
		return nil, fmt.Errorf("fetching diff: %w", err)
	}

	return &platform.PRContext{
		ID:           pr.ID,
		Title:        pr.Title,
		Description:  pr.Description,
		Author:       pr.Author.DisplayName,
		SourceBranch: pr.Source.Branch.Name,
		TargetBranch: pr.Destination.Branch.Name,
		State:        pr.State,
		HeadCommit:   pr.Source.Commit.Hash,
		BaseCommit:   pr.Destination.Commit.Hash,
		Files:        files,
		DiffHunks:    diffHunks,
	}, nil
}

// fetchDiffstat retrieves the diffstat for a pull request, following pagination.
func (c *Client) fetchDiffstat(ctx context.Context, path string) ([]platform.ChangedFile, error) {
	var allFiles []platform.ChangedFile

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	for {
		var page paginatedResponse
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("decoding diffstat page: %w", err)
		}

		var entries []bbDiffStatEntry
		if err := json.Unmarshal(page.Values, &entries); err != nil {
			return nil, fmt.Errorf("decoding diffstat entries: %w", err)
		}

		for _, e := range entries {
			cf := platform.ChangedFile{
				Path:   e.New.Path,
				Status: normalizeDiffStatus(e.Status),
			}
			// For renames/moves, the old path differs from the new path.
			if e.Old.Path != "" && e.Old.Path != e.New.Path {
				cf.OldPath = e.Old.Path
			}
			// For deleted files, the new path may be empty.
			if cf.Path == "" {
				cf.Path = e.Old.Path
			}
			allFiles = append(allFiles, cf)
		}

		if page.Next == "" {
			break
		}

		data, err = c.doURL(ctx, page.Next)
		if err != nil {
			return nil, fmt.Errorf("fetching diffstat next page: %w", err)
		}
	}

	return allFiles, nil
}

// fetchDiff retrieves the raw unified diff for a pull request and parses it.
func (c *Client) fetchDiff(ctx context.Context, path string) ([]platform.DiffHunk, error) {
	resp, err := c.doRaw(ctx, "GET", path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading diff body: %w", err)
	}

	return parseDiff(string(raw))
}

// normalizeDiffStatus maps Bitbucket diffstat status values to a consistent
// set of status strings.
func normalizeDiffStatus(status string) string {
	switch status {
	case "added":
		return "added"
	case "removed":
		return "deleted"
	case "modified":
		return "modified"
	case "renamed":
		return "renamed"
	default:
		return status
	}
}
