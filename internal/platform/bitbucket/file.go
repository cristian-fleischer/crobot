package bitbucket

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

// GetFileContent retrieves the raw content of a file at a specific commit from
// the Bitbucket Cloud API.
func (c *Client) GetFileContent(ctx context.Context, opts platform.FileRequest) ([]byte, error) {
	workspace := c.resolveWorkspace(opts.Workspace)

	path := fmt.Sprintf("/2.0/repositories/%s/%s/src/%s/%s",
		url.PathEscape(workspace), url.PathEscape(opts.Repo), url.PathEscape(opts.Commit), opts.Path)

	resp, err := c.doRaw(ctx, "GET", path)
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
