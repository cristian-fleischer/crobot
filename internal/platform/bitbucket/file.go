package bitbucket

import (
	"context"
	"fmt"
	"io"

	"github.com/dizzyc/crobot/internal/platform"
)

// GetFileContent retrieves the raw content of a file at a specific commit from
// the Bitbucket Cloud API.
func (c *Client) GetFileContent(ctx context.Context, opts platform.FileRequest) ([]byte, error) {
	workspace := opts.Workspace
	if workspace == "" {
		workspace = c.workspace
	}

	path := fmt.Sprintf("/2.0/repositories/%s/%s/src/%s/%s",
		workspace, opts.Repo, opts.Commit, opts.Path)

	resp, err := c.doRaw(ctx, "GET", path)
	if err != nil {
		return nil, fmt.Errorf("fetching file content: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading file content: %w", err)
	}

	return data, nil
}
