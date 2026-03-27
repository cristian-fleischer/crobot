package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

// ghComment is the GitHub API representation of a pull request review comment.
type ghComment struct {
	ID           int    `json:"id"`
	Body         string `json:"body"`
	Path         string `json:"path"`
	Line         int    `json:"line"`
	Side         string `json:"side"` // "LEFT" or "RIGHT"
	CommitID     string `json:"commit_id"`
	InReplyToID  *int   `json:"in_reply_to_id"`
	User         ghUser `json:"user"`
	CreatedAt    string `json:"created_at"`
}

// ghCreateComment is the request body for creating a GitHub review comment.
type ghCreateComment struct {
	Body     string `json:"body"`
	CommitID string `json:"commit_id"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Side     string `json:"side"` // "RIGHT" or "LEFT"
}

// ListBotComments retrieves all review comments on a pull request that contain
// a CRoBot fingerprint marker, indicating they were posted by this bot.
func (c *Client) ListBotComments(ctx context.Context, opts platform.PRRequest) ([]platform.Comment, error) {
	owner := c.resolveOwner(opts.Workspace)

	basePath := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments",
		url.PathEscape(owner), url.PathEscape(opts.Repo), opts.PRNumber)

	var botComments []platform.Comment

	data, headers, err := c.do(ctx, "GET", formatPerPage(basePath), nil)
	if err != nil {
		return nil, fmt.Errorf("listing PR comments: %w", err)
	}

	for {
		var comments []ghComment
		if err := json.Unmarshal(data, &comments); err != nil {
			return nil, fmt.Errorf("decoding comments page: %w", err)
		}

		for _, gc := range comments {
			fp := platform.ExtractFingerprint(gc.Body)
			if fp == "" {
				continue
			}

			botComments = append(botComments, platform.Comment{
				ID:          strconv.Itoa(gc.ID),
				Path:        gc.Path,
				Line:        gc.Line,
				Body:        gc.Body,
				Author:      gc.User.Login,
				CreatedAt:   gc.CreatedAt,
				IsBot:       true,
				Fingerprint: fp,
			})
		}

		next := parseLinkNext(headers)
		if next == "" {
			break
		}

		data, headers, err = c.doURL(ctx, next)
		if err != nil {
			return nil, fmt.Errorf("fetching comments next page: %w", err)
		}
	}

	return botComments, nil
}

// ListPRComments retrieves all review comments on a pull request.
// Note: GitHub's REST API does not expose per-comment resolution status;
// IsResolved will always be false. Thread resolution requires the GraphQL API.
func (c *Client) ListPRComments(ctx context.Context, opts platform.PRRequest) ([]platform.Comment, error) {
	owner := c.resolveOwner(opts.Workspace)

	basePath := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments",
		url.PathEscape(owner), url.PathEscape(opts.Repo), opts.PRNumber)

	var allComments []platform.Comment

	data, headers, err := c.do(ctx, "GET", formatPerPage(basePath), nil)
	if err != nil {
		return nil, fmt.Errorf("listing PR comments: %w", err)
	}

	for {
		var comments []ghComment
		if err := json.Unmarshal(data, &comments); err != nil {
			return nil, fmt.Errorf("decoding comments page: %w", err)
		}

		for _, gc := range comments {
			comment := platform.Comment{
				ID:          strconv.Itoa(gc.ID),
				Path:        gc.Path,
				Line:        gc.Line,
				Body:        gc.Body,
				Author:      gc.User.Login,
				CreatedAt:   gc.CreatedAt,
				IsBot:       gc.User.Type == "Bot" || platform.ExtractFingerprint(gc.Body) != "",
				Fingerprint: platform.ExtractFingerprint(gc.Body),
			}
			if gc.InReplyToID != nil {
				comment.ParentID = strconv.Itoa(*gc.InReplyToID)
			}
			allComments = append(allComments, comment)
		}

		next := parseLinkNext(headers)
		if next == "" {
			break
		}

		data, headers, err = c.doURL(ctx, next)
		if err != nil {
			return nil, fmt.Errorf("fetching comments next page: %w", err)
		}
	}

	return allComments, nil
}

// CreateInlineComment posts a single inline review comment on a pull request.
// It requires a headCommit (the PR's head SHA) to be set on the PRRequest via
// the HeadCommit field — the caller should obtain this from a prior
// GetPRContext call.
func (c *Client) CreateInlineComment(ctx context.Context, opts platform.PRRequest, comment platform.InlineComment) (*platform.Comment, error) {
	owner := c.resolveOwner(opts.Workspace)

	basePath := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments",
		url.PathEscape(owner), url.PathEscape(opts.Repo), opts.PRNumber)

	// Map CRoBot side to GitHub side.
	side := "RIGHT"
	if comment.Side == "old" {
		side = "LEFT"
	}

	// The commit_id is stored in the PRRequest's Workspace field by the caller
	// via the HeadCommit from GetPRContext. However, the standard approach is
	// for the caller to provide it. We'll look for it in the comment's
	// Fingerprint as a fallback, but the primary approach is to require it
	// to be passed through the existing mechanism. For now, we need to fetch
	// the PR to get the head commit.
	headCommit, err := c.getHeadCommit(ctx, owner, opts.Repo, opts.PRNumber)
	if err != nil {
		return nil, fmt.Errorf("getting head commit for comment: %w", err)
	}

	payload := ghCreateComment{
		Body:     comment.Body,
		CommitID: headCommit,
		Path:     comment.Path,
		Line:     comment.Line,
		Side:     side,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling comment payload: %w", err)
	}

	respBody, _, err := c.do(ctx, "POST", basePath, jsonBody)
	if err != nil {
		return nil, fmt.Errorf("creating inline comment: %w", err)
	}

	var created ghComment
	if err := json.Unmarshal(respBody, &created); err != nil {
		return nil, fmt.Errorf("decoding created comment: %w", err)
	}

	return &platform.Comment{
		ID:          strconv.Itoa(created.ID),
		Path:        created.Path,
		Line:        created.Line,
		Body:        created.Body,
		Author:      created.User.Login,
		CreatedAt:   created.CreatedAt,
		IsBot:       true,
		Fingerprint: platform.ExtractFingerprint(created.Body),
	}, nil
}

// DeleteComment removes a previously posted review comment. Note: the GitHub
// endpoint for deleting pull request review comments does NOT include the PR
// number in the path.
func (c *Client) DeleteComment(ctx context.Context, opts platform.PRRequest, commentID string) error {
	owner := c.resolveOwner(opts.Workspace)

	// GitHub delete path: /repos/{owner}/{repo}/pulls/comments/{comment_id}
	path := fmt.Sprintf("/repos/%s/%s/pulls/comments/%s",
		url.PathEscape(owner), url.PathEscape(opts.Repo), url.PathEscape(commentID))

	_, _, err := c.do(ctx, "DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("deleting comment: %w", err)
	}

	return nil
}

// getHeadCommit fetches the PR metadata to obtain the head commit SHA, which
// is required for creating inline comments on GitHub.
func (c *Client) getHeadCommit(ctx context.Context, owner, repo string, prNumber int) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d",
		url.PathEscape(owner), url.PathEscape(repo), prNumber)

	data, _, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return "", fmt.Errorf("fetching PR for head commit: %w", err)
	}

	var pr ghPullRequest
	if err := json.Unmarshal(data, &pr); err != nil {
		return "", fmt.Errorf("decoding PR for head commit: %w", err)
	}

	if pr.Head.SHA == "" {
		return "", fmt.Errorf("github: PR head commit SHA is empty")
	}

	return pr.Head.SHA, nil
}
