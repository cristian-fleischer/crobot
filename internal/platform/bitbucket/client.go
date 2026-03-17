// Package bitbucket implements the platform.Platform interface for Bitbucket
// Cloud. It communicates with the Bitbucket Cloud REST API v2.0 using HTTP
// Basic Auth.
package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
)

const (
	// defaultBaseURL is the base URL for the Bitbucket Cloud API.
	defaultBaseURL = "https://api.bitbucket.org"

	// maxRetries is the maximum number of retries for rate-limited requests.
	maxRetries = 3
)

// Config holds the configuration needed to create a Bitbucket platform client.
type Config struct {
	// Workspace is the Bitbucket workspace slug.
	Workspace string
	// User is the Bitbucket username or email for authentication.
	User string
	// Token is the Bitbucket API token (app password) for authentication.
	Token string
	// BaseURL overrides the default Bitbucket API base URL. Useful for testing.
	BaseURL string
	// HTTPClient is an optional HTTP client. If nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// Client implements platform.Platform for Bitbucket Cloud.
type Client struct {
	httpClient *http.Client
	baseURL    string
	user       string
	token      string
	workspace  string
}

func init() {
	platform.Register("bitbucket", func(cfg config.Config) (platform.Platform, error) {
		return NewClient(&Config{
			Workspace: cfg.Bitbucket.Workspace,
			User:      cfg.Bitbucket.User,
			Token:     cfg.Bitbucket.Token,
		})
	})
}

// NewClient creates a new Bitbucket Cloud client from the provided configuration.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("bitbucket: config must not be nil")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("bitbucket: user must not be empty")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("bitbucket: token must not be empty")
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
		user:       cfg.User,
		token:      cfg.Token,
		workspace:  cfg.Workspace,
	}, nil
}

// retryWithBackoff executes fn up to maxRetries times, backing off
// exponentially when fn returns true for shouldRetry. It is the shared
// rate-limit retry loop used by do, doRaw, and doURL.
func retryWithBackoff(ctx context.Context, fn func(attempt int) (done bool, err error)) error {
	var lastErr error
	for attempt := range maxRetries {
		done, err := fn(attempt)
		if done || err != nil {
			return err
		}
		// fn indicated a retry is needed (rate limited).
		lastErr = fmt.Errorf("bitbucket: rate limited (attempt %d/%d)", attempt+1, maxRetries)
		backoff := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
		select {
		case <-ctx.Done():
			return fmt.Errorf("bitbucket: %w", ctx.Err())
		case <-time.After(backoff):
		}
	}
	return lastErr
}

// do executes an HTTP request with Basic Auth, rate-limit retries, and error
// mapping. It returns the response body bytes on success. The body parameter
// is a byte slice (not io.Reader) so it can be safely re-sent on retries.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var result []byte
	err := retryWithBackoff(ctx, func(_ int) (bool, error) {
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return true, fmt.Errorf("bitbucket: creating request: %w", err)
		}
		req.SetBasicAuth(c.user, c.token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return true, fmt.Errorf("bitbucket: executing request: %w", err)
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
		resp.Body.Close()
		if err != nil {
			return true, fmt.Errorf("bitbucket: reading response body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			return false, nil // signal retry
		}

		if err := mapHTTPError(resp.StatusCode, respBody); err != nil {
			return true, err
		}

		result = respBody
		return true, nil
	})
	return result, err
}

// doRaw is like do but returns the raw response body as an io.ReadCloser
// without reading it into memory. The caller must close the body.
func (c *Client) doRaw(ctx context.Context, method, path string) (*http.Response, error) {
	var result *http.Response
	err := retryWithBackoff(ctx, func(_ int) (bool, error) {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
		if err != nil {
			return true, fmt.Errorf("bitbucket: creating request: %w", err)
		}
		req.SetBasicAuth(c.user, c.token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return true, fmt.Errorf("bitbucket: executing request: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			return false, nil // signal retry
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
func (c *Client) doURL(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: parsing pagination URL: %w", err)
	}
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: parsing base URL: %w", err)
	}
	if parsed.Host != base.Host {
		return nil, fmt.Errorf("bitbucket: pagination URL host %q does not match base host %q", parsed.Host, base.Host)
	}

	var result []byte
	err = retryWithBackoff(ctx, func(_ int) (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return true, fmt.Errorf("bitbucket: creating request: %w", err)
		}
		req.SetBasicAuth(c.user, c.token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return true, fmt.Errorf("bitbucket: executing request: %w", err)
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
		resp.Body.Close()
		if err != nil {
			return true, fmt.Errorf("bitbucket: reading response body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			return false, nil // signal retry
		}

		if err := mapHTTPError(resp.StatusCode, respBody); err != nil {
			return true, err
		}

		result = respBody
		return true, nil
	})
	return result, err
}

// resolveWorkspace returns ws if non-empty, otherwise falls back to the
// client's configured workspace.
func (c *Client) resolveWorkspace(ws string) string {
	if ws != "" {
		return ws
	}
	return c.workspace
}

// paginatedResponse is the common shape for Bitbucket paginated API responses.
type paginatedResponse struct {
	Values json.RawMessage `json:"values"`
	Next   string          `json:"next"`
}

// mapHTTPError converts Bitbucket HTTP error status codes into structured Go
// errors.
func mapHTTPError(statusCode int, body []byte) error {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return nil
	case statusCode == http.StatusUnauthorized:
		return fmt.Errorf("bitbucket: authentication failed (401): %s", truncateBody(body))
	case statusCode == http.StatusForbidden:
		return fmt.Errorf("bitbucket: access denied (403): %s", truncateBody(body))
	case statusCode == http.StatusNotFound:
		return fmt.Errorf("bitbucket: resource not found (404): %s", truncateBody(body))
	default:
		return fmt.Errorf("bitbucket: unexpected status %d: %s", statusCode, truncateBody(body))
	}
}

// truncateBody returns at most 512 bytes of the response body for error
// messages.
func truncateBody(body []byte) string {
	const maxLen = 512
	if len(body) > maxLen {
		return string(body[:maxLen]) + "..."
	}
	return string(body)
}
