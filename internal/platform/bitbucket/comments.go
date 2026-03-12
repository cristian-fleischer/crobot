package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/dizzyc/crobot/internal/platform"
)

// bbComment is the Bitbucket API representation of a pull request comment.
type bbComment struct {
	ID      int `json:"id"`
	Content struct {
		Raw string `json:"raw"`
	} `json:"content"`
	Inline *struct {
		Path string `json:"path"`
		To   *int   `json:"to"`
		From *int   `json:"from"`
	} `json:"inline"`
	User struct {
		DisplayName string `json:"display_name"`
	} `json:"user"`
	CreatedOn string `json:"created_on"`
}

// bbCommentCreatePayload is the request body for creating a Bitbucket comment.
type bbCommentCreatePayload struct {
	Content struct {
		Raw string `json:"raw"`
	} `json:"content"`
	Inline *bbInlinePayload `json:"inline,omitempty"`
}

// bbInlinePayload represents the inline position for a Bitbucket comment.
type bbInlinePayload struct {
	Path string `json:"path"`
	To   *int   `json:"to,omitempty"`
	From *int   `json:"from,omitempty"`
}

// ListBotComments retrieves all comments on a pull request that contain a
// CRoBot fingerprint marker, indicating they were posted by this bot.
func (c *Client) ListBotComments(ctx context.Context, opts platform.PRRequest) ([]platform.Comment, error) {
	workspace := opts.Workspace
	if workspace == "" {
		workspace = c.workspace
	}

	basePath := fmt.Sprintf("/2.0/repositories/%s/%s/pullrequests/%d/comments",
		workspace, opts.Repo, opts.PRNumber)

	var botComments []platform.Comment

	data, err := c.do(ctx, "GET", basePath, nil)
	if err != nil {
		return nil, fmt.Errorf("listing PR comments: %w", err)
	}

	for {
		var page paginatedResponse
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("decoding comments page: %w", err)
		}

		var comments []bbComment
		if err := json.Unmarshal(page.Values, &comments); err != nil {
			return nil, fmt.Errorf("decoding comments entries: %w", err)
		}

		for _, bc := range comments {
			fp := platform.ExtractFingerprint(bc.Content.Raw)
			if fp == "" {
				continue
			}

			comment := platform.Comment{
				ID:          strconv.Itoa(bc.ID),
				Body:        bc.Content.Raw,
				Author:      bc.User.DisplayName,
				CreatedAt:   bc.CreatedOn,
				IsBot:       true,
				Fingerprint: fp,
			}

			if bc.Inline != nil {
				comment.Path = bc.Inline.Path
				if bc.Inline.To != nil {
					comment.Line = *bc.Inline.To
				} else if bc.Inline.From != nil {
					comment.Line = *bc.Inline.From
				}
			}

			botComments = append(botComments, comment)
		}

		if page.Next == "" {
			break
		}

		data, err = c.doURL(ctx, page.Next)
		if err != nil {
			return nil, fmt.Errorf("fetching comments next page: %w", err)
		}
	}

	return botComments, nil
}

// CreateInlineComment posts a single inline comment on a pull request.
func (c *Client) CreateInlineComment(ctx context.Context, opts platform.PRRequest, comment platform.InlineComment) (*platform.Comment, error) {
	workspace := opts.Workspace
	if workspace == "" {
		workspace = c.workspace
	}

	basePath := fmt.Sprintf("/2.0/repositories/%s/%s/pullrequests/%d/comments",
		workspace, opts.Repo, opts.PRNumber)

	payload := bbCommentCreatePayload{}
	payload.Content.Raw = comment.Body

	inline := &bbInlinePayload{
		Path: comment.Path,
	}
	if comment.Side == "old" {
		inline.From = &comment.Line
	} else {
		inline.To = &comment.Line
	}
	payload.Inline = inline

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling comment payload: %w", err)
	}

	respBody, err := c.do(ctx, "POST", basePath, jsonBody)
	if err != nil {
		return nil, fmt.Errorf("creating inline comment: %w", err)
	}

	var created bbComment
	if err := json.Unmarshal(respBody, &created); err != nil {
		return nil, fmt.Errorf("decoding created comment: %w", err)
	}

	result := &platform.Comment{
		ID:          strconv.Itoa(created.ID),
		Body:        created.Content.Raw,
		Author:      created.User.DisplayName,
		CreatedAt:   created.CreatedOn,
		IsBot:       true,
		Fingerprint: platform.ExtractFingerprint(created.Content.Raw),
	}
	if created.Inline != nil {
		result.Path = created.Inline.Path
		if created.Inline.To != nil {
			result.Line = *created.Inline.To
		} else if created.Inline.From != nil {
			result.Line = *created.Inline.From
		}
	}

	return result, nil
}

// DeleteComment removes a previously posted comment from a pull request.
func (c *Client) DeleteComment(ctx context.Context, opts platform.PRRequest, commentID string) error {
	workspace := opts.Workspace
	if workspace == "" {
		workspace = c.workspace
	}

	path := fmt.Sprintf("/2.0/repositories/%s/%s/pullrequests/%d/comments/%s",
		workspace, opts.Repo, opts.PRNumber, commentID)

	_, err := c.do(ctx, "DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("deleting comment: %w", err)
	}

	return nil
}
