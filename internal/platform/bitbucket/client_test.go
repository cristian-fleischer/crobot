package bitbucket

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
		User:  "user",
		Token: "token",
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

func TestNewClient_EmptyUser(t *testing.T) {
	t.Parallel()

	_, err := NewClient(&Config{Token: "token"})
	if err == nil {
		t.Fatal("expected error for empty user")
	}
}

func TestNewClient_EmptyToken(t *testing.T) {
	t.Parallel()

	_, err := NewClient(&Config{User: "user"})
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestNewClient_CustomBaseURL(t *testing.T) {
	t.Parallel()

	c, err := NewClient(&Config{
		User:    "user",
		Token:   "token",
		BaseURL: "http://custom.api",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.baseURL != "http://custom.api" {
		t.Errorf("baseURL: got %q, want %q", c.baseURL, "http://custom.api")
	}
}

func TestNewClient_CustomHTTPClient(t *testing.T) {
	t.Parallel()

	custom := &http.Client{}
	c, err := NewClient(&Config{
		User:       "user",
		Token:      "token",
		HTTPClient: custom,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.httpClient != custom {
		t.Error("expected custom HTTP client to be used")
	}
}

func TestFactory_Registration(t *testing.T) {
	t.Parallel()

	// The init() function should have registered "bitbucket".
	// We test this via the platform factory in the integration test files,
	// but here we just verify the constructor works with the right config type.
	cfg := &Config{User: "user", Token: "token"}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestDo_BasicAuth(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("expected Basic Auth")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if user != "testuser" || pass != "testtoken" {
			t.Errorf("auth: got %s/%s, want testuser/testtoken", user, pass)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok": true}`)
	}))
	defer server.Close()

	c, err := NewClient(&Config{
		User:       "testuser",
		Token:      "testtoken",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body, err := c.do(context.Background(), "GET", "/test", nil)
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

func TestDo_401(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.do(context.Background(), "GET", "/test", nil)
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
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.do(context.Background(), "GET", "/test", nil)
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
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

func TestDo_500(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got: %v", err)
	}
}

func TestDo_RateLimitRetry(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"rate limited"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	body, err := c.do(context.Background(), "GET", "/test", nil)
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

func TestDo_RateLimitExhausted(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"rate limited"}`)
	}))
	defer server.Close()

	c := testClient(t, server)
	_, err := c.do(context.Background(), "GET", "/test", nil)
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
	cancel() // cancel immediately

	_, err := c.do(ctx, "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
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
	if len(got) > 520 { // 512 + "..."
		t.Errorf("expected truncated body, got length %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected '...' suffix")
	}
}

// testClient creates a Client pointing at the given test server.
func testClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	c, err := NewClient(&Config{
		User:       "testuser",
		Token:      "testtoken",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	return c
}
