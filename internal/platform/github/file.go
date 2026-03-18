package github

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

// GetFileContent retrieves the raw content of a file at a specific commit from
// the GitHub REST API. It uses the application/vnd.github.raw+json Accept
// header to get raw content directly, avoiding base64 decoding.
func (c *Client) GetFileContent(ctx context.Context, opts platform.FileRequest) ([]byte, error) {
	owner := c.resolveOwner(opts.Workspace)

	path := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s",
		url.PathEscape(owner), url.PathEscape(opts.Repo), opts.Path, url.QueryEscape(opts.Commit))

	resp, err := c.doRaw(ctx, "GET", path, "application/vnd.github.raw+json")
	if err != nil {
		return nil, fmt.Errorf("fetching file content: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("reading file content: %w", err)
	}

	return data, nil
}
