package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient_Valid(t *testing.T) {
	t.Parallel()

	c, err := NewClient(&Config{
		Token: "ghp_test123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.baseURL != defaultBaseURL {
		t.Errorf("baseURL: got %q, want %q", c.baseURL, defaultBaseURL)
	}
}

func TestNewClient_NilConfig(t *testing.T) {
	t.Parallel()

	_, err := NewClient(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewClient_EmptyToken(t *testing.T) {
	t.Parallel()

	_, err := NewClient(&Config{Owner: "owner"})
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestNewClient_CustomBaseURL(t *testing.T) {
	t.Parallel()

	c, err := NewClient(&Config{
		Token:   "ghp_test",
		BaseURL: "https://github.example.com/api/v3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.baseURL != "https://github.example.com/api/v3" {
		t.Errorf("baseURL: got %q", c.baseURL)
	}
}

func TestNewClient_CustomHTTPClient(t *testing.T) {
	t.Parallel()

	custom := &http.Client{}
	c, err := NewClient(&Config{
		Token:      "ghp_test",
		HTTPClient: custom,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.httpClient != custom {
		t.Error("expected custom HTTP client to be used")
	}
}

func TestDo_BearerAuth(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer ghp_testtoken" {
			t.Errorf("Authorization: got %q, want %q", auth, "Bearer ghp_testtoken")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header is required")
		}
		if r.Header.Get("X-GitHub-Api-Version") != apiVersion {
			t.Errorf("X-GitHub-Api-Version: got %q, want %q", r.Header.Get("X-GitHub-Api-Version"), apiVersion)
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("Accept: got %q", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok": true}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	body, _, err := c.do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]bool
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result["ok"] {
		t.Error("expected ok=true")
	}
}

func TestDo_UserAgentHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if !strings.HasPrefix(ua, "CRoBot/") {
			t.Errorf("User-Agent: got %q, want prefix CRoBot/", ua)
		}
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, _, err := c.do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDo_401(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, _, err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestDo_403(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Non-rate-limit 403 (has remaining > 0).
		w.Header().Set("X-Ratelimit-Remaining", "4999")
		http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, _, err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 in error, got: %v", err)
	}
}

func TestDo_404(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, _, err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

func TestDo_422(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Validation Failed"}`, http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, _, err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for 422")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("expected 422 in error, got: %v", err)
	}
}

func TestDo_500(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Internal Server Error"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, _, err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got: %v", err)
	}
}

func TestDo_429AbuseRateLimit(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"message":"abuse rate limit"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	body, _, err := c.do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]bool
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result["ok"] {
		t.Error("expected ok=true after retry")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_403PrimaryRateLimit(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 2 {
			w.Header().Set("X-Ratelimit-Remaining", "0")
			w.Header().Set("X-Ratelimit-Reset", "9999999999")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"message":"API rate limit exceeded"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	body, _, err := c.do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]bool
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result["ok"] {
		t.Error("expected ok=true after retry")
	}
}

func TestDo_RateLimitExhausted(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"message":"rate limited"}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, _, err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error when rate limit exhausted")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	c := testClient(t, server)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := c.do(ctx, "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestDoURL_HostMismatch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not have been sent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, _, err := c.doURL(context.Background(), "https://evil.example.com/steal-creds")
	if err == nil {
		t.Fatal("expected error for host mismatch")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("expected host mismatch error, got: %v", err)
	}
}

func TestParseLinkNext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "has next",
			header: `<https://api.github.com/repos/owner/repo/pulls/1/files?page=2>; rel="next", <https://api.github.com/repos/owner/repo/pulls/1/files?page=5>; rel="last"`,
			want:   "https://api.github.com/repos/owner/repo/pulls/1/files?page=2",
		},
		{
			name:   "only last",
			header: `<https://api.github.com/repos/owner/repo/pulls/1/files?page=1>; rel="last"`,
			want:   "",
		},
		{
			name:   "empty",
			header: "",
			want:   "",
		},
		{
			name:   "next only",
			header: `<https://api.github.com/repos/owner/repo/pulls/1/comments?page=3>; rel="next"`,
			want:   "https://api.github.com/repos/owner/repo/pulls/1/comments?page=3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := http.Header{}
			if tt.header != "" {
				h.Set("Link", tt.header)
			}
			got := parseLinkNext(h)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateBody(t *testing.T) {
	t.Parallel()

	short := "short body"
	if got := truncateBody([]byte(short)); got != short {
		t.Errorf("got %q, want %q", got, short)
	}

	long := strings.Repeat("x", 600)
	got := truncateBody([]byte(long))
	if len(got) > 520 {
		t.Errorf("expected truncated body, got length %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected '...' suffix")
	}
}

func TestFormatPerPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{"no query", "/repos/o/r/pulls/1/files", "/repos/o/r/pulls/1/files?per_page=100"},
		{"existing query", "/repos/o/r/contents/f?ref=abc", "/repos/o/r/contents/f?ref=abc&per_page=100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatPerPage(tt.path)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		remaining  string
		want       bool
	}{
		{"429", http.StatusTooManyRequests, "", true},
		{"403 with remaining=0", http.StatusForbidden, "0", true},
		{"403 with remaining>0", http.StatusForbidden, "4999", false},
		{"403 no header", http.StatusForbidden, "", false},
		{"200", http.StatusOK, "4999", false},
		{"500", http.StatusInternalServerError, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header{},
			}
			if tt.remaining != "" {
				resp.Header.Set("X-Ratelimit-Remaining", tt.remaining)
			}
			got := shouldRetry(resp)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveOwner(t *testing.T) {
	t.Parallel()

	c := &Client{owner: "default-owner"}

	if got := c.resolveOwner("explicit"); got != "explicit" {
		t.Errorf("got %q, want %q", got, "explicit")
	}
	if got := c.resolveOwner(""); got != "default-owner" {
		t.Errorf("got %q, want %q", got, "default-owner")
	}
}

// testClient creates a Client pointing at the given test server.
func testClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	c, err := NewClient(&Config{
		Token:      "ghp_testtoken",
		Owner:      "testowner",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	return c
}
