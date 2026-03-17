// Package platform defines the abstractions and shared types for interacting
// with code-hosting platforms (Bitbucket, GitHub, GitLab, etc.).
package platform

import "context"

// Platform defines the contract that every code-hosting integration must
// satisfy. Each method accepts a context for cancellation and timeouts.
type Platform interface {
	// GetPRContext returns normalized metadata, changed files, and diff hunks
	// for the specified pull request.
	GetPRContext(ctx context.Context, opts PRRequest) (*PRContext, error)

	// GetFileContent returns the raw content of a file at a specific commit.
	GetFileContent(ctx context.Context, opts FileRequest) ([]byte, error)

	// ListBotComments returns existing comments posted by this bot on the
	// specified pull request.
	ListBotComments(ctx context.Context, opts PRRequest) ([]Comment, error)

	// CreateInlineComment posts a single inline comment on a pull request.
	CreateInlineComment(ctx context.Context, opts PRRequest, comment InlineComment) (*Comment, error)

	// DeleteComment removes a previously posted comment from a pull request.
	DeleteComment(ctx context.Context, opts PRRequest, commentID string) error
}
