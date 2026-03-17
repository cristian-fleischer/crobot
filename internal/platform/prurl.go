package platform

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParsePRURL parses a pull request URL and extracts the workspace, repo, and
// PR number. Currently supports Bitbucket Cloud URLs of the form:
//
//	https://bitbucket.org/{workspace}/{repo}/pull-requests/{id}
func ParsePRURL(rawURL string) (*PRRequest, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid PR URL: %w", err)
	}

	// Normalize: strip trailing slash and fragments/query.
	p := strings.TrimSuffix(u.Path, "/")
	segments := strings.Split(strings.TrimPrefix(p, "/"), "/")

	switch u.Host {
	case "bitbucket.org":
		return parseBitbucketURL(segments)
	default:
		return nil, fmt.Errorf("unsupported PR URL host %q (supported: bitbucket.org)", u.Host)
	}
}

// parseBitbucketURL parses path segments for a Bitbucket Cloud PR URL.
// Expected: [workspace, repo, "pull-requests", id]
func parseBitbucketURL(segments []string) (*PRRequest, error) {
	if len(segments) < 4 || segments[2] != "pull-requests" {
		return nil, fmt.Errorf("invalid Bitbucket PR URL: expected /{workspace}/{repo}/pull-requests/{id}")
	}

	prNum, err := strconv.Atoi(segments[3])
	if err != nil || prNum <= 0 {
		return nil, fmt.Errorf("invalid Bitbucket PR URL: %q is not a valid PR number", segments[3])
	}

	return &PRRequest{
		Workspace: segments[0],
		Repo:      segments[1],
		PRNumber:  prNum,
	}, nil
}

// IsPRURL returns true if s looks like a pull request URL (starts with http:// or https://).
func IsPRURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
