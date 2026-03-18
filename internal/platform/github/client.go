// Package github implements the platform.Platform interface for GitHub.
// It communicates with the GitHub REST API v3 using Bearer token authentication.
package github

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/version"
)

const (
	// defaultBaseURL is the base URL for the GitHub REST API.
	defaultBaseURL = "https://api.github.com"

	// apiVersion is the GitHub API version to use.
	apiVersion = "2022-11-28"

	// maxRetries is the maximum number of retries for rate-limited requests.
	maxRetries = 3
)

// linkNextRe matches the "next" relation in a Link header.
var linkNextRe = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// Config holds the configuration needed to create a GitHub platform client.
type Config struct {
	// Owner is the GitHub repository owner (user or organization).
	Owner string
	// Token is the GitHub personal access token or app installation token.
	Token string
	// BaseURL overrides the default GitHub API base URL. Useful for testing
	// or GitHub Enterprise Server.
	BaseURL string
	// HTTPClient is an optional HTTP client. If nil, a default client is used.
	HTTPClient *http.Client
}

// Client implements platform.Platform for GitHub.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	owner      string
	userAgent  string
}

func init() {
	platform.Register("github", func(cfg config.Config) (platform.Platform, error) {
		return NewClient(&Config{
			Owner: cfg.GitHub.Owner,
			Token: cfg.GitHub.Token,
		})
	})
}

// NewClient creates a new GitHub client from the provided configuration.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("github: config must not be nil")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("github: token must not be empty")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		token:      cfg.Token,
		owner:      cfg.Owner,
		userAgent:  "CRoBot/" + version.Version,
	}, nil
}

// retryWithBackoff executes fn up to maxRetries times, backing off when fn
// signals a retry is needed. It handles both primary rate limits (403 with
// x-ratelimit-remaining: 0) and secondary/abuse limits (429 with retry-after).
func retryWithBackoff(ctx context.Context, fn func(attempt int) (done bool, err error)) error {
	var lastErr error
	for attempt := range maxRetries {
		done, err := fn(attempt)
		if done || err != nil {
			return err
		}
		lastErr = fmt.Errorf("github: rate limited (attempt %d/%d)", attempt+1, maxRetries)
		backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("github: %w", ctx.Err())
		case <-time.After(backoff):
		}
	}
	return lastErr
}

// do executes an HTTP request with Bearer token auth, required GitHub headers,
// rate-limit retries, and error mapping. The body parameter is a byte slice
// (not io.Reader) so it can be safely re-sent on retries.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, http.Header, error) {
	var result []byte
	var resultHeader http.Header
	err := retryWithBackoff(ctx, func(_ int) (bool, error) {
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return true, fmt.Errorf("github: creating request: %w", err)
		}
		c.setHeaders(req)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return true, fmt.Errorf("github: executing request: %w", err)
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
		resp.Body.Close()
		if err != nil {
			return true, fmt.Errorf("github: reading response body: %w", err)
		}

		if shouldRetry(resp) {
			return false, nil
		}

		if err := mapHTTPError(resp.StatusCode, respBody); err != nil {
			return true, err
		}

		result = respBody
		resultHeader = resp.Header
		return true, nil
	})
	return result, resultHeader, err
}

// doRaw executes an HTTP request and returns the raw response without reading
// it into memory. The caller must close the response body.
func (c *Client) doRaw(ctx context.Context, method, path string, accept string) (*http.Response, error) {
	var result *http.Response
	err := retryWithBackoff(ctx, func(_ int) (bool, error) {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
		if err != nil {
			return true, fmt.Errorf("github: creating request: %w", err)
		}
		c.setHeaders(req)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return true, fmt.Errorf("github: executing request: %w", err)
		}

		if shouldRetry(resp) {
			resp.Body.Close()
			return false, nil
		}

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
			resp.Body.Close()
			if err := mapHTTPError(resp.StatusCode, body); err != nil {
				return true, err
			}
		}

		result = resp
		return true, nil
	})
	return result, err
}

// doURL executes a GET request against an absolute URL (used for pagination).
// It validates that the URL's host matches the configured base URL host to
// prevent SSRF via malicious pagination links.
func (c *Client) doURL(ctx context.Context, rawURL string) ([]byte, http.Header, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, fmt.Errorf("github: parsing pagination URL: %w", err)
	}
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("github: parsing base URL: %w", err)
	}
	if parsed.Host != base.Host {
		return nil, nil, fmt.Errorf("github: pagination URL host %q does not match base host %q", parsed.Host, base.Host)
	}

	var result []byte
	var resultHeader http.Header
	err = retryWithBackoff(ctx, func(_ int) (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return true, fmt.Errorf("github: creating request: %w", err)
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return true, fmt.Errorf("github: executing request: %w", err)
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
		resp.Body.Close()
		if err != nil {
			return true, fmt.Errorf("github: reading response body: %w", err)
		}

		if shouldRetry(resp) {
			return false, nil
		}

		if err := mapHTTPError(resp.StatusCode, respBody); err != nil {
			return true, err
		}

		result = respBody
		resultHeader = resp.Header
		return true, nil
	})
	return result, resultHeader, err
}

// setHeaders adds the required GitHub API headers to a request.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
}

// resolveOwner returns owner if non-empty, otherwise falls back to the
// client's configured owner.
func (c *Client) resolveOwner(owner string) string {
	if owner != "" {
		return owner
	}
	return c.owner
}

// parseLinkNext extracts the "next" URL from a Link header. Returns "" if
// there is no next page.
func parseLinkNext(header http.Header) string {
	link := header.Get("Link")
	if link == "" {
		return ""
	}
	matches := linkNextRe.FindStringSubmatch(link)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// shouldRetry returns true if the response indicates a rate limit and the
// request should be retried.
func shouldRetry(resp *http.Response) bool {
	// Secondary/abuse rate limit: 429 with Retry-After.
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	// Primary rate limit: 403 with x-ratelimit-remaining: 0.
	if resp.StatusCode == http.StatusForbidden {
		remaining := resp.Header.Get("X-Ratelimit-Remaining")
		if remaining == "0" {
			return true
		}
	}
	return false
}

// mapHTTPError converts GitHub HTTP error status codes into structured Go errors.
func mapHTTPError(statusCode int, body []byte) error {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return nil
	case statusCode == http.StatusUnauthorized:
		return fmt.Errorf("github: authentication failed (401): %s", truncateBody(body))
	case statusCode == http.StatusForbidden:
		return fmt.Errorf("github: access denied (403): %s", truncateBody(body))
	case statusCode == http.StatusNotFound:
		return fmt.Errorf("github: resource not found (404): %s", truncateBody(body))
	case statusCode == http.StatusUnprocessableEntity:
		return fmt.Errorf("github: validation failed (422): %s", truncateBody(body))
	default:
		return fmt.Errorf("github: unexpected status %d: %s", statusCode, truncateBody(body))
	}
}

// truncateBody returns at most 512 bytes of the response body for error messages.
func truncateBody(body []byte) string {
	const maxLen = 512
	if len(body) > maxLen {
		return string(body[:maxLen]) + "..."
	}
	return string(body)
}

// formatPerPage appends the per_page=100 query parameter to a URL path.
func formatPerPage(path string) string {
	if containsQuery(path) {
		return path + "&per_page=100"
	}
	return path + "?per_page=100"
}

// containsQuery returns true if the path already contains a query string.
func containsQuery(path string) bool {
	for _, c := range path {
		if c == '?' {
			return true
		}
	}
	return false
}
